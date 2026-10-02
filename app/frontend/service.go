package frontend

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type Service struct {
	directory string
}

func New(directory string) (*Service, error) {
	directory = filepath.Clean(directory)
	if _, err := os.Stat(filepath.Join(directory, "index.html")); err != nil {
		return nil, fmt.Errorf("open gallery frontend: %w", err)
	}
	return &Service{directory: directory}, nil
}

func (service *Service) RegisterRoutes(router *gin.Engine) {
	router.GET("/", func(context *gin.Context) {
		context.File(filepath.Join(service.directory, "index.html"))
	})
	router.StaticFS("/assets", gin.Dir(filepath.Join(service.directory, "assets"), false))
	for _, name := range []string{"favicon.svg", "icon.png", "icons.svg"} {
		name := name
		router.GET("/"+name, func(context *gin.Context) {
			context.File(filepath.Join(service.directory, name))
		})
	}
	router.NoRoute(func(context *gin.Context) {
		context.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
}
