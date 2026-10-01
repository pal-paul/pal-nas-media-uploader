package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

type memoryRepository struct {
	user               User
	challenge          Challenge
	challengeAvailable bool
	session            string
}

func (repository *memoryRepository) GetAuthUserByUsername(_ context.Context, username string) (User, error) {
	if repository.user.Username != username {
		return User{}, errors.New("not found")
	}
	return repository.user, nil
}
func (repository *memoryRepository) CreateAuthChallenge(_ context.Context, _ string, userID, secret string, expiresAt time.Time) error {
	repository.challenge = Challenge{UserID: userID, Username: repository.user.Username, TOTPSecret: repository.user.TOTPSecret, PendingSecret: secret, ExpiresAt: expiresAt}
	repository.challengeAvailable = true
	return nil
}
func (repository *memoryRepository) GetAuthChallenge(context.Context, string) (Challenge, error) {
	if !repository.challengeAvailable {
		return Challenge{}, errors.New("not found")
	}
	return repository.challenge, nil
}
func (repository *memoryRepository) DeleteAuthChallenge(context.Context, string) (bool, error) {
	if !repository.challengeAvailable {
		return false, nil
	}
	repository.challengeAvailable = false
	return true, nil
}
func (repository *memoryRepository) EnableTOTP(_ context.Context, _ string, secret string) error {
	repository.user.TOTPSecret = secret
	return nil
}
func (repository *memoryRepository) CreateAuthSession(_ context.Context, tokenHash, _ string, _ time.Time) error {
	repository.session = tokenHash
	return nil
}
func (repository *memoryRepository) GetAuthSessionUser(_ context.Context, tokenHash string, _ time.Time) (User, error) {
	if tokenHash != repository.session {
		return User{}, errors.New("not found")
	}
	return repository.user, nil
}
func (repository *memoryRepository) DeleteAuthSession(context.Context, string) error {
	repository.session = ""
	return nil
}

func TestEnrollmentCreatesAuthenticatedSession(t *testing.T) {
	ctx := context.Background()
	repository := &memoryRepository{user: passwordUser(t, true)}
	service := NewService(repository, "PAL Gallery Test")
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
	user, err := service.Authenticate(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != "admin" {
		t.Fatalf("got username %q", user.Username)
	}
}

func TestChallengeCannotBeReused(t *testing.T) {
	ctx := context.Background()
	repository := &memoryRepository{user: passwordUser(t, true)}
	service := NewService(repository, "PAL Gallery Test")
	challenge, err := service.BeginLogin(ctx, "admin", "correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(challenge.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Verify(ctx, challenge.ChallengeToken, code); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Verify(ctx, challenge.ChallengeToken, code); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("got %v, want invalid challenge", err)
	}
}

func TestAdminLoginDoesNotRequireTOTP(t *testing.T) {
	ctx := context.Background()
	repository := &memoryRepository{user: passwordUser(t, false)}
	service := NewService(repository, "PAL Gallery Test")
	result, err := service.BeginLogin(ctx, "admin", "correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.SessionToken == "" || result.ChallengeToken != "" {
		t.Fatalf("unexpected login result: %#v", result)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	ctx := context.Background()
	repository := &memoryRepository{user: passwordUser(t, false)}
	service := NewService(repository, "PAL Gallery Test")
	if _, err := service.BeginLogin(ctx, "admin", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got %v", err)
	}
}

func passwordUser(t *testing.T, totpRequired bool) User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse-battery"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return User{ID: "admin-id", Username: "admin", PasswordHash: string(hash), Role: "admin", TOTPRequired: totpRequired}
}
