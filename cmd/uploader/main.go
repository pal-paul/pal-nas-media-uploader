package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"pal-nas-media-uploader/app/auth"
	store "pal-nas-media-uploader/app/db"
	uploader "pal-nas-media-uploader/app/uploader"

	gin "github.com/gin-gonic/gin"
	env "github.com/pal-paul/go-libraries/pkg/env"
)

var (
	err    error
	envVar Environment
	router *gin.Engine
)

type Environment struct {
	Mode string `env:"ENV_GIN_MODE"`
	Port string `env:"ENV_PORT,default=8080"`

	DatabaseURL    string `env:"ENV_DATABASE_URL"`
	AdminUsername  string `env:"ENV_ADMIN_USERNAME"`
	AdminPassword  string `env:"ENV_ADMIN_PASSWORD"`
	MediaDir       string `env:"ENV_MEDIA_DIR,default=./vol/medias"`
	TmpDir         string `env:"ENV_TMP_DIR,default=./vol/tmp"`
	MaxUploadSize  int64  `env:"ENV_MAX_UPLOAD_SIZE_BYTES,default=107374182400"`
	AllowedOrigins string `env:"ENV_CORS_ALLOWED_ORIGINS,default=http://localhost:3000"`
	Issuer         string `env:"ENV_ISSUER,default=issuer.palpaul.com"`
}

// Initializing environment variables
func init() {
	_, err := env.Unmarshal(&envVar)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if strings.TrimSpace(envVar.DatabaseURL) == "" {
		return fmt.Errorf("ENV_DATABASE_URL is required")
	}

	ctx := context.Background()
	database, err := store.NewPostgres(ctx, envVar.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := database.Migrate(ctx); err != nil {
		return err
	}

	authService := auth.NewService(database, envVar.Issuer)
	if err := authService.EnsureAdmin(ctx, envVar.AdminUsername, envVar.AdminPassword); err != nil {
		return fmt.Errorf("ensure initial admin: %w", err)
	}
	uploaderService, err := uploader.New(
		envVar.TmpDir,
		envVar.MediaDir,
		uploader.WithMaxUploadSize(envVar.MaxUploadSize),
		uploader.WithMediaRepository(database),
	)
	if err != nil {
		return fmt.Errorf("create uploader service: %w", err)
	}

	// Setup router
	router = gin.Default()
	router.Use(corsMiddleware(envVar.AllowedOrigins))
	router.POST("/auth/login", gin.WrapF(authService.Login))
	router.POST("/auth/verify", gin.WrapF(authService.VerifyTOTP))

	protected := router.Group("/")
	protected.Use(authService.RequireAuth())
	protected.GET("/auth/session", gin.WrapF(authService.Session))
	protected.POST("/auth/logout", gin.WrapF(authService.LogoutHandler))
	if err := uploader.RegisterRoutes(protected, uploaderService); err != nil {
		return fmt.Errorf("register uploader routes: %w", err)
	}

	// Graceful shutdown
	srv := &http.Server{
		Addr:              ":" + envVar.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// Channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serverErrors := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("failed to start server: %w", err)
		}
	}()

	log.Println("server started successfully")
	log.Println("press Ctrl+C to stop")

	select {
	case err := <-serverErrors:
		return err
	case <-quit:
	}
	log.Println("shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	shutdownErr := srv.Shutdown(shutdownCtx)
	cancel()

	if shutdownErr != nil {
		return fmt.Errorf("server forced to shutdown: %w", shutdownErr)
	}

	log.Println("server stopped gracefully")
	return nil
}

// corsMiddleware handles CORS
func corsMiddleware(allowedOrigins string) gin.HandlerFunc {
	allowed := make(map[string]struct{})
	for _, origin := range strings.Split(allowedOrigins, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowed[origin]; origin != "" && !ok {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Vary", "Origin")
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Authorization, Accept, Cache-Control,  X-Client-Id")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, DELETE, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
