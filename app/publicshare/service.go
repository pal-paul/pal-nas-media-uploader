package publicshare

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pal-nas-media-uploader/app/auth"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type File struct {
	Filename  string `json:"filename"`
	MimeType  string `json:"mimeType"`
	MediaPath string `json:"-"`
	Size      int64  `json:"size"`
}

type Link struct {
	ID           string    `json:"id"`
	OwnerID      string    `json:"-"`
	Title        string    `json:"title"`
	PasswordHash string    `json:"-"`
	ExpiresAt    time.Time `json:"expiresAt"`
	Files        []File    `json:"files"`
}

type Repository interface {
	CreatePublicLink(context.Context, Link, string, string) error
	ResolvePublicLink(context.Context, string, time.Time) (Link, error)
	DeletePublicLink(context.Context, string, string) error
}

type attempt struct {
	Count       int
	LockedUntil time.Time
}

type Service struct {
	repository Repository
	mediaDir   string
	mutex      sync.Mutex
	attempts   map[string]attempt
}

func New(repository Repository, mediaDir string) *Service {
	return &Service{repository: repository, mediaDir: filepath.Clean(mediaDir), attempts: make(map[string]attempt)}
}

func (service *Service) Create(context *gin.Context) {
	identity, ok := context.Get(auth.UserContextKey)
	user, valid := identity.(auth.User)
	if !ok || !valid {
		context.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	var request struct {
		MediaID  string    `json:"mediaId"`
		AlbumID  string    `json:"albumId"`
		Password string    `json:"password"`
		Expires  time.Time `json:"expiresAt"`
	}
	if err := context.ShouldBindJSON(&request); err != nil || (request.MediaID == "") == (request.AlbumID == "") || !request.Expires.After(time.Now()) {
		context.JSON(http.StatusBadRequest, gin.H{"error": "one mediaId or albumId and a future expiresAt are required"})
		return
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	link := Link{ID: tokenHash(token), OwnerID: user.ID, ExpiresAt: request.Expires}
	if request.Password != "" {
		if len(request.Password) < 8 {
			context.JSON(http.StatusBadRequest, gin.H{"error": "share password must be at least 8 characters"})
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
		if err != nil {
			context.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		link.PasswordHash = string(hash)
	}
	if err := service.repository.CreatePublicLink(context, link, request.MediaID, request.AlbumID); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "unable to create public link"})
		return
	}
	context.JSON(http.StatusCreated, gin.H{"id": link.ID, "token": token, "url": "/public/" + token, "expiresAt": link.ExpiresAt})
}

func (service *Service) Delete(context *gin.Context) {
	identity, _ := context.Get(auth.UserContextKey)
	user, ok := identity.(auth.User)
	if !ok || service.repository.DeletePublicLink(context, context.Param("id"), user.ID) != nil {
		context.JSON(http.StatusNotFound, gin.H{"error": "public link not found"})
		return
	}
	context.Status(http.StatusNoContent)
}

func (service *Service) Resolve(context *gin.Context) {
	link, ok := service.authorize(context)
	if !ok {
		return
	}
	context.JSON(http.StatusOK, gin.H{"title": link.Title, "expiresAt": link.ExpiresAt, "files": link.Files})
}

func (service *Service) Download(context *gin.Context) {
	link, ok := service.authorize(context)
	if !ok {
		return
	}
	if len(link.Files) == 1 {
		path, err := service.safePath(link.Files[0].MediaPath)
		if err != nil {
			context.JSON(http.StatusNotFound, gin.H{"error": "shared file not found"})
			return
		}
		context.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": link.Files[0].Filename}))
		context.File(path)
		return
	}
	context.Header("Content-Type", "application/zip")
	context.Header("Content-Disposition", "attachment; filename=\"shared-album.zip\"")
	writer := zip.NewWriter(context.Writer)
	defer writer.Close()
	for _, item := range link.Files {
		path, err := service.safePath(item.MediaPath)
		if err != nil {
			continue
		}
		input, err := os.Open(path)
		if err != nil {
			continue
		}
		entry, err := writer.Create(filepath.Base(item.Filename))
		if err == nil {
			_, _ = io.Copy(entry, input)
		}
		_ = input.Close()
	}
}

func (service *Service) authorize(context *gin.Context) (Link, bool) {
	token := context.Param("token")
	key := tokenHash(token) + ":" + context.ClientIP()
	service.mutex.Lock()
	state := service.attempts[key]
	locked := time.Now().Before(state.LockedUntil)
	service.mutex.Unlock()
	if locked {
		context.JSON(http.StatusTooManyRequests, gin.H{"error": "too many password attempts"})
		return Link{}, false
	}
	link, err := service.repository.ResolvePublicLink(context, tokenHash(token), time.Now())
	if err != nil {
		context.JSON(http.StatusNotFound, gin.H{"error": "public link not found"})
		return Link{}, false
	}
	if link.PasswordHash != "" && bcrypt.CompareHashAndPassword([]byte(link.PasswordHash), []byte(context.GetHeader("X-Share-Password"))) != nil {
		service.mutex.Lock()
		state.Count++
		if state.Count >= 3 {
			state.LockedUntil = time.Now().Add(5 * time.Minute)
			state.Count = 0
		}
		service.attempts[key] = state
		service.mutex.Unlock()
		context.JSON(http.StatusUnauthorized, gin.H{"error": "share password required"})
		return Link{}, false
	}
	service.mutex.Lock()
	delete(service.attempts, key)
	service.mutex.Unlock()
	return link, true
}

func (service *Service) safePath(relative string) (string, error) {
	path := filepath.Join(service.mediaDir, filepath.FromSlash(relative))
	resolved, err := filepath.Rel(service.mediaDir, path)
	if err != nil || resolved == ".." || strings.HasPrefix(resolved, ".."+string(filepath.Separator)) || filepath.IsAbs(resolved) {
		return "", errors.New("invalid media path")
	}
	return path, nil
}

func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
