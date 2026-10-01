package uploader

import (
	"context"
	"sync"
	"time"
)

type CreateUploadRequest struct {
	Filename string `json:"filename" binding:"required"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size" binding:"required"`
	SHA256   string `json:"sha256"`
}

type Upload struct {
	ID         string    `json:"id"`
	OwnerID    string    `json:"owner_id"`
	UserFolder string    `json:"user_folder"`
	Filename   string    `json:"filename"`
	MimeType   string    `json:"mime_type"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256,omitempty"`
	Chunks     int64     `json:"chunks"`
	Created    time.Time `json:"created_at"`
	Completed  time.Time `json:"completed_at,omitempty"`
	MediaPath  string    `json:"media_path,omitempty"`
}

type CompletedMedia struct {
	UploadID  string
	OwnerID   string
	Filename  string
	MimeType  string
	Size      int64
	SHA256    string
	MediaPath string
	CreatedAt time.Time
}

type MediaRepository interface {
	SaveCompletedMedia(context.Context, CompletedMedia) error
	OwnerStorageBytes(context.Context, string) (int64, error)
}

type uploadSession struct {
	upload  Upload
	mutex   sync.RWMutex
	deleted bool
}
