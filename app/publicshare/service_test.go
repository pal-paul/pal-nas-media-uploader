package publicshare

import (
	"context"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type downloadRepository struct {
	link Link
}

func (*downloadRepository) CreatePublicLink(context.Context, Link, string, string) error {
	return nil
}

func (repository *downloadRepository) ResolvePublicLink(context.Context, string, time.Time) (Link, error) {
	return repository.link, nil
}

func (*downloadRepository) DeletePublicLink(context.Context, string, string) error {
	return nil
}

func TestDownloadFormatsUnsafeFilename(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mediaDir := t.TempDir()
	filename := "photo\r\nX-Injected: true.jpg"
	if err := os.WriteFile(filepath.Join(mediaDir, "media.jpg"), []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	repository := &downloadRepository{link: Link{Files: []File{{Filename: filename, MediaPath: "media.jpg"}}}}
	router := gin.New()
	router.GET("/public/:token/download", New(repository, mediaDir).Download)
	request := httptest.NewRequest(http.MethodGet, "/public/token/download", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	disposition := response.Header().Get("Content-Disposition")
	if strings.ContainsAny(disposition, "\r\n") {
		t.Fatalf("unsafe content disposition %q", disposition)
	}
	_, parameters, err := mime.ParseMediaType(disposition)
	if err != nil || parameters["filename"] != filename {
		t.Fatalf("content disposition %q parsed as %#v: %v", disposition, parameters, err)
	}
}
