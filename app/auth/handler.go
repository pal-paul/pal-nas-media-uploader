package auth

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const sessionCookie = "pal_medias_uploader_session"
const UsernameContextKey = "auth.username"

func (handler *Service) Login(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	challenge, err := handler.BeginLogin(request.Context(), input.Username, input.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		writeError(writer, http.StatusUnauthorized, err)
		return
	}
	respond(writer, challenge, err)
}

func (handler *Service) VerifyTOTP(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		ChallengeToken string `json:"challengeToken"`
		Code           string `json:"code"`
	}
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	token, err := handler.Verify(request.Context(), input.ChallengeToken, input.Code)
	if errors.Is(err, ErrInvalidChallenge) || errors.Is(err, ErrInvalidCode) {
		writeError(writer, http.StatusUnauthorized, err)
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	http.SetCookie(writer, handler.authCookie(request, token, 30*24*60*60))
	writeJSON(writer, http.StatusOK, map[string]string{"status": "authenticated"})
}

func (handler *Service) Session(writer http.ResponseWriter, request *http.Request) {
	cookie, err := request.Cookie(sessionCookie)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	username, err := handler.Authenticate(request.Context(), cookie.Value)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"username": username})
}

func (handler *Service) LogoutHandler(writer http.ResponseWriter, request *http.Request) {
	cookie, _ := request.Cookie(sessionCookie)
	if cookie != nil {
		_ = handler.Logout(request.Context(), cookie.Value)
	}
	http.SetCookie(writer, handler.authCookie(request, "", -1))
	writer.WriteHeader(http.StatusNoContent)
}

func (handler *Service) authCookie(request *http.Request, value string, maxAge int) *http.Cookie {
	origin := request.Header.Get("Origin")
	secure := request.TLS != nil || request.Header.Get("X-Forwarded-Proto") == "https" || strings.HasPrefix(origin, "https://")
	sameSite := http.SameSiteStrictMode
	if secure && request.Header.Get("Origin") != "" {
		sameSite = http.SameSiteNoneMode
	}
	return &http.Cookie{Name: sessionCookie, Value: value, Path: "/", HttpOnly: true, Secure: secure, SameSite: sameSite, MaxAge: maxAge}
}

func (handler *Service) RequireAuth() gin.HandlerFunc {
	return func(context *gin.Context) {
		cookie, err := context.Request.Cookie(sessionCookie)
		if err != nil {
			writeError(context.Writer, http.StatusUnauthorized, ErrUnauthorized)
			context.Abort()
			return
		}
		username, err := handler.Authenticate(context.Request.Context(), cookie.Value)
		if err != nil {
			writeError(context.Writer, http.StatusUnauthorized, ErrUnauthorized)
			context.Abort()
			return
		}
		context.Set(UsernameContextKey, username)
		context.Next()
	}
}
func parseDate(value string) (*time.Time, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, errors.New("dates must use YYYY-MM-DD format")
	}
	return &parsed, nil
}

func decodeJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func respond(writer http.ResponseWriter, value any, err error) {
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func writeError(writer http.ResponseWriter, status int, err error) {
	slog.Error("request failed", "status", status, "error", err)
	writeJSON(writer, status, map[string]string{"error": http.StatusText(status)})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		slog.Error("encode response", "error", err)
	}
}
