package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

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
