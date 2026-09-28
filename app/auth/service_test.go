package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

type memoryRepository struct {
	user      User
	challenge Challenge
	session   string
}

func (repository *memoryRepository) AuthUserCount(context.Context) (int, error) {
	if repository.user.ID == "" {
		return 0, nil
	}
	return 1, nil
}
func (repository *memoryRepository) CreateAuthUser(_ context.Context, user User) error {
	repository.user = user
	return nil
}
func (repository *memoryRepository) GetAuthUserByUsername(_ context.Context, username string) (User, error) {
	if repository.user.Username != username {
		return User{}, errors.New("not found")
	}
	return repository.user, nil
}
func (repository *memoryRepository) CreateAuthChallenge(_ context.Context, _ string, userID, secret string, expiresAt time.Time) error {
	repository.challenge = Challenge{UserID: userID, Username: repository.user.Username, TOTPSecret: repository.user.TOTPSecret, PendingSecret: secret, ExpiresAt: expiresAt}
	return nil
}
func (repository *memoryRepository) GetAuthChallenge(context.Context, string) (Challenge, error) {
	return repository.challenge, nil
}
func (repository *memoryRepository) DeleteAuthChallenge(context.Context, string) error { return nil }
func (repository *memoryRepository) EnableTOTP(_ context.Context, _ string, secret string) error {
	repository.user.TOTPSecret = secret
	return nil
}
func (repository *memoryRepository) CreateAuthSession(_ context.Context, tokenHash, _ string, _ time.Time) error {
	repository.session = tokenHash
	return nil
}
func (repository *memoryRepository) GetAuthSessionUsername(_ context.Context, tokenHash string, _ time.Time) (string, error) {
	if tokenHash != repository.session {
		return "", errors.New("not found")
	}
	return repository.user.Username, nil
}
func (repository *memoryRepository) DeleteAuthSession(context.Context, string) error {
	repository.session = ""
	return nil
}

func TestEnrollmentCreatesAuthenticatedSession(t *testing.T) {
	ctx := context.Background()
	repository := &memoryRepository{}
	service := NewService(repository, "PAL Gallery Test")
	if err := service.EnsureAdmin(ctx, "admin", "correct-horse-battery"); err != nil {
		t.Fatal(err)
	}
	challenge, err := service.BeginLogin(ctx, "admin", "correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Secret == "" || challenge.ProvisioningURI == "" {
		t.Fatal("expected enrollment details")
	}
	code, err := totp.GenerateCode(challenge.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.Verify(ctx, challenge.ChallengeToken, code)
	if err != nil {
		t.Fatal(err)
	}
	username, err := service.Authenticate(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	if username != "admin" {
		t.Fatalf("got username %q", username)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	ctx := context.Background()
	repository := &memoryRepository{}
	service := NewService(repository, "PAL Gallery Test")
	if err := service.EnsureAdmin(ctx, "admin", "correct-horse-battery"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BeginLogin(ctx, "admin", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got %v", err)
	}
}
