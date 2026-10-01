package user

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

var ErrForbidden = errors.New("forbidden")
var ErrInvalidFolder = errors.New("upload folder must be a relative path without parent traversal")

//go:embed templates/config.html
var configurationTemplates embed.FS

var configurationTemplate = template.Must(template.ParseFS(configurationTemplates, "templates/config.html"))

type Repository interface {
	CreateUser(context.Context, NewAccount) error
	ListUsers(context.Context) ([]Account, error)
	UpdateUserFolder(context.Context, string, string) error
	ListAccessibleMedia(context.Context, string) ([]Media, error)
	GetAccessibleMedia(context.Context, string, string) (Media, error)
	ShareMedia(context.Context, string, string, string) error
	UnshareMedia(context.Context, string, string, string) error
}

type Service struct {
	repository    Repository
	issuer        string
	mediaDir      string
	frontendPages bool
}

type Option func(*Service)

func WithFrontendPages(enabled bool) Option {
	return func(service *Service) { service.frontendPages = enabled }
}

func NewService(repository Repository, issuer, mediaDir string, options ...Option) *Service {
	service := &Service{repository: repository, issuer: issuer, mediaDir: filepath.Clean(mediaDir), frontendPages: true}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *Service) CreateAccount(ctx context.Context, actorRole, username, password, role, folder string) (Account, string, error) {
	if actorRole != "admin" {
		return Account{}, "", ErrForbidden
	}
	username = strings.TrimSpace(username)
	role = strings.ToLower(strings.TrimSpace(role))
	if username == "" || len(password) < 12 {
		return Account{}, "", errors.New("username and a password of at least 12 characters are required")
	}
	if role != "admin" && role != "user" {
		return Account{}, "", errors.New("role must be admin or user")
	}
	folder, err := sanitizeFolder(folder)
	if err != nil {
		return Account{}, "", err
	}
	if role == "user" && folder == "" {
		return Account{}, "", errors.New("an upload folder is required for regular users")
	}
	if role == "admin" {
		folder = ""
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Account{}, "", fmt.Errorf("hash password: %w", err)
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: service.issuer, AccountName: username})
	if err != nil {
		return Account{}, "", fmt.Errorf("generate TOTP key: %w", err)
	}
	account := Account{ID: uuid.NewString(), Username: username, Role: role, UploadFolder: folder, TOTPEnabled: true}
	err = service.repository.CreateUser(ctx, NewAccount{
		ID: account.ID, Username: username, PasswordHash: string(hash), Role: role,
		UploadFolder: folder, TOTPSecret: key.Secret(),
	})
	if err != nil {
		return Account{}, "", err
	}
	return account, key.URL(), nil
}

func (service *Service) SetUploadFolder(ctx context.Context, userID, role, folder string) error {
	if role != "user" {
		return ErrForbidden
	}
	folder, err := sanitizeFolder(folder)
	if err != nil {
		return err
	}
	if folder == "" {
		return errors.New("upload folder is required")
	}
	return service.repository.UpdateUserFolder(ctx, userID, folder)
}

func sanitizeFolder(folder string) (string, error) {
	folder = filepath.ToSlash(filepath.Clean(strings.TrimSpace(folder)))
	if folder == "." || folder == "" {
		return "", nil
	}
	if filepath.IsAbs(folder) || folder == ".." || strings.HasPrefix(folder, "../") {
		return "", ErrInvalidFolder
	}
	return folder, nil
}
