package operations

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type testRepository struct {
	pingErr        error
	mediaPaths     []string
	deletedRecords int64
}

func (repository *testRepository) Ping(context.Context) error { return repository.pingErr }
func (repository *testRepository) CleanupExpiredAuth(context.Context, time.Time) (int64, error) {
	return repository.deletedRecords, nil
}
func (repository *testRepository) ListMediaPaths(context.Context) ([]string, error) {
	return repository.mediaPaths, nil
}

type testUploads struct{ removed int }

func (uploads *testUploads) CleanupExpiredUploads(time.Time, time.Duration) (int, error) {
	return uploads.removed, nil
}

func TestReadinessChecksDatabaseAndStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	repository := &testRepository{}
	service, err := New(repository, &testUploads{}, root, root, 100)
	if err != nil {
		t.Fatal(err)
	}
	service.diskFree = func(string) (int64, error) { return 101, nil }
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)
	service.Readiness(context)
	if context.Writer.Status() != http.StatusOK {
		t.Fatalf("got %d, want %d", context.Writer.Status(), http.StatusOK)
	}

	repository.pingErr = errors.New("database unavailable")
	context, _ = gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)
	service.Readiness(context)
	if context.Writer.Status() != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want %d", context.Writer.Status(), http.StatusServiceUnavailable)
	}
}

func TestScanIntegrityReportsBothDirections(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "alice"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".thumbnails", "user-1"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "alice", "extra.jpg"), []byte("image"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".thumbnails", "user-1", "media-1.jpg"), []byte("image"), 0640); err != nil {
		t.Fatal(err)
	}
	service, err := New(&testRepository{mediaPaths: []string{"alice/missing.jpg"}}, &testUploads{}, root, t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	report, err := service.ScanIntegrity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.FilesWithoutRecords) != 1 || report.FilesWithoutRecords[0] != "alice/extra.jpg" {
		t.Fatalf("unexpected unrecorded files: %#v", report.FilesWithoutRecords)
	}
	if len(report.RecordsWithoutFiles) != 1 || report.RecordsWithoutFiles[0] != "alice/missing.jpg" {
		t.Fatalf("unexpected missing files: %#v", report.RecordsWithoutFiles)
	}
}

func TestCleanupCombinesResults(t *testing.T) {
	service, err := New(&testRepository{deletedRecords: 3}, &testUploads{removed: 2}, t.TempDir(), t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Cleanup(context.Background(), time.Now(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if result.AuthRecords != 3 || result.Uploads != 2 {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}
}
