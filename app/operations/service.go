package operations

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Repository interface {
	Ping(context.Context) error
	CleanupExpiredAuth(context.Context, time.Time) (int64, error)
	ListMediaPaths(context.Context) ([]string, error)
}

type UploadManager interface {
	CleanupExpiredUploads(time.Time, time.Duration) (int, error)
}

type Service struct {
	repository  Repository
	uploads     UploadManager
	mediaDir    string
	tmpDir      string
	minDiskFree int64
	diskFree    func(string) (int64, error)
}

type CleanupResult struct {
	AuthRecords int64 `json:"authRecords"`
	Uploads     int   `json:"uploads"`
}

type IntegrityReport struct {
	FilesWithoutRecords []string  `json:"filesWithoutRecords"`
	RecordsWithoutFiles []string  `json:"recordsWithoutFiles"`
	CheckedAt           time.Time `json:"checkedAt"`
}

func New(repository Repository, uploads UploadManager, mediaDir, tmpDir string, minDiskFree int64) (*Service, error) {
	if repository == nil || uploads == nil {
		return nil, fmt.Errorf("operations repository and upload manager are required")
	}
	if minDiskFree < 0 {
		return nil, fmt.Errorf("minimum disk free size cannot be negative")
	}
	return &Service{
		repository: repository, uploads: uploads,
		mediaDir: filepath.Clean(mediaDir), tmpDir: filepath.Clean(tmpDir),
		minDiskFree: minDiskFree, diskFree: availableDiskBytes,
	}, nil
}

func (service *Service) Liveness(context *gin.Context) {
	context.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (service *Service) Readiness(context *gin.Context) {
	checkContext, cancel := contextWithTimeout(context.Request.Context(), 3*time.Second)
	defer cancel()
	databaseReady := service.repository.Ping(checkContext) == nil
	mediaFree, mediaErr := service.diskFree(service.mediaDir)
	tmpFree, tmpErr := service.diskFree(service.tmpDir)
	storageReady := mediaErr == nil && tmpErr == nil && mediaFree >= service.minDiskFree && tmpFree >= service.minDiskFree
	status := http.StatusOK
	state := "ready"
	if !databaseReady || !storageReady {
		status = http.StatusServiceUnavailable
		state = "not_ready"
	}
	context.JSON(status, gin.H{
		"status": state, "database": databaseReady, "storage": storageReady,
		"mediaFreeBytes": mediaFree, "temporaryFreeBytes": tmpFree,
	})
}

func (service *Service) Integrity(context *gin.Context) {
	report, err := service.ScanIntegrity(context.Request.Context())
	if err != nil {
		_ = context.Error(err)
		context.JSON(http.StatusInternalServerError, gin.H{"error": "integrity scan failed"})
		return
	}
	context.JSON(http.StatusOK, report)
}

func (service *Service) Cleanup(ctx context.Context, now time.Time, uploadRetention time.Duration) (CleanupResult, error) {
	authRecords, authErr := service.repository.CleanupExpiredAuth(ctx, now)
	uploads, uploadErr := service.uploads.CleanupExpiredUploads(now, uploadRetention)
	return CleanupResult{AuthRecords: authRecords, Uploads: uploads}, errors.Join(authErr, uploadErr)
}

func (service *Service) ScanIntegrity(ctx context.Context) (IntegrityReport, error) {
	paths, err := service.repository.ListMediaPaths(ctx)
	if err != nil {
		return IntegrityReport{}, err
	}
	recorded := make(map[string]struct{}, len(paths))
	report := IntegrityReport{CheckedAt: time.Now().UTC()}
	for _, path := range paths {
		path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
		recorded[path] = struct{}{}
		absolute, err := service.mediaPath(path)
		if err != nil {
			return IntegrityReport{}, err
		}
		if _, err := os.Stat(absolute); errors.Is(err, os.ErrNotExist) {
			report.RecordsWithoutFiles = append(report.RecordsWithoutFiles, path)
		} else if err != nil {
			return IntegrityReport{}, err
		}
	}
	err = filepath.WalkDir(service.mediaDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".complete-") {
			return nil
		}
		relative, err := filepath.Rel(service.mediaDir, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if _, ok := recorded[relative]; !ok {
			report.FilesWithoutRecords = append(report.FilesWithoutRecords, relative)
		}
		return nil
	})
	if err != nil {
		return IntegrityReport{}, err
	}
	sort.Strings(report.FilesWithoutRecords)
	sort.Strings(report.RecordsWithoutFiles)
	return report, nil
}

func (service *Service) mediaPath(relative string) (string, error) {
	path := filepath.Join(service.mediaDir, filepath.FromSlash(relative))
	cleanRelative, err := filepath.Rel(service.mediaDir, path)
	if err != nil || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) || filepath.IsAbs(cleanRelative) {
		return "", fmt.Errorf("media path %q escapes storage root", relative)
	}
	return path, nil
}

var contextWithTimeout = context.WithTimeout
