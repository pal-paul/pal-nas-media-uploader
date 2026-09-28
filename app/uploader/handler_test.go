package uploader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

type createResponse struct {
	ID string `json:"id"`
}

type completeResponse struct {
	MediaPath string `json:"media_path"`
}

type recordingMediaRepository struct {
	media []CompletedMedia
}

func (repository *recordingMediaRepository) SaveCompletedMedia(_ context.Context, media CompletedMedia) error {
	repository.media = append(repository.media, media)
	return nil
}

func TestUploadSurvivesRestartAndCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	tmpDir := filepath.Join(root, "tmp")
	mediaDir := filepath.Join(root, "media")
	content := []byte("durable media")
	digest := sha256.Sum256(content)

	firstService := newTestService(t, tmpDir, mediaDir, int64(len(content)))
	firstRouter := newTestRouter(t, firstService)
	created := createUpload(t, firstRouter, fmt.Sprintf(
		`{"filename":"../video.mp4","size":%d,"sha256":"%s"}`,
		len(content), hex.EncodeToString(digest[:]),
	))

	if _, err := os.Stat(filepath.Join(tmpDir, created.ID, metadataFilename)); err != nil {
		t.Fatalf("metadata was not persisted: %v", err)
	}

	recoveredService := newTestService(t, tmpDir, mediaDir, int64(len(content)))
	recoveredRouter := newTestRouter(t, recoveredService)
	response := performRequest(recoveredRouter, http.MethodPut, "/media/upload/"+created.ID+"/parts/0", bytes.NewReader(content))
	if response.Code != http.StatusOK {
		t.Fatalf("upload part returned %d: %s", response.Code, response.Body.String())
	}

	response = performRequest(recoveredRouter, http.MethodPost, "/media/upload/"+created.ID+"/complete", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("complete returned %d: %s", response.Code, response.Body.String())
	}
	var completed completeResponse
	if err := json.Unmarshal(response.Body.Bytes(), &completed); err != nil {
		t.Fatalf("decode completion response: %v", err)
	}
	if completed.MediaPath == "" || filepath.IsAbs(completed.MediaPath) {
		t.Fatalf("expected a relative media path, got %q", completed.MediaPath)
	}
	stored, err := os.ReadFile(filepath.Join(mediaDir, filepath.FromSlash(completed.MediaPath)))
	if err != nil {
		t.Fatalf("read completed media: %v", err)
	}
	if !bytes.Equal(stored, content) {
		t.Fatalf("completed content mismatch: got %q", stored)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, created.ID)); !os.IsNotExist(err) {
		t.Fatalf("temporary upload directory still exists: %v", err)
	}
}

func TestCreateUploadValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	service := newTestService(t, filepath.Join(root, "tmp"), filepath.Join(root, "media"), 4)
	router := newTestRouter(t, service)

	tests := []struct {
		name string
		body string
	}{
		{name: "too large", body: `{"filename":"video.mp4","size":5}`},
		{name: "invalid filename", body: `{"filename":"..","size":4}`},
		{name: "invalid checksum", body: `{"filename":"video.mp4","size":4,"sha256":"invalid"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := performRequest(router, http.MethodPost, "/media/upload", bytes.NewBufferString(test.body))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestConcurrentChunkReplacement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	service := newTestService(t, filepath.Join(root, "tmp"), filepath.Join(root, "media"), 4)
	router := newTestRouter(t, service)
	created := createUpload(t, router, `{"filename":"video.mp4","size":4}`)

	contents := [][]byte{[]byte("aaaa"), []byte("bbbb")}
	var waitGroup sync.WaitGroup
	errors := make(chan string, len(contents))
	for _, content := range contents {
		content := content
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			response := performRequest(router, http.MethodPut, "/media/upload/"+created.ID+"/parts/0", bytes.NewReader(content))
			if response.Code != http.StatusOK {
				errors <- fmt.Sprintf("status %d: %s", response.Code, response.Body.String())
			}
		}()
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}

	response := performRequest(router, http.MethodPost, "/media/upload/"+created.ID+"/complete", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("complete returned %d: %s", response.Code, response.Body.String())
	}
}

func TestCompleteUploadPersistsMedia(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	content := []byte("persisted media")
	repository := &recordingMediaRepository{}
	service := newTestService(
		t,
		filepath.Join(root, "tmp"),
		filepath.Join(root, "media"),
		int64(len(content)),
		WithMediaRepository(repository),
	)
	router := newTestRouter(t, service)
	created := createUpload(t, router, fmt.Sprintf(`{"filename":"video.mp4","size":%d}`, len(content)))

	response := performRequest(router, http.MethodPut, "/media/upload/"+created.ID+"/parts/0", bytes.NewReader(content))
	if response.Code != http.StatusOK {
		t.Fatalf("upload part returned %d: %s", response.Code, response.Body.String())
	}
	response = performRequest(router, http.MethodPost, "/media/upload/"+created.ID+"/complete", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("complete returned %d: %s", response.Code, response.Body.String())
	}
	if len(repository.media) != 1 {
		t.Fatalf("got %d media records, want 1", len(repository.media))
	}
	media := repository.media[0]
	if media.UploadID != created.ID || media.MediaPath == "" || media.SHA256 == "" {
		t.Fatalf("incomplete media record: %#v", media)
	}
}

func newTestService(t *testing.T, tmpDir string, mediaDir string, maxUploadSize int64, options ...Option) IUploaderService {
	t.Helper()
	options = append([]Option{WithMaxUploadSize(maxUploadSize)}, options...)
	service, err := New(tmpDir, mediaDir, options...)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return service
}

func newTestRouter(t *testing.T, service IUploaderService) *gin.Engine {
	t.Helper()
	router, err := SetupRouter(gin.New(), service)
	if err != nil {
		t.Fatalf("setup router: %v", err)
	}
	return router
}

func createUpload(t *testing.T, router http.Handler, body string) createResponse {
	t.Helper()
	response := performRequest(router, http.MethodPost, "/media/upload", bytes.NewBufferString(body))
	if response.Code != http.StatusCreated {
		t.Fatalf("create returned %d: %s", response.Code, response.Body.String())
	}
	var created createResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	return created
}

func performRequest(handler http.Handler, method string, target string, body io.Reader) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, body)
	if method == http.MethodPost && body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
