package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"tritontube/internal/proto"
)

type Server struct {
	proto.UnimplementedStorageServer
	BaseDir    string
	videoIndex map[string][]string
	mu         sync.RWMutex
}

func NewServer(baseDir string) *Server {
	return &Server{
		BaseDir:    baseDir,
		videoIndex: make(map[string][]string),
	}
}

func (s *Server) Upload(stream proto.Storage_UploadServer) error {
	var file *os.File
	var path string
	var videoId, filename string

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			if file != nil {
				file.Close()
			}
			s.mu.Lock()
			if s.videoIndex == nil {
				s.videoIndex = make(map[string][]string)
			}
			found := false
			for _, f := range s.videoIndex[videoId] {
				if f == filename {
					found = true
					break
				}
			}
			if !found {
				s.videoIndex[videoId] = append(s.videoIndex[videoId], filename)
			}
			s.mu.Unlock()

			log.Printf("[UPLOAD] Completed: %s", path)
			return stream.SendAndClose(&proto.UploadAck{Success: true})
		}
		if err != nil {
			return fmt.Errorf("error receiving chunk: %v", err)
		}

		if file == nil {
			videoId = chunk.VideoId
			filename = chunk.Filename
			path = filepath.Join(s.BaseDir, videoId, filename)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return fmt.Errorf("mkdir failed: %v", err)
			}
			file, err = os.Create(path)
			if err != nil {
				return fmt.Errorf("file create failed: %v", err)
			}
			log.Printf("[UPLOAD] Started: %s", path)
		}

		if _, err := file.Write(chunk.Data); err != nil {
			return fmt.Errorf("write failed: %v", err)
		}
	}
}

func (s *Server) Download(req *proto.FileRequest, stream proto.Storage_DownloadServer) error {
	path := filepath.Join(s.BaseDir, req.VideoId, req.Filename)
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open error: %v", err)
	}
	defer file.Close()

	buf := make([]byte, 1024*1024)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("read error: %v", err)
		}
		if n == 0 {
			break
		}
		if err := stream.Send(&proto.FileChunk{
			VideoId:  req.VideoId,
			Filename: req.Filename,
			Data:     buf[:n],
		}); err != nil {
			return fmt.Errorf("send error: %v", err)
		}
	}
	return nil
}

func (s *Server) ListVideos(ctx context.Context, req *proto.ListVideosRequest) (*proto.ListVideosResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var vids []string
	for vid := range s.videoIndex {
		vids = append(vids, vid)
	}
	return &proto.ListVideosResponse{VideoIds: vids}, nil
}

func (s *Server) ListVideoFiles(ctx context.Context, req *proto.ListVideoFilesRequest) (*proto.ListVideoFilesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &proto.ListVideoFilesResponse{
		Filenames: s.videoIndex[req.VideoId],
	}, nil
}

func (s *Server) DeleteFiles(ctx context.Context, req *proto.BatchDeleteRequest) (*proto.DeleteFileResponse, error) {
	basePath := filepath.Join(s.BaseDir, req.VideoId)
	s.mu.Lock()
	defer s.mu.Unlock()

	currentFiles := s.videoIndex[req.VideoId]
	if currentFiles == nil {
		log.Printf("[DELETE] Video %s not found in index", req.VideoId)
		return &proto.DeleteFileResponse{Success: false}, nil
	}

	filesToDelete := make(map[string]bool)
	for _, fname := range req.Filenames {
		filesToDelete[fname] = true
	}

	successfullyDeleted := make(map[string]bool)

	for _, fname := range req.Filenames {
		path := filepath.Join(basePath, fname)
		if err := os.Remove(path); err != nil {
			log.Printf("[DELETE] Failed to remove %s: %v", path, err)
		} else {
			log.Printf("[DELETE] Removed %s", path)
			successfullyDeleted[fname] = true
		}
	}

	remaining := []string{}
	for _, fname := range currentFiles {
		if !successfullyDeleted[fname] {
			remaining = append(remaining, fname)
		}
	}

	if len(remaining) == 0 {
		delete(s.videoIndex, req.VideoId)
	} else {
		s.videoIndex[req.VideoId] = remaining
	}

	allDeleted := len(successfullyDeleted) == len(req.Filenames)
	return &proto.DeleteFileResponse{Success: allDeleted}, nil
}
