package trash

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ExpiredMedia struct {
	ID            string
	MediaPath     string
	ThumbnailPath string
}

type Repository interface {
	ListExpiredTrash(context.Context, time.Time, int) ([]ExpiredMedia, error)
	DeleteExpiredMedia(context.Context, string) error
}

type Service struct {
	repository Repository
	mediaDir   string
}

func New(repository Repository, mediaDir string) *Service {
	return &Service{repository: repository, mediaDir: filepath.Clean(mediaDir)}
}

func (service *Service) Cleanup(ctx context.Context) (int, error) {
	items, err := service.repository.ListExpiredTrash(ctx, time.Now(), 500)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, item := range items {
		for _, relative := range []string{item.MediaPath, item.ThumbnailPath} {
			if relative == "" {
				continue
			}
			path, pathErr := service.safePath(relative)
			if pathErr != nil {
				return removed, pathErr
			}
			if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return removed, removeErr
			}
		}
		if err := service.repository.DeleteExpiredMedia(ctx, item.ID); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func (service *Service) Run(ctx context.Context) {
	service.runCleanup(ctx)
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.runCleanup(ctx)
		}
	}
}

func (service *Service) runCleanup(ctx context.Context) {
	for {
		removed, err := service.Cleanup(ctx)
		if err != nil || removed < 500 {
			return
		}
	}
}

func (service *Service) CleanupHandler(context *gin.Context) {
	removed, err := service.Cleanup(context.Request.Context())
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": "trash cleanup failed"})
		return
	}
	context.JSON(http.StatusOK, gin.H{"removed": removed})
}

func (service *Service) safePath(relative string) (string, error) {
	path := filepath.Join(service.mediaDir, filepath.FromSlash(relative))
	resolved, err := filepath.Rel(service.mediaDir, path)
	if err != nil || resolved == ".." || strings.HasPrefix(resolved, ".."+string(filepath.Separator)) || filepath.IsAbs(resolved) {
		return "", errors.New("invalid media path")
	}
	return path, nil
}
