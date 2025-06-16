package web

import (
	"context"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdVideoMetadataService struct {
	client *clientv3.Client
	prefix string
}

var _ VideoMetadataService = (*EtcdVideoMetadataService)(nil)

func NewEtcdVideoMetadataService(endpointsCSV string) (*EtcdVideoMetadataService, error) {
	endpoints := strings.Split(endpointsCSV, ",")

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return &EtcdVideoMetadataService{
		client: cli,
		prefix: "/videos/",
	}, nil
}

func (e *EtcdVideoMetadataService) Create(videoID string, uploadedAt time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := e.prefix + videoID
	value := uploadedAt.UTC().Format(time.RFC3339)

	_, err := e.client.Put(ctx, key, value)
	return err
}

func (e *EtcdVideoMetadataService) Read(videoID string) (*VideoMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := e.prefix + videoID
	resp, err := e.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}

	uploadedAtStr := string(resp.Kvs[0].Value)
	t, err := time.Parse(time.RFC3339, uploadedAtStr)
	if err != nil {
		t, err = time.Parse("2006-01-02 15:04:05", uploadedAtStr)
		if err != nil {
			return nil, err
		}
	}

	return &VideoMetadata{
		Id:         videoID,
		UploadedAt: t,
	}, nil
}

func (e *EtcdVideoMetadataService) List() ([]VideoMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := e.client.Get(ctx, e.prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	var videos []VideoMetadata
	for _, kv := range resp.Kvs {
		id := strings.TrimPrefix(string(kv.Key), e.prefix)
		uploadedAtStr := string(kv.Value)

		t, err := time.Parse(time.RFC3339, uploadedAtStr)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", uploadedAtStr)
			if err != nil {
				return nil, err
			}
		}

		videos = append(videos, VideoMetadata{
			Id:         id,
			UploadedAt: t,
		})
	}

	return videos, nil
}
