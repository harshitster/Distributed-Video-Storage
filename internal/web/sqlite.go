package web

import (
	"database/sql"
	"errors"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteVideoMetadataService struct {
	db *sql.DB
}

var _ VideoMetadataService = (*SQLiteVideoMetadataService)(nil)

func NewSQLiteVideoMetadataService(dbpath string) (*SQLiteVideoMetadataService, error) {
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		return nil, err
	}

	createStmt := `
		CREATE TABLE IF NOT EXISTS videos (
			ID TEXT PRIMARY KEY,
			uploaded_at DATETIME NOT NULL
		);
	`

	if _, err := db.Exec(createStmt); err != nil {
		return nil, err
	}

	return &SQLiteVideoMetadataService{db: db}, nil
}

func (s *SQLiteVideoMetadataService) Create(videoID string, uploaded_at time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO videos (ID, uploaded_at)
		VALUES (?, ?)
	`, videoID, uploaded_at.UTC().Format(time.RFC3339))

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	return nil
}

func (s *SQLiteVideoMetadataService) Read(videoID string) (*VideoMetadata, error) {
	row := s.db.QueryRow(`
		SELECT ID, uploaded_at
		FROM videos
		WHERE ID = ?
	`, videoID)

	var ID string
	var uploaded_at string

	if err := row.Scan(&ID, &uploaded_at); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	t, err := time.Parse(time.RFC3339, uploaded_at)
	if err != nil {
		t, err = time.Parse("2006-01-02 15:04:05", uploaded_at)
		if err != nil {
			return nil, err
		}
	}

	return &VideoMetadata{
		Id:         ID,
		UploadedAt: t,
	}, nil
}

func (s *SQLiteVideoMetadataService) List() ([]VideoMetadata, error) {
	rows, err := s.db.Query(`SELECT ID, uploaded_at FROM videos`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []VideoMetadata
	for rows.Next() {
		var Id string
		var uploaded_at string

		if err := rows.Scan(&Id, &uploaded_at); err != nil {
			return nil, err
		}

		t, err := time.Parse(time.RFC3339, uploaded_at)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", uploaded_at)
			if err != nil {
				return nil, err
			}
		}

		videos = append(videos, VideoMetadata{
			Id:         Id,
			UploadedAt: t,
		})
	}

	return videos, nil
}
