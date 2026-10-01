package uploader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"
)

type service struct {
	tmpDir         string
	mediaDir       string
	maxUploadSize  int64
	maxPendingSize int64
	maxStorageSize int64
	maxActive      int
	mediaStore     MediaRepository
	uploads        map[string]*uploadSession
	uploadMux      sync.RWMutex
}

type IUploaderService interface {
	HandleCreateUpload(context *gin.Context)
	HandleUploadPart(context *gin.Context)
	HandleCompleteUpload(context *gin.Context)
	HandleGetUploadStatus(context *gin.Context)
	HandleDeleteUpload(context *gin.Context)
}

type Option func(*service) error

func WithMaxUploadSize(size int64) Option {
	return func(service *service) error {
		if size <= 0 {
			return fmt.Errorf("maximum upload size must be positive")
		}
		service.maxUploadSize = size
		return nil
	}
}

func WithMaxPendingUploadSize(size int64) Option {
	return func(service *service) error {
		if size <= 0 {
			return fmt.Errorf("maximum pending upload size must be positive")
		}
		service.maxPendingSize = size
		return nil
	}
}

func WithMaxUserStorageSize(size int64) Option {
	return func(service *service) error {
		if size <= 0 {
			return fmt.Errorf("maximum user storage size must be positive")
		}
		service.maxStorageSize = size
		return nil
	}
}

func WithMaxActiveUploads(count int) Option {
	return func(service *service) error {
		if count <= 0 {
			return fmt.Errorf("maximum active uploads must be positive")
		}
		service.maxActive = count
		return nil
	}
}

func WithMediaRepository(repository MediaRepository) Option {
	return func(service *service) error {
		if repository == nil {
			return fmt.Errorf("media repository is required")
		}
		service.mediaStore = repository
		return nil
	}
}

func New(tmpDir string, mediaDir string, options ...Option) (IUploaderService, error) {
	if tmpDir == "" || mediaDir == "" {
		return nil, fmt.Errorf("temporary and media directories are required")
	}

	service := &service{
		tmpDir:         filepath.Clean(tmpDir),
		mediaDir:       filepath.Clean(mediaDir),
		maxUploadSize:  defaultMaxUploadSize,
		maxPendingSize: defaultMaxPendingSize,
		maxStorageSize: defaultMaxStorageSize,
		maxActive:      defaultMaxActive,
		uploads:        make(map[string]*uploadSession),
	}
	for _, option := range options {
		if err := option(service); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(service.tmpDir, 0750); err != nil {
		return nil, fmt.Errorf("create temporary directory: %w", err)
	}
	if err := os.MkdirAll(service.mediaDir, 0750); err != nil {
		return nil, fmt.Errorf("create media directory: %w", err)
	}
	if err := service.loadUploads(); err != nil {
		return nil, err
	}
	return service, nil
}

func (s *service) loadUploads() error {
	entries, err := os.ReadDir(s.tmpDir)
	if err != nil {
		return fmt.Errorf("read temporary directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metadataPath := filepath.Join(s.tmpDir, entry.Name(), metadataFilename)
		file, err := os.Open(metadataPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("open upload metadata %q: %w", entry.Name(), err)
		}
		var upload Upload
		decodeErr := json.NewDecoder(file).Decode(&upload)
		closeErr := file.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode upload metadata %q: %w", entry.Name(), decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close upload metadata %q: %w", entry.Name(), closeErr)
		}
		if upload.ID != entry.Name() || upload.Size <= 0 || upload.Size > s.maxUploadSize || upload.Chunks <= 0 {
			return fmt.Errorf("invalid upload metadata %q", entry.Name())
		}
		s.uploads[upload.ID] = &uploadSession{upload: upload}
	}
	return nil
}

func (s *service) writeMetadata(upload *Upload) error {
	dir := filepath.Join(s.tmpDir, upload.ID)
	temporary, err := os.CreateTemp(dir, ".upload-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryName)
		}
	}()

	if err := json.NewEncoder(temporary).Encode(upload); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, filepath.Join(dir, metadataFilename)); err != nil {
		return err
	}
	removeTemporary = false
	return nil
}
