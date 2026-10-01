package gallery

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"pal-nas-media-uploader/app/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	ErrNotFound  = errors.New("resource not found")
	ErrForbidden = errors.New("operation not permitted")
)

type Service struct {
	repository Repository
	mediaDir   string
}

func NewService(repository Repository, mediaDir string) *Service {
	return &Service{repository: repository, mediaDir: filepath.Clean(mediaDir)}
}

func RegisterRoutes(router gin.IRoutes, service *Service) {
	router.GET("/albums", service.ListAlbums)
	router.POST("/albums", service.CreateAlbum)
	router.PATCH("/albums/order", service.SetAlbumOrder)
	router.GET("/albums/:id", service.GetAlbum)
	router.PATCH("/albums/:id", service.UpdateAlbum)
	router.DELETE("/albums/:id", service.DeleteAlbum)
	router.POST("/albums/:id/media", service.AddAlbumMedia)
	router.DELETE("/albums/:id/media/:mediaID", service.RemoveAlbumMedia)
	router.PATCH("/albums/:id/cover", service.SetAlbumCover)
	router.GET("/media", service.ListMedia)
	router.PATCH("/media/files/:id/favorite", service.SetFavorite)
	router.DELETE("/media/files/:id", service.TrashMedia)
	router.PATCH("/media/files/:id/restore", service.RestoreMedia)
	router.DELETE("/media/files/:id/permanent", service.DeleteMedia)
	router.GET("/storage", service.GetStorage)
}

func (service *Service) ListAlbums(context *gin.Context) {
	user := currentUser(context)
	albums, err := service.repository.ListAlbums(context, user.ID, context.Query("search"))
	respond(context, albums, err)
}

func (service *Service) CreateAlbum(context *gin.Context) {
	user := currentUser(context)
	var request struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if decodeJSON(context, &request) != nil || strings.TrimSpace(request.Title) == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	album := Album{ID: uuid.NewString(), OwnerID: user.ID, Title: strings.TrimSpace(request.Title), Description: strings.TrimSpace(request.Description)}
	if err := service.repository.CreateAlbum(context, album); err != nil {
		respond(context, nil, err)
		return
	}
	created, err := service.repository.GetAlbum(context, album.ID, user.ID)
	if err != nil {
		respond(context, nil, err)
		return
	}
	context.JSON(http.StatusCreated, created)
}

func (service *Service) SetAlbumOrder(context *gin.Context) {
	var request struct {
		AlbumIDs []string `json:"albumIds"`
	}
	if decodeJSON(context, &request) != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid album order"})
		return
	}
	respondNoContent(context, service.repository.SetAlbumOrder(context, currentUser(context).ID, request.AlbumIDs))
}

func (service *Service) GetAlbum(context *gin.Context) {
	album, err := service.repository.GetAlbum(context, context.Param("id"), currentUser(context).ID)
	respond(context, album, err)
}

func (service *Service) UpdateAlbum(context *gin.Context) {
	var request struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if decodeJSON(context, &request) != nil || strings.TrimSpace(request.Title) == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	err := service.repository.UpdateAlbum(context, context.Param("id"), currentUser(context).ID, strings.TrimSpace(request.Title), strings.TrimSpace(request.Description))
	respondNoContent(context, err)
}

func (service *Service) DeleteAlbum(context *gin.Context) {
	respondNoContent(context, service.repository.DeleteAlbum(context, context.Param("id"), currentUser(context).ID))
}

func (service *Service) AddAlbumMedia(context *gin.Context) {
	var request struct {
		MediaIDs []string `json:"mediaIds"`
	}
	if decodeJSON(context, &request) != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid media list"})
		return
	}
	err := service.repository.AddAlbumMedia(context, context.Param("id"), currentUser(context).ID, request.MediaIDs)
	respondNoContent(context, err)
}

func (service *Service) RemoveAlbumMedia(context *gin.Context) {
	err := service.repository.RemoveAlbumMedia(context, context.Param("id"), currentUser(context).ID, context.Param("mediaID"))
	respondNoContent(context, err)
}

func (service *Service) SetAlbumCover(context *gin.Context) {
	var request struct {
		MediaID string `json:"mediaId"`
	}
	if decodeJSON(context, &request) != nil || request.MediaID == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "mediaId is required"})
		return
	}
	err := service.repository.SetAlbumCover(context, context.Param("id"), currentUser(context).ID, request.MediaID)
	respondNoContent(context, err)
}

func (service *Service) ListMedia(context *gin.Context) {
	filter := MediaFilter{Search: context.Query("search"), Kind: context.Query("kind"), Sort: context.Query("sort"), Trash: context.Query("trash") == "true"}
	media, err := service.repository.ListMedia(context, currentUser(context).ID, filter)
	respond(context, media, err)
}

func (service *Service) SetFavorite(context *gin.Context) {
	var request struct {
		Favorite bool `json:"favorite"`
	}
	if decodeJSON(context, &request) != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid favorite state"})
		return
	}
	err := service.repository.SetFavorite(context, context.Param("id"), currentUser(context).ID, request.Favorite)
	respondNoContent(context, err)
}

func (service *Service) TrashMedia(context *gin.Context) {
	err := service.repository.TrashMedia(context, context.Param("id"), currentUser(context).ID)
	respondNoContent(context, err)
}

func (service *Service) RestoreMedia(context *gin.Context) {
	err := service.repository.RestoreMedia(context, context.Param("id"), currentUser(context).ID)
	respondNoContent(context, err)
}

func (service *Service) DeleteMedia(context *gin.Context) {
	user := currentUser(context)
	mediaID := context.Param("id")
	relativePath, err := service.repository.GetOwnedMediaPath(context, mediaID, user.ID)
	if err != nil {
		respondNoContent(context, err)
		return
	}
	path := filepath.Join(service.mediaDir, filepath.FromSlash(relativePath))
	cleanRelative, pathErr := filepath.Rel(service.mediaDir, path)
	if pathErr != nil || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		respondNoContent(context, ErrForbidden)
		return
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		context.JSON(http.StatusBadGateway, gin.H{"error": "unable to delete media file"})
		return
	}
	respondNoContent(context, service.repository.DeleteMedia(context, mediaID, user.ID))
}

func (service *Service) GetStorage(context *gin.Context) {
	storage, err := service.repository.GetStorage(context, currentUser(context).ID)
	respond(context, storage, err)
}

func currentUser(context *gin.Context) auth.User {
	value, _ := context.Get(auth.UserContextKey)
	user, _ := value.(auth.User)
	return user
}

func decodeJSON(context *gin.Context, target any) error {
	decoder := json.NewDecoder(io.LimitReader(context.Request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func respond(context *gin.Context, value any, err error) {
	if err == nil {
		context.JSON(http.StatusOK, value)
		return
	}
	respondError(context, err)
}

func respondNoContent(context *gin.Context, err error) {
	if err == nil {
		context.Status(http.StatusNoContent)
		return
	}
	respondError(context, err)
}

func respondError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		context.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, ErrForbidden):
		context.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	default:
		_ = context.Error(err)
		context.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
