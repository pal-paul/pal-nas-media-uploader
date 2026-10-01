package user

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pal-nas-media-uploader/app/auth"

	"github.com/gin-gonic/gin"
)

type memoryRepository struct {
	created          NewAccount
	folder           string
	passwordHash     string
	unsharedUsername string
}

func (repository *memoryRepository) ResetUserPassword(_ context.Context, _ string, passwordHash string) error {
	repository.passwordHash = passwordHash
	return nil
}
func (repository *memoryRepository) UpdateUserQuota(context.Context, string, *int64) error {
	return nil
}
func (repository *memoryRepository) UpdateTrashRetention(context.Context, string, int) error {
	return nil
}

func (repository *memoryRepository) CreateUser(_ context.Context, account NewAccount) error {
	repository.created = account
	return nil
}
func (repository *memoryRepository) ListUsers(context.Context) ([]Account, error) { return nil, nil }
func (repository *memoryRepository) UpdateUserFolder(_ context.Context, _ string, folder string) error {
	repository.folder = folder
	return nil
}
func (repository *memoryRepository) ListAccessibleMedia(context.Context, string) ([]Media, error) {
	return nil, nil
}
func (repository *memoryRepository) GetAccessibleMedia(context.Context, string, string) (Media, error) {
	return Media{}, nil
}
func (repository *memoryRepository) GetAccessibleThumbnailPath(context.Context, string, string) (string, error) {
	return "", nil
}
func (repository *memoryRepository) ShareMedia(context.Context, string, string, string) error {
	return nil
}
func (repository *memoryRepository) UnshareMedia(_ context.Context, _, _, username string) error {
	repository.unsharedUsername = username
	return nil
}

func TestAdminCreatesTOTPUser(t *testing.T) {
	repository := &memoryRepository{}
	service := NewService(repository, "PAL Test", t.TempDir())
	account, provisioningURI, err := service.CreateAccount(context.Background(), "admin", "alice", "correct-horse-battery", "user", "family/alice")
	if err != nil {
		t.Fatal(err)
	}
	if !account.TOTPEnabled || provisioningURI == "" || repository.created.TOTPSecret == "" {
		t.Fatalf("expected TOTP enrollment, got %#v", account)
	}
	if repository.created.UploadFolder != "family/alice" {
		t.Fatalf("got folder %q", repository.created.UploadFolder)
	}
}

func TestRegularUserCannotCreateAccounts(t *testing.T) {
	service := NewService(&memoryRepository{}, "PAL Test", t.TempDir())
	_, _, err := service.CreateAccount(context.Background(), "user", "alice", "correct-horse-battery", "user", "alice")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestAdminResetsPassword(t *testing.T) {
	repository := &memoryRepository{}
	service := NewService(repository, "PAL Test", t.TempDir())
	if err := service.ResetPassword(context.Background(), "admin", "user-id", "new-correct-password"); err != nil {
		t.Fatal(err)
	}
	if repository.passwordHash == "" || repository.passwordHash == "new-correct-password" {
		t.Fatal("expected a password hash")
	}
	if err := service.ResetPassword(context.Background(), "user", "user-id", "another-correct-password"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("regular user reset returned %v", err)
	}
}

func TestFolderRejectsParentTraversal(t *testing.T) {
	service := NewService(&memoryRepository{}, "PAL Test", t.TempDir())
	err := service.SetUploadFolder(context.Background(), "id", "user", "../private")
	if !errors.Is(err, ErrInvalidFolder) {
		t.Fatalf("got %v", err)
	}
}

func TestUnshareAcceptsUsernameQueryAndLegacyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name string
		url  string
		body string
	}{
		{name: "query", url: "/media/files/media-id/shares?username=bob"},
		{name: "legacy body", url: "/media/files/media-id/shares", body: `{"username":"bob"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &memoryRepository{}
			service := NewService(repository, "PAL Test", t.TempDir())
			request := httptest.NewRequest(http.MethodDelete, test.url, strings.NewReader(test.body))
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = request
			context.Params = gin.Params{{Key: "id", Value: "media-id"}}
			context.Set(auth.UserContextKey, auth.User{ID: "owner-id", Role: "user"})

			service.UnshareMedia(context)

			if context.Writer.Status() != http.StatusNoContent || repository.unsharedUsername != "bob" {
				t.Fatalf("status=%d username=%q", context.Writer.Status(), repository.unsharedUsername)
			}
		})
	}
}
