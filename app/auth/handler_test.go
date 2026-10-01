package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoginRateLimit(t *testing.T) {
	service := NewService(&memoryRepository{}, "PAL Gallery Test")
	for attempt := 1; attempt <= loginAccountAttempts+1; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"username":"admin","password":"wrong"}`))
		request.RemoteAddr = "192.0.2.1:1234"
		response := httptest.NewRecorder()
		service.Login(response, request)
		want := http.StatusUnauthorized
		if attempt > loginAccountAttempts {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Fatalf("attempt %d: got status %d, want %d", attempt, response.Code, want)
		}
	}
}

func TestSessionRejectsMissingCookie(t *testing.T) {
	service := NewService(&memoryRepository{}, "PAL Gallery Test")
	request := httptest.NewRequest(http.MethodGet, "/session", nil)
	response := httptest.NewRecorder()

	service.Session(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuthRejectsMissingCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := NewService(&memoryRepository{}, "PAL Gallery Test")
	router := gin.New()
	router.GET("/protected", service.RequireAuth(), func(context *gin.Context) {
		context.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
