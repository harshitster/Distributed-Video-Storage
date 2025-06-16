package web

import (
	"fmt"
	"os"
	"path/filepath"
)

type FSVideoContentService struct {
	base_dir string
}

var _ VideoContentService = (*FSVideoContentService)(nil)

func NewFSVideoContentService(base_dir string) *FSVideoContentService {
	return &FSVideoContentService{
		base_dir: base_dir,
	}
}

func (s *FSVideoContentService) Write(videoId string, filename string, data []byte) error {
	dirPath := filepath.Join(s.base_dir, videoId)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed tp create directory: %w", err)
	}

	filePath := filepath.Join(dirPath, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (s *FSVideoContentService) Read(videoId string, filename string) ([]byte, error) {
	filePath := filepath.Join(s.base_dir, videoId, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return data, nil
}
