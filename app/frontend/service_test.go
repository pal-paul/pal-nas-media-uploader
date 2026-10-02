package frontend

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesServesGalleryAndAssets(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "assets"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("<main>Gallery</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "assets", "app.js"), []byte("window.gallery=true"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	service.RegisterRoutes(router)

	for _, test := range []struct {
		path string
		body string
	}{
		{path: "/", body: "Gallery"},
		{path: "/assets/app.js", body: "window.gallery=true"},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), test.body) {
			t.Fatalf("GET %s returned %d: %s", test.path, response.Code, response.Body.String())
		}
	}
}

func TestNoRouteKeepsMissingAPIAsJSON404(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("<main>Gallery</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	service.RegisterRoutes(router)
	request := httptest.NewRequest(http.MethodGet, "/missing-api", nil)
	request.Header.Set("Accept", gin.MIMEJSON)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"error":"not found"`) {
		t.Fatalf("missing API returned %d: %s", response.Code, response.Body.String())
	}
}
