package transfer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"pal-nas-media-uploader/app/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Media struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	MimeType  string    `json:"mimeType"`
	Size      int64     `json:"size"`
	SHA256    string    `json:"sha256"`
	CreatedAt time.Time `json:"createdAt"`
}

type Album struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	MediaIDs    []string `json:"mediaIds"`
}

type Manifest struct {
	Version    int       `json:"version"`
	ExportedAt time.Time `json:"exportedAt"`
	Media      []Media   `json:"media"`
	Albums     []Album   `json:"albums"`
}

type Repository interface {
	ExportManifest(context.Context, string) (Manifest, error)
	ImportManifest(context.Context, string, string, Manifest) (int, error)
}

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (service *Service) Export(context *gin.Context) {
	user, ok := currentUser(context)
	if !ok {
		context.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	manifest, err := service.repository.ExportManifest(context, user.ID)
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": "export failed"})
		return
	}
	manifest.Version = 1
	manifest.ExportedAt = time.Now().UTC()
	context.Header("Content-Disposition", "attachment; filename=\"pal-media-metadata.json\"")
	context.JSON(http.StatusOK, manifest)
}

func (service *Service) Import(context *gin.Context) {
	user, ok := currentUser(context)
	if !ok {
		context.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	decoder := json.NewDecoder(io.LimitReader(context.Request.Body, 10<<20))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil || manifest.Version != 1 || len(manifest.Albums) > 1000 {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid or unsupported metadata manifest"})
		return
	}
	sessionID := uuid.NewString()
	imported, err := service.repository.ImportManifest(context, user.ID, sessionID, manifest)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "metadata import failed"})
		return
	}
	context.JSON(http.StatusAccepted, gin.H{"sessionId": sessionID, "status": "completed", "importedAlbums": imported})
}

func currentUser(context *gin.Context) (auth.User, bool) {
	value, exists := context.Get(auth.UserContextKey)
	user, ok := value.(auth.User)
	return user, exists && ok
}
