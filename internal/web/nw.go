package web

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sort"
	"strings"
	"sync"

	"tritontube/internal/proto"

	"google.golang.org/grpc"
)

type NetworkVideoContentService struct {
	proto.UnimplementedVideoContentAdminServiceServer

	nodes       map[string]proto.StorageClient
	nodesHashes []uint64
	hashToNode  map[uint64]string
	mu          sync.RWMutex
}

var _ VideoContentService = (*NetworkVideoContentService)(nil)

func hashStringToUint64(s string) uint64 {
	sum := sha256.Sum256([]byte(s))
	return binary.BigEndian.Uint64(sum[:8])
}

func NewNetworkVideoContentService(addresses []string, adminHostPort string) (*NetworkVideoContentService, error) {
	n := &NetworkVideoContentService{
		nodes:      make(map[string]proto.StorageClient),
		hashToNode: make(map[uint64]string),
	}

	for _, addr := range addresses {
		conn, err := grpc.Dial(
			addr,
			grpc.WithInsecure(),
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(100*1024*1024),
				grpc.MaxCallSendMsgSize(100*1024*1024),
			),
		)
		if err != nil {
			log.Printf("[ERROR] Failed to connect to %s: %v", addr, err)
			return nil, err
		}
		client := proto.NewStorageClient(conn)
		hash := hashStringToUint64(addr)
		n.nodes[addr] = client
		n.hashToNode[hash] = addr
		n.nodesHashes = append(n.nodesHashes, hash)
	}

	sort.Slice(n.nodesHashes, func(i, j int) bool {
		return n.nodesHashes[i] < n.nodesHashes[j]
	})

	log.Printf("[INIT] Initialized with %d nodes", len(n.nodesHashes))

	go func() {
		listener, err := net.Listen("tcp", adminHostPort)
		if err != nil {
			log.Fatalf("Admin - Failed to listen on %s: %v", adminHostPort, err)
			return
		}

		log.Printf("[Admin] gRPC server listening at %s", adminHostPort)

		grpcServer := grpc.NewServer()
		proto.RegisterVideoContentAdminServiceServer(grpcServer, n)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Admin gRPC server failed: %v", err)
		}
	}()

	return n, nil
}

func (n *NetworkVideoContentService) getClientForKey(key string) (proto.StorageClient, string) {
	hash := hashStringToUint64(key)

	n.mu.RLock()
	defer n.mu.RUnlock()

	idx := sort.Search(len(n.nodesHashes), func(i int) bool {
		return n.nodesHashes[i] >= hash
	})
	if idx == len(n.nodesHashes) {
		idx = 0
	}

	nodeHash := n.nodesHashes[idx]
	nodeAddr := n.hashToNode[nodeHash]
	log.Printf("[ROUTING] Key '%s' → Hash %d → Node %s", key, hash, nodeAddr)
	return n.nodes[nodeAddr], nodeAddr
}

func (n *NetworkVideoContentService) Read(videoId, filename string) ([]byte, error) {
	key := fmt.Sprintf("%s/%s", videoId, filename)
	client, nodeAddr := n.getClientForKey(key)

	log.Printf("[READ] %s from node %s", key, nodeAddr)

	stream, err := client.Download(context.Background(), &proto.FileRequest{
		VideoId:  videoId,
		Filename: filename,
	})
	if err != nil {
		log.Printf("[ERROR] Download start failed: %v", err)
		return nil, err
	}

	var data []byte
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Printf("[ERROR] Failed to receive chunk: %v", err)
			return nil, err
		}
		data = append(data, chunk.Data...)
	}
	return data, nil
}

func (n *NetworkVideoContentService) Write(videoId, filename string, data []byte) error {
	if strings.HasSuffix(filename, ".mp4") {
		log.Printf("[SKIP] Skipping storage of raw MP4 file: %s", filename)
		return nil
	}

	key := fmt.Sprintf("%s/%s", videoId, filename)
	client, nodeAddr := n.getClientForKey(key)

	log.Printf("[WRITE] %s to node %s", key, nodeAddr)

	stream, err := client.Upload(context.Background())
	if err != nil {
		log.Printf("[ERROR] Upload start failed: %v", err)
		return err
	}

	const chunkSize = 1024 * 1024
	for start := 0; start < len(data); start += chunkSize {
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}
		err = stream.Send(&proto.FileChunk{
			VideoId:  videoId,
			Filename: filename,
			Data:     data[start:end],
		})
		if err != nil {
			log.Printf("[ERROR] Upload chunk failed: %v", err)
			return err
		}
	}

	_, err = stream.CloseAndRecv()
	if err != nil {
		log.Printf("[ERROR] Upload finalize failed: %v", err)
	}
	return err
}

func (svc *NetworkVideoContentService) AddNode(ctx context.Context, req *proto.AddNodeRequest) (*proto.AddNodeResponse, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	newAddr := req.NodeAddress
	newHash := hashStringToUint64(newAddr)

	if _, exists := svc.nodes[newAddr]; exists {
		log.Printf("[AddNode] Node %s already exists", newAddr)
		return nil, fmt.Errorf("node already exists")
	}

	conn, err := grpc.Dial(newAddr,
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1024*1024*100),
			grpc.MaxCallSendMsgSize(1024*1024*100),
		),
	)
	if err != nil {
		log.Printf("[AddNode] Failed to connect to %s: %v", newAddr, err)
		return nil, fmt.Errorf("failed to connect to new node: %v", err)
	}
	client := proto.NewStorageClient(conn)

	svc.nodes[newAddr] = client
	svc.hashToNode[newHash] = newAddr
	svc.nodesHashes = append(svc.nodesHashes, newHash)
	sort.Slice(svc.nodesHashes, func(i, j int) bool {
		return svc.nodesHashes[i] < svc.nodesHashes[j]
	})

	var newIndex int
	for i, h := range svc.nodesHashes {
		if h == newHash {
			newIndex = i
			break
		}
	}

	predIndex := newIndex - 1
	if predIndex < 0 {
		predIndex = len(svc.nodesHashes) - 1
	}
	predHash := svc.nodesHashes[predIndex]
	predAddr := svc.hashToNode[predHash]

	succIndex := (newIndex + 1) % len(svc.nodesHashes)
	succHash := svc.nodesHashes[succIndex]
	succAddr := svc.hashToNode[succHash]
	succClient := svc.nodes[succAddr]

	log.Printf("[AddNode] New node %s (hash: %d), predecessor: %s (hash: %d), successor: %s (hash: %d)",
		newAddr, newHash, predAddr, predHash, succAddr, succHash)

	if len(svc.nodesHashes) == 1 || succAddr == newAddr {
		log.Printf("[AddNode] Skipping migration - single node configuration")
		return &proto.AddNodeResponse{MigratedFileCount: 0}, nil
	}

	var migratedCount int32
	videosResp, err := succClient.ListVideos(ctx, &proto.ListVideosRequest{})
	if err != nil {
		log.Printf("[AddNode] ListVideos failed from %s: %v", succAddr, err)
		return nil, fmt.Errorf("list videos failed: %v", err)
	}

	log.Printf("[AddNode] Found %d video IDs on successor %s", len(videosResp.VideoIds), succAddr)
	for _, vid := range videosResp.VideoIds {
		filesResp, err := succClient.ListVideoFiles(ctx, &proto.ListVideoFilesRequest{
			VideoId: vid,
		})
		if err != nil {
			log.Printf("[AddNode] Error listing files for %s: %v", vid, err)
			continue
		}

		var delFilenames []string

		log.Printf("[AddNode] Checking %d files for video %s", len(filesResp.Filenames), vid)
		for _, fname := range filesResp.Filenames {
			key := fmt.Sprintf("%s/%s", vid, fname)
			keyHash := hashStringToUint64(key)

			inRange := inRangeExclusive(predHash, newHash, keyHash)
			log.Printf("[AddNode] Key '%s' → Hash %d → InRange: %v", key, keyHash, inRange)

			if inRange {
				err := migrateFileSync(vid, fname, succClient, client)
				if err != nil {
					log.Printf("[AddNode] Failed to migrate source file %s/%s: %v", vid, fname, err)
				} else {
					delFilenames = append(delFilenames, fname)
					migratedCount++
				}
			}
		}

		if len(delFilenames) > 0 {
			_, err := succClient.DeleteFiles(ctx, &proto.BatchDeleteRequest{
				VideoId:   vid,
				Filenames: delFilenames,
			})
			if err != nil {
				log.Printf("[AddNode] Failed to delete source files for Video ID %s: %v", vid, err)
			}
		}
	}

	log.Printf("[AddNode] Finished adding %s. Migrated %d files from %s", newAddr, migratedCount, succAddr)
	return &proto.AddNodeResponse{MigratedFileCount: migratedCount}, nil
}

func (svc *NetworkVideoContentService) RemoveNode(ctx context.Context, req *proto.RemoveNodeRequest) (*proto.RemoveNodeResponse, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	numNodes := len(svc.nodesHashes)
	if numNodes == 1 {
		return nil, fmt.Errorf("system needs to have atleast one")
	}

	removeAddr := req.NodeAddress
	removeHash := hashStringToUint64(removeAddr)

	client, exists := svc.nodes[removeAddr]
	if !exists {
		return nil, fmt.Errorf("node does not exist")
	}

	var succHash uint64
	for i, h := range svc.nodesHashes {
		if h == removeHash {
			succHash = svc.nodesHashes[(i+1)%len(svc.nodesHashes)]
			break
		}
	}
	succAddr := svc.hashToNode[succHash]
	succClient := svc.nodes[succAddr]

	var migratedCount int32
	videosResp, err := client.ListVideos(ctx, &proto.ListVideosRequest{})
	if err != nil {
		return nil, fmt.Errorf("list videos failed: %v", err)
	}
	for _, vid := range videosResp.VideoIds {
		filesResp, err := client.ListVideoFiles(ctx, &proto.ListVideoFilesRequest{
			VideoId: vid,
		})
		if err != nil {
			log.Printf("[RemoveNode] Error listing files for %s: %v", vid, err)
			continue
		}
		for _, fname := range filesResp.Filenames {
			if err := migrateFileSync(vid, fname, client, succClient); err == nil {
				migratedCount++
			}
		}
	}

	delete(svc.nodes, removeAddr)
	delete(svc.hashToNode, removeHash)
	for i, h := range svc.nodesHashes {
		if h == removeHash {
			svc.nodesHashes = append(svc.nodesHashes[:i], svc.nodesHashes[i+1:]...)
			break
		}
	}

	log.Printf("[RemoveNode] Removed %s (hash %d), migrated %d files to %s", removeAddr, removeHash, migratedCount, succAddr)
	return &proto.RemoveNodeResponse{MigratedFileCount: migratedCount}, nil
}

func (svc *NetworkVideoContentService) ListNodes(ctx context.Context, req *proto.ListNodesRequest) (*proto.ListNodesResponse, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	var sortedNodes []string
	for _, hash := range svc.nodesHashes {
		addr := svc.hashToNode[hash]
		sortedNodes = append(sortedNodes, addr)
	}

	log.Printf("[ListNodes] Returning %d nodes: %v", len(sortedNodes), sortedNodes)
	return &proto.ListNodesResponse{Nodes: sortedNodes}, nil
}

func inRangeExclusive(start, end, hash uint64) bool {
	if start < end {
		return hash > start && hash <= end
	}
	return hash > start || hash <= end
}

func migrateFileSync(videoId, filename string, from proto.StorageClient, to proto.StorageClient) error {
	ctx := context.Background()
	log.Printf("[MIGRATE] Starting migration of %s/%s", videoId, filename)

	downloadStream, err := from.Download(ctx, &proto.FileRequest{
		VideoId:  videoId,
		Filename: filename,
	})
	if err != nil {
		log.Printf("[MIGRATE] Failed to start download for %s/%s: %v", videoId, filename, err)
		return err
	}

	uploadStream, err := to.Upload(ctx)
	if err != nil {
		log.Printf("[MIGRATE] Failed to start upload for %s/%s: %v", videoId, filename, err)
		return err
	}

	chunkCount := 0
	for {
		chunk, err := downloadStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[MIGRATE] Error receiving chunk for %s/%s: %v", videoId, filename, err)
			return err
		}

		chunkCount++
		if err := uploadStream.Send(chunk); err != nil {
			log.Printf("[MIGRATE] Error sending chunk for %s/%s: %v", videoId, filename, err)
			return err
		}
	}

	ack, err := uploadStream.CloseAndRecv()
	if err != nil {
		log.Printf("[MIGRATE] Failed to finalize upload for %s/%s: %v", videoId, filename, err)
		return err
	}
	if !ack.Success {
		log.Printf("[MIGRATE] Upload failed (ack=false) for %s/%s", videoId, filename)
		return fmt.Errorf("upload ack failed for %s/%s", videoId, filename)
	}

	log.Printf("[MIGRATE] Successfully migrated %s/%s with %d chunks", videoId, filename, chunkCount)
	return nil
}
