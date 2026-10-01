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
	"sync"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid username or password")
var ErrInvalidChallenge = errors.New("invalid or expired authentication challenge")
var ErrInvalidCode = errors.New("invalid authentication code")
var ErrUnauthorized = errors.New("authentication required")

const (
	loginAccountAttempts = 5
	loginIPAttempts      = 20
	verifyTokenAttempts  = 6
	verifyIPAttempts     = 30
	attemptWindow        = 5 * time.Minute
)

type Database interface {
	GetAuthUserByUsername(context.Context, string) (User, error)
	CreateAuthChallenge(context.Context, string, string, string, time.Time) error
	GetAuthChallenge(context.Context, string) (Challenge, error)
	DeleteAuthChallenge(context.Context, string) (bool, error)
	EnableTOTP(context.Context, string, string) error
	CreateAuthSession(context.Context, string, string, time.Time) error
	GetAuthSessionUser(context.Context, string, time.Time) (User, error)
	DeleteAuthSession(context.Context, string) error
}

type Service struct {
	database           Database
	issuer             string
	attemptsMutex      sync.Mutex
	loginAttempts      map[string]attemptState
	verifyAttempts     map[string]attemptState
	lastAttemptCleanup time.Time
}

type attemptState struct {
	count       int
	windowStart time.Time
}

func NewService(
	repository Database,
	issuer string,
) *Service {
	return &Service{
		database: repository, issuer: issuer,
		loginAttempts: make(map[string]attemptState), verifyAttempts: make(map[string]attemptState),
	}
}

func (service *Service) allowLogin(username, address string) bool {
	return service.attemptAllowed(service.loginAttempts, "account:"+strings.ToLower(strings.TrimSpace(username)), loginAccountAttempts) &&
		service.attemptAllowed(service.loginAttempts, "ip:"+address, loginIPAttempts)
}

func (service *Service) allowVerify(challengeToken, address string) bool {
	return service.attemptAllowed(service.verifyAttempts, "token:"+tokenHash(challengeToken), verifyTokenAttempts) &&
		service.attemptAllowed(service.verifyAttempts, "ip:"+address, verifyIPAttempts)
}

func (service *Service) attemptAllowed(attempts map[string]attemptState, key string, limit int) bool {
	service.attemptsMutex.Lock()
	defer service.attemptsMutex.Unlock()
	now := time.Now()
	state := attempts[key]
	if state.windowStart.IsZero() || now.Sub(state.windowStart) >= attemptWindow {
		delete(attempts, key)
		return true
	}
	return state.count < limit
}

func (service *Service) recordFailedAttempt(attempts map[string]attemptState, keys ...string) {
	service.attemptsMutex.Lock()
	defer service.attemptsMutex.Unlock()
	now := time.Now()
	if service.lastAttemptCleanup.IsZero() || now.Sub(service.lastAttemptCleanup) >= attemptWindow {
		for _, tracked := range []map[string]attemptState{service.loginAttempts, service.verifyAttempts} {
			for key, state := range tracked {
				if now.Sub(state.windowStart) >= attemptWindow {
					delete(tracked, key)
				}
			}
		}
		service.lastAttemptCleanup = now
	}
	for _, key := range keys {
		state := attempts[key]
		if state.windowStart.IsZero() || now.Sub(state.windowStart) >= attemptWindow {
			state = attemptState{windowStart: now}
		}
		state.count++
		attempts[key] = state
	}
}

func (service *Service) BeginLogin(ctx context.Context, username, password string) (LoginChallenge, error) {
	user, err := service.database.GetAuthUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return LoginChallenge{}, ErrInvalidCredentials
	}
	if !user.TOTPRequired {
		sessionToken, err := service.createSession(ctx, user.ID)
		if err != nil {
			return LoginChallenge{}, err
		}
		return LoginChallenge{Authenticated: true, SessionToken: sessionToken}, nil
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
	deleted, err := service.database.DeleteAuthChallenge(ctx, hash)
	if err != nil {
		return "", err
	}
	if !deleted {
		return "", ErrInvalidChallenge
	}
	return service.createSession(ctx, challenge.UserID)
}

func (service *Service) Authenticate(ctx context.Context, sessionToken string) (User, error) {
	if sessionToken == "" {
		return User{}, ErrUnauthorized
	}
	user, err := service.database.GetAuthSessionUser(ctx, tokenHash(sessionToken), time.Now())
	if err != nil {
		return User{}, ErrUnauthorized
	}
	return user, nil
}

func (service *Service) createSession(ctx context.Context, userID string) (string, error) {
	sessionToken, err := randomToken()
	if err != nil {
		return "", err
	}
	if err := service.database.CreateAuthSession(ctx, tokenHash(sessionToken), userID, time.Now().Add(30*24*time.Hour)); err != nil {
		return "", err
	}
	return sessionToken, nil
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
