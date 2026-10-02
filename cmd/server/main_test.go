package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name           string
		origin         string
		fetchSite      string
		allowedOrigins string
		wantStatus     int
	}{
		{name: "same origin", origin: "http://127.0.0.1:8081", wantStatus: http.StatusNoContent},
		{name: "null same origin navigation", origin: "null", fetchSite: "same-origin", wantStatus: http.StatusNoContent},
		{name: "null cross site navigation", origin: "null", fetchSite: "cross-site", wantStatus: http.StatusForbidden},
		{name: "explicitly allowed", origin: "http://localhost:3000", allowedOrigins: "http://localhost:3000", wantStatus: http.StatusNoContent},
		{name: "foreign origin", origin: "https://example.com", wantStatus: http.StatusForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(corsMiddleware(test.allowedOrigins))
			router.GET("/asset", func(context *gin.Context) {
				context.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081/asset", nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Sec-Fetch-Site", test.fetchSite)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}
