package gallery

import (
	"context"
	"time"
)

type Album struct {
	ID           string    `json:"id"`
	OwnerID      string    `json:"ownerId"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Automatic    bool      `json:"automatic"`
	CreatedAt    time.Time `json:"createdAt"`
	ItemCount    int64     `json:"itemCount"`
	CoverMediaID string    `json:"coverMediaId,omitempty"`
	Media        []Media   `json:"media,omitempty"`
}

type Media struct {
	ID            string     `json:"id"`
	OwnerID       string     `json:"ownerId"`
	OwnerUsername string     `json:"ownerUsername"`
	Title         string     `json:"title"`
	Kind          string     `json:"kind"`
	Favorite      bool       `json:"favorite"`
	Filename      string     `json:"fileName"`
	MimeType      string     `json:"mimeType"`
	Size          int64      `json:"fileSize"`
	SHA256        string     `json:"sha256"`
	CreatedAt     time.Time  `json:"createdAt"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
	Shared        bool       `json:"shared"`
}

type MediaFilter struct {
	Search string
	Kind   string
	Sort   string
	Trash  bool
}

type Storage struct {
	TotalBytes int64 `json:"totalBytes"`
	PhotoBytes int64 `json:"photoBytes"`
	VideoBytes int64 `json:"videoBytes"`
	PhotoCount int64 `json:"photoCount"`
	VideoCount int64 `json:"videoCount"`
}

type Repository interface {
	ListAlbums(context.Context, string, string) ([]Album, error)
	CreateAlbum(context.Context, Album) error
	GetAlbum(context.Context, string, string) (Album, error)
	UpdateAlbum(context.Context, string, string, string, string) error
	DeleteAlbum(context.Context, string, string) error
	SetAlbumOrder(context.Context, string, []string) error
	AddAlbumMedia(context.Context, string, string, []string) error
	RemoveAlbumMedia(context.Context, string, string, string) error
	SetAlbumCover(context.Context, string, string, string) error
	ListMedia(context.Context, string, MediaFilter) ([]Media, error)
	SetFavorite(context.Context, string, string, bool) error
	TrashMedia(context.Context, string, string) error
	RestoreMedia(context.Context, string, string) error
	GetOwnedMediaPath(context.Context, string, string) (string, error)
	DeleteMedia(context.Context, string, string) error
	GetStorage(context.Context, string) (Storage, error)
}
