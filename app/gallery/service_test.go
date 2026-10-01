package gallery

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"pal-nas-media-uploader/app/auth"

	"github.com/gin-gonic/gin"
)

type testRepository struct {
	createdAlbum Album
	mediaPath    string
	deletedID    string
	deletedOwner string
}

func (repository *testRepository) ListAlbums(context.Context, string, string) ([]Album, error) {
	return nil, nil
}
func (repository *testRepository) CreateAlbum(_ context.Context, album Album) error {
	repository.createdAlbum = album
	return nil
}
func (repository *testRepository) GetAlbum(context.Context, string, string) (Album, error) {
	return repository.createdAlbum, nil
}
func (repository *testRepository) UpdateAlbum(context.Context, string, string, string, string) error {
	return nil
}
func (repository *testRepository) DeleteAlbum(context.Context, string, string) error { return nil }
func (repository *testRepository) SetAlbumOrder(context.Context, string, []string) error {
	return nil
}
func (repository *testRepository) AddAlbumMedia(context.Context, string, string, []string) error {
	return nil
}
func (repository *testRepository) RemoveAlbumMedia(context.Context, string, string, string) error {
	return nil
}
func (repository *testRepository) SetAlbumCover(context.Context, string, string, string) error {
	return nil
}
func (repository *testRepository) ListMedia(context.Context, string, MediaFilter) ([]Media, error) {
	return nil, nil
}
func (repository *testRepository) SetFavorite(context.Context, string, string, bool) error {
	return nil
}
func (repository *testRepository) TrashMedia(context.Context, string, string) error { return nil }
func (repository *testRepository) RestoreMedia(context.Context, string, string) error {
	return nil
}
func (repository *testRepository) GetOwnedMediaPath(context.Context, string, string) (string, error) {
	return repository.mediaPath, nil
}
func (repository *testRepository) DeleteMedia(_ context.Context, mediaID, ownerID string) error {
	repository.deletedID = mediaID
	repository.deletedOwner = ownerID
	return nil
}
func (repository *testRepository) GetStorage(context.Context, string) (Storage, error) {
	return Storage{}, nil
}

func TestCreateAlbumUsesAuthenticatedOwner(t *testing.T) {
	repository := &testRepository{}
	router := testRouter(repository, t.TempDir())
	request := httptest.NewRequest(http.MethodPost, "/albums", bytes.NewBufferString(`{"title":"  Holiday  ","description":"  Coast  "}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}
	if repository.createdAlbum.OwnerID != "user-1" {
		t.Fatalf("expected authenticated owner, got %q", repository.createdAlbum.OwnerID)
	}
	if repository.createdAlbum.Title != "Holiday" || repository.createdAlbum.Description != "Coast" {
		t.Fatalf("expected trimmed album fields, got %#v", repository.createdAlbum)
	}
}

func TestDeleteMediaRemovesOwnedFile(t *testing.T) {
	mediaDir := t.TempDir()
	relativePath := filepath.Join("alice", "video.mp4")
	path := filepath.Join(mediaDir, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("media"), 0600); err != nil {
		t.Fatal(err)
	}
	repository := &testRepository{mediaPath: relativePath}
	router := testRouter(repository, mediaDir)
	request := httptest.NewRequest(http.MethodDelete, "/media/files/media-1/permanent", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, response.Code, response.Body.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected media file to be removed, got %v", err)
	}
	if repository.deletedID != "media-1" || repository.deletedOwner != "user-1" {
		t.Fatalf("unexpected delete scope: media=%q owner=%q", repository.deletedID, repository.deletedOwner)
	}
}

func testRouter(repository Repository, mediaDir string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(context *gin.Context) {
		context.Set(auth.UserContextKey, auth.User{ID: "user-1", Role: "user", UploadFolder: "alice"})
	})
	RegisterRoutes(router, NewService(repository, mediaDir))
	return router
}
