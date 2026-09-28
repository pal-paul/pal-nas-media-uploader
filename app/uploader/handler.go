package uploader

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *service) HandleCreateUpload(context *gin.Context) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maxCreateRequestSize)
	var req CreateUploadRequest
	if err := context.ShouldBindJSON(&req); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload request"})
		return
	}
	filename, err := sanitizeFilename(req.Filename)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Size <= 0 || req.Size > s.maxUploadSize {
		context.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("size must be between 1 and %d bytes", s.maxUploadSize)})
		return
	}
	checksum := strings.ToLower(strings.TrimSpace(req.SHA256))
	if checksum != "" {
		decoded, decodeErr := hex.DecodeString(checksum)
		if decodeErr != nil || len(decoded) != sha256.Size {
			context.JSON(http.StatusBadRequest, gin.H{"error": "sha256 must be a 64-character hexadecimal value"})
			return
		}
	}
	if len(req.MimeType) > 255 {
		context.JSON(http.StatusBadRequest, gin.H{"error": "mime_type is too long"})
		return
	}

	id, err := generateUniqueID()
	if err != nil {
		internalError(context, fmt.Errorf("generate upload ID: %w", err))
		return
	}

	chunks := (req.Size + chunkSize - 1) / chunkSize
	u := &Upload{
		ID:       id,
		Filename: filename,
		MimeType: req.MimeType,
		Size:     req.Size,
		SHA256:   checksum,
		Chunks:   chunks,
		Created:  time.Now().UTC(),
	}
	dir := filepath.Join(s.tmpDir, id)
	if err := os.Mkdir(dir, 0750); err != nil {
		internalError(context, fmt.Errorf("create upload directory: %w", err))
		return
	}
	if err := s.writeMetadata(u); err != nil {
		_ = os.RemoveAll(dir)
		internalError(context, fmt.Errorf("write upload metadata: %w", err))
		return
	}
	s.uploadMux.Lock()
	s.uploads[id] = &uploadSession{upload: *u}
	s.uploadMux.Unlock()
	context.JSON(http.StatusCreated, gin.H{"id": id, "chunk_size": chunkSize, "chunks": chunks})
}

func (s *service) HandleUploadPart(context *gin.Context) {
	session, ok := s.getSession(context.Param("id"))
	if !ok {
		context.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
		return
	}
	session.mutex.RLock()
	defer session.mutex.RUnlock()
	if session.deleted {
		context.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
		return
	}
	u := &session.upload
	part, err := strconv.ParseInt(context.Param("part"), 10, 64)
	if err != nil || part < 0 || part >= u.Chunks {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid part"})
		return
	}
	expected := chunkSize
	if part == u.Chunks-1 {
		expected = u.Size - part*chunkSize
	}
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, expected+1)
	path := filepath.Join(s.tmpDir, u.ID, fmt.Sprintf("%08d.part", part))
	f, err := os.CreateTemp(filepath.Dir(path), fmt.Sprintf(".%08d-*.tmp", part))
	if err != nil {
		internalError(context, fmt.Errorf("create temporary chunk: %w", err))
		return
	}
	tmp := f.Name()
	n, copyErr := io.Copy(f, io.LimitReader(context.Request.Body, expected+1))
	if copyErr == nil && n == expected {
		copyErr = f.Sync()
	}
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		internalError(context, fmt.Errorf("write chunk: %w", firstError(copyErr, closeErr)))
		return
	}
	if n != expected {
		_ = os.Remove(tmp)
		context.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("expected %d bytes, received %d", expected, n)})
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		internalError(context, fmt.Errorf("commit chunk: %w", err))
		return
	}
	context.JSON(http.StatusOK, gin.H{"upload_id": u.ID, "part": part, "size": n})
}
func (s *service) HandleCompleteUpload(context *gin.Context) {
	session, ok := s.getSession(context.Param("id"))
	if !ok {
		context.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
		return
	}
	session.mutex.Lock()
	defer session.mutex.Unlock()
	if session.deleted {
		context.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
		return
	}
	u := &session.upload
	dir := filepath.Join(s.tmpDir, u.ID)
	if u.MediaPath == "" {
		now := time.Now().UTC()
		u.MediaPath = filepath.Join(now.Format("2006"), now.Format("01"), u.ID+"_"+u.Filename)
		if err := s.writeMetadata(u); err != nil {
			u.MediaPath = ""
			internalError(context, fmt.Errorf("persist media path: %w", err))
			return
		}
	}
	finalPath := filepath.Join(s.mediaDir, u.MediaPath)
	outDir := filepath.Dir(finalPath)
	if err := os.MkdirAll(outDir, 0750); err != nil {
		internalError(context, fmt.Errorf("create media directory: %w", err))
		return
	}
	out, err := os.CreateTemp(outDir, ".complete-*.tmp")
	if err != nil {
		internalError(context, fmt.Errorf("create destination file: %w", err))
		return
	}
	tmp := out.Name()
	hash := sha256.New()
	writer := io.MultiWriter(out, hash)
	var total int64
	for i := int64(0); i < u.Chunks; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%08d.part", i))
		part, err := os.Open(p)
		if err != nil {
			_ = out.Close()
			_ = os.Remove(tmp)
			context.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("missing part %d", i)})
			return
		}
		n, err := io.Copy(writer, part)
		_ = part.Close()
		if err != nil {
			_ = out.Close()
			_ = os.Remove(tmp)
			internalError(context, fmt.Errorf("copy part %d: %w", i, err))
			return
		}
		total += n
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		internalError(context, fmt.Errorf("sync completed upload: %w", err))
		return
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		internalError(context, fmt.Errorf("close completed upload: %w", err))
		return
	}
	if total != u.Size {
		_ = os.Remove(tmp)
		context.JSON(http.StatusBadRequest, gin.H{"error": "final size mismatch"})
		return
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	if u.SHA256 != "" && !strings.EqualFold(u.SHA256, sum) {
		_ = os.Remove(tmp)
		context.JSON(http.StatusBadRequest, gin.H{"error": "SHA-256 mismatch"})
		return
	}
	if err := os.Rename(tmp, finalPath); err != nil {
		_ = os.Remove(tmp)
		internalError(context, fmt.Errorf("commit completed upload: %w", err))
		return
	}
	if s.mediaStore != nil {
		media := CompletedMedia{
			UploadID:  u.ID,
			Filename:  u.Filename,
			MimeType:  u.MimeType,
			Size:      total,
			SHA256:    sum,
			MediaPath: filepath.ToSlash(u.MediaPath),
			CreatedAt: u.Created,
		}
		if err := s.mediaStore.SaveCompletedMedia(context.Request.Context(), media); err != nil {
			internalError(context, fmt.Errorf("save completed media: %w", err))
			return
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		_ = context.Error(fmt.Errorf("remove completed upload chunks: %w", err))
	}
	session.deleted = true
	s.uploadMux.Lock()
	if s.uploads[u.ID] == session {
		delete(s.uploads, u.ID)
	}
	s.uploadMux.Unlock()
	context.JSON(http.StatusOK, gin.H{"status": "completed", "media_path": filepath.ToSlash(u.MediaPath), "size": total, "sha256": sum})
}

func (s *service) HandleGetUploadStatus(context *gin.Context) {
	session, ok := s.getSession(context.Param("id"))
	if !ok {
		context.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
		return
	}
	session.mutex.RLock()
	defer session.mutex.RUnlock()
	if session.deleted {
		context.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
		return
	}
	u := &session.upload
	var parts []int64
	for i := int64(0); i < u.Chunks; i++ {
		if _, err := os.Stat(filepath.Join(s.tmpDir, u.ID, fmt.Sprintf("%08d.part", i))); err == nil {
			parts = append(parts, i)
		}
	}
	context.JSON(http.StatusOK, gin.H{"id": u.ID, "filename": u.Filename, "size": u.Size, "chunks": u.Chunks, "uploaded_parts": parts})
}
func (s *service) HandleDeleteUpload(context *gin.Context) {
	id := context.Param("id")
	session, ok := s.getSession(id)
	if !ok {
		context.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
		return
	}
	session.mutex.Lock()
	defer session.mutex.Unlock()
	if session.deleted {
		context.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
		return
	}
	if err := os.RemoveAll(filepath.Join(s.tmpDir, id)); err != nil {
		internalError(context, fmt.Errorf("delete upload: %w", err))
		return
	}
	session.deleted = true
	s.uploadMux.Lock()
	if s.uploads[id] == session {
		delete(s.uploads, id)
	}
	s.uploadMux.Unlock()
	context.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func generateUniqueID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *service) getSession(id string) (*uploadSession, bool) {
	s.uploadMux.RLock()
	defer s.uploadMux.RUnlock()
	session, ok := s.uploads[id]
	return session, ok
}

func sanitizeFilename(filename string) (string, error) {
	filename = filepath.Base(strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/"))
	if filename == "" || filename == "." || filename == ".." || filename == string(filepath.Separator) {
		return "", fmt.Errorf("filename is invalid")
	}
	if len(filename) > 255 {
		return "", fmt.Errorf("filename is too long")
	}
	return filename, nil
}

func firstError(errors ...error) error {
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}

func internalError(context *gin.Context, err error) {
	_ = context.Error(err)
	context.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
