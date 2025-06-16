// Lab 7: Implement a web server

package web

import (
	"bytes"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type server struct {
	Addr string
	Port int

	metadataService VideoMetadataService
	contentService  VideoContentService

	mux *http.ServeMux
}

type indexData struct {
	Id         string
	EscapedID  string
	UploadTime time.Time
}

type VideoData struct {
	Id         string
	UploadedAt time.Time
}

func NewServer(
	metadataService VideoMetadataService,
	contentService VideoContentService,
) *server {
	return &server{
		metadataService: metadataService,
		contentService:  contentService,
	}
}

func (s *server) Start(lis net.Listener) error {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/upload", s.handleUpload)
	s.mux.HandleFunc("/videos/", s.handleVideo)
	s.mux.HandleFunc("/content/", s.handleVideoContent)
	s.mux.HandleFunc("/", s.handleIndex)

	return http.Serve(lis, s.mux)
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := s.metadataService.List()
	if err != nil {
		http.Error(w, "unable to list videos", http.StatusInternalServerError)
		return
	}

	var video_metas []indexData
	for _, meta := range data {
		video_metas = append(video_metas, indexData{
			Id:         meta.Id,
			EscapedID:  url.PathEscape(meta.Id),
			UploadTime: meta.UploadedAt,
		})
	}

	indexTemplate := template.Must(template.New("index").Parse(indexHTML))

	var buf bytes.Buffer
	if err := indexTemplate.Execute(&buf, video_metas); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "failed to parse multi-part form", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file not provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(handler.Filename, ".mp4") {
		http.Error(w, "only .mp4 allowed", http.StatusBadRequest)
		return
	}

	videoID := strings.TrimSuffix(handler.Filename, ".mp4")

	exists, _ := s.metadataService.Read(videoID)
	if exists != nil {
		http.Error(w, "video ID already exists", http.StatusConflict)
		return
	}

	tempDir, err := os.MkdirTemp("", "upload-*")
	if err != nil {
		http.Error(w, "cannot create temp dir", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	mp4Path := filepath.Join(tempDir, handler.Filename)
	outFile, err := os.Create(mp4Path)
	if err != nil {
		http.Error(w, "cannot save file", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()
	io.Copy(outFile, file)

	manifestPath := filepath.Join(tempDir, "manifest.mpd")
	cmd := exec.Command("ffmpeg",
		"-i", mp4Path,
		"-c:v", "libx264",
		"-c:a", "aac",
		"-bf", "1",
		"-keyint_min", "120",
		"-g", "120",
		"-sc_threshold", "0",
		"-b:v", "3000k",
		"-b:a", "128k",
		"-f", "dash",
		"-use_timeline", "1",
		"-use_template", "1",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		"-seg_duration", "4",
		manifestPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Println("FFmpeg error:", string(output))
		http.Error(w, "ffmpeg failed", http.StatusInternalServerError)
		return
	}

	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return s.contentService.Write(videoID, filepath.Base(path), data)
	})

	if err != nil {
		http.Error(w, "failed to store content", http.StatusInternalServerError)
		return
	}

	if err := s.metadataService.Create(videoID, time.Now()); err != nil {
		http.Error(w, "failed to store metadata", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) handleVideo(w http.ResponseWriter, r *http.Request) {
	videoId := r.URL.Path[len("/videos/"):]
	metaData, err := s.metadataService.Read(videoId)
	if err != nil {
		http.Error(w, "failed to fetch metadata", http.StatusInternalServerError)
		return
	}
	if metaData == nil {
		http.NotFound(w, r)
		return
	}

	data := VideoData{
		Id:         metaData.Id,
		UploadedAt: metaData.UploadedAt,
	}

	var buf bytes.Buffer
	videoTemplate := template.Must(template.New("video").Parse(videoHTML))
	if err := videoTemplate.Execute(&buf, data); err != nil {
		http.Error(w, "template render failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func (s *server) handleVideoContent(w http.ResponseWriter, r *http.Request) {
	videoId := r.URL.Path[len("/content/"):]
	parts := strings.Split(videoId, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid content path", http.StatusBadRequest)
		return
	}
	videoId = parts[0]
	filename := parts[1]

	data, err := s.contentService.Read(videoId, filename)
	if err != nil {
		http.Error(w, "failed to read content", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
