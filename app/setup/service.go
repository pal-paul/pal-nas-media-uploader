package setup

import (
	"context"
	"crypto/subtle"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrAlreadyConfigured = errors.New("initial administrator is already configured")
	ErrInvalidBootstrap  = errors.New("invalid bootstrap credentials")
	ErrInvalidAdmin      = errors.New("admin username and a password of at least 12 characters are required")
	ErrPasswordMismatch  = errors.New("admin passwords do not match")
)

//go:embed templates/setup.html
var templateFiles embed.FS

type Account struct {
	ID           string
	Username     string
	PasswordHash string
}

type Repository interface {
	SetupComplete(context.Context) (bool, error)
	CreateInitialAdmin(context.Context, Account) error
}

type Service struct {
	repository        Repository
	bootstrapUsername string
	bootstrapPassword string
	frontendEnabled   bool
	template          *template.Template
}

func NewService(repository Repository, bootstrapUsername, bootstrapPassword string, frontendEnabled bool) (*Service, error) {
	bootstrapUsername = strings.TrimSpace(bootstrapUsername)
	if bootstrapUsername == "" || len(bootstrapPassword) < 12 {
		return nil, errors.New("bootstrap username and a password of at least 12 characters are required")
	}
	page, err := template.ParseFS(templateFiles, "templates/setup.html")
	if err != nil {
		return nil, fmt.Errorf("parse setup template: %w", err)
	}
	return &Service{
		repository: repository, bootstrapUsername: bootstrapUsername,
		bootstrapPassword: bootstrapPassword, frontendEnabled: frontendEnabled,
		template: page,
	}, nil
}

func (service *Service) CreateAdmin(ctx context.Context, bootstrapUsername, bootstrapPassword, username, password, confirmation string) error {
	configured, err := service.repository.SetupComplete(ctx)
	if err != nil {
		return err
	}
	if configured {
		return ErrAlreadyConfigured
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(bootstrapUsername)), []byte(service.bootstrapUsername)) != 1 ||
		subtle.ConstantTimeCompare([]byte(bootstrapPassword), []byte(service.bootstrapPassword)) != 1 {
		return ErrInvalidBootstrap
	}
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 12 {
		return ErrInvalidAdmin
	}
	if password != confirmation {
		return ErrPasswordMismatch
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	return service.repository.CreateInitialAdmin(ctx, Account{
		ID: uuid.NewString(), Username: username, PasswordHash: string(hash),
	})
}

func (service *Service) RegisterRoutes(router gin.IRoutes) {
	router.GET("/setup", service.SetupPage)
	router.POST("/setup", service.SubmitSetup)
}

func (service *Service) SetupPage(context *gin.Context) {
	if !service.pageAvailable(context) {
		return
	}
	service.render(context, http.StatusOK, nil)
}

func (service *Service) SubmitSetup(context *gin.Context) {
	if !service.pageAvailable(context) {
		return
	}
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, 64<<10)
	if err := context.Request.ParseForm(); err != nil {
		service.render(context, http.StatusBadRequest, gin.H{"Error": "Invalid setup form."})
		return
	}
	err := service.CreateAdmin(
		context.Request.Context(),
		context.PostForm("bootstrapUsername"), context.PostForm("bootstrapPassword"),
		context.PostForm("username"), context.PostForm("password"), context.PostForm("confirmPassword"),
	)
	if err != nil {
		if errors.Is(err, ErrAlreadyConfigured) {
			context.Status(http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrInvalidBootstrap) {
			service.render(context, http.StatusUnauthorized, gin.H{"Error": err.Error()})
			return
		}
		if errors.Is(err, ErrInvalidAdmin) || errors.Is(err, ErrPasswordMismatch) {
			service.render(context, http.StatusBadRequest, gin.H{"Error": err.Error()})
			return
		}
		_ = context.Error(err)
		service.render(context, http.StatusInternalServerError, gin.H{"Error": "Unable to complete setup."})
		return
	}
	service.render(context, http.StatusCreated, gin.H{"Complete": true})
}

func (service *Service) pageAvailable(context *gin.Context) bool {
	if !service.frontendEnabled {
		context.Status(http.StatusNotFound)
		return false
	}
	configured, err := service.repository.SetupComplete(context.Request.Context())
	if err != nil {
		_ = context.Error(err)
		context.Status(http.StatusInternalServerError)
		return false
	}
	if configured {
		context.Status(http.StatusNotFound)
		return false
	}
	return true
}

func (service *Service) render(context *gin.Context, status int, data any) {
	context.Header("Content-Type", "text/html; charset=utf-8")
	context.Status(status)
	if err := service.template.ExecuteTemplate(context.Writer, "setup.html", data); err != nil {
		_ = context.Error(err)
	}
}
