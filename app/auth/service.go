package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid username or password")
var ErrInvalidChallenge = errors.New("invalid or expired authentication challenge")
var ErrInvalidCode = errors.New("invalid authentication code")
var ErrUnauthorized = errors.New("authentication required")

type Database interface {
	AuthUserCount(context.Context) (int, error)
	CreateAuthUser(context.Context, User) error
	GetAuthUserByUsername(context.Context, string) (User, error)
	CreateAuthChallenge(context.Context, string, string, string, time.Time) error
	GetAuthChallenge(context.Context, string) (Challenge, error)
	DeleteAuthChallenge(context.Context, string) error
	EnableTOTP(context.Context, string, string) error
	CreateAuthSession(context.Context, string, string, time.Time) error
	GetAuthSessionUsername(context.Context, string, time.Time) (string, error)
	DeleteAuthSession(context.Context, string) error
}

type Service struct {
	database Database
	issuer   string
}

func NewService(
	repository Database,
	issuer string,
) *Service {
	return &Service{database: repository, issuer: issuer}
}

func (service *Service) EnsureAdmin(ctx context.Context, username, password string) error {
	count, err := service.database.AuthUserCount(ctx)
	if err != nil || count > 0 {
		return err
	}
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 12 {
		return errors.New("ENV_ADMIN_USERNAME and ENV_ADMIN_PASSWORD (at least 12 characters) are required for first startup")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	return service.database.CreateAuthUser(ctx, User{ID: uuid.NewString(), Username: username, PasswordHash: string(hash)})
}

func (service *Service) BeginLogin(ctx context.Context, username, password string) (LoginChallenge, error) {
	user, err := service.database.GetAuthUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return LoginChallenge{}, ErrInvalidCredentials
	}
	pendingSecret := ""
	result := LoginChallenge{}
	if user.TOTPSecret == "" {
		key, err := totp.Generate(totp.GenerateOpts{Issuer: service.issuer, AccountName: user.Username})
		if err != nil {
			return result, fmt.Errorf("generate TOTP key: %w", err)
		}
		pendingSecret = key.Secret()
		result.ProvisioningURI = key.URL()
		result.Secret = key.Secret()
	}
	token, err := randomToken()
	if err != nil {
		return result, err
	}
	result.ChallengeToken = token
	if err := service.database.CreateAuthChallenge(ctx, tokenHash(token), user.ID, pendingSecret, time.Now().Add(5*time.Minute)); err != nil {
		return LoginChallenge{}, err
	}
	return result, nil
}

func (service *Service) Verify(ctx context.Context, challengeToken, code string) (string, error) {
	hash := tokenHash(challengeToken)
	challenge, err := service.database.GetAuthChallenge(ctx, hash)
	if err != nil || time.Now().After(challenge.ExpiresAt) {
		return "", ErrInvalidChallenge
	}
	secret := challenge.TOTPSecret
	if secret == "" {
		secret = challenge.PendingSecret
	}
	valid, err := totp.ValidateCustom(strings.TrimSpace(code), secret, time.Now(), totp.ValidateOpts{
		Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil || !valid {
		return "", ErrInvalidCode
	}
	if challenge.TOTPSecret == "" {
		if err := service.database.EnableTOTP(ctx, challenge.UserID, secret); err != nil {
			return "", err
		}
	}
	if err := service.database.DeleteAuthChallenge(ctx, hash); err != nil {
		return "", err
	}
	sessionToken, err := randomToken()
	if err != nil {
		return "", err
	}
	if err := service.database.CreateAuthSession(ctx, tokenHash(sessionToken), challenge.UserID, time.Now().Add(30*24*time.Hour)); err != nil {
		return "", err
	}
	return sessionToken, nil
}

func (service *Service) Authenticate(ctx context.Context, sessionToken string) (string, error) {
	if sessionToken == "" {
		return "", ErrUnauthorized
	}
	username, err := service.database.GetAuthSessionUsername(ctx, tokenHash(sessionToken), time.Now())
	if err != nil {
		return "", ErrUnauthorized
	}
	return username, nil
}

func (service *Service) Logout(ctx context.Context, sessionToken string) error {
	if sessionToken == "" {
		return nil
	}
	return service.database.DeleteAuthSession(ctx, tokenHash(sessionToken))
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate authentication token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
