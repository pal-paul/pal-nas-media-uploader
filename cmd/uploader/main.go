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
	"pal-nas-media-uploader/app/autoalbum"
	store "pal-nas-media-uploader/app/db"
	"pal-nas-media-uploader/app/gallery"
	setupapp "pal-nas-media-uploader/app/setup"
	uploader "pal-nas-media-uploader/app/uploader"
	userapp "pal-nas-media-uploader/app/user"

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

	DatabaseURL     string `env:"ENV_DATABASE_URL"`
	AdminUsername   string `env:"ENV_ADMIN_USERNAME"`
	AdminPassword   string `env:"ENV_ADMIN_PASSWORD"`
	FrontendPages   string `env:"ENV_FRONTEND_PAGES_ENABLED,default=YES"`
	MediaDir        string `env:"ENV_MEDIA_DIR,default=./vol/medias"`
	TmpDir          string `env:"ENV_TMP_DIR,default=./vol/tmp"`
	MaxUploadSize   int64  `env:"ENV_MAX_UPLOAD_SIZE_BYTES,default=107374182400"`
	MaxPendingSize  int64  `env:"ENV_MAX_PENDING_UPLOAD_BYTES_PER_USER,default=107374182400"`
	MaxStorageSize  int64  `env:"ENV_MAX_STORAGE_BYTES_PER_USER,default=1099511627776"`
	MaxActive       int    `env:"ENV_MAX_ACTIVE_UPLOADS_PER_USER,default=10"`
	HTTPReadTimeout string `env:"ENV_HTTP_READ_TIMEOUT,default=5m"`
	AllowedOrigins  string `env:"ENV_CORS_ALLOWED_ORIGINS,default=http://localhost:3000"`
	Issuer          string `env:"ENV_ISSUER,default=issuer.palpaul.com"`
	AutoAlbumRunAt  string `env:"ENV_AUTO_ALBUM_RUN_AT,default=02:00"`
	AutoAlbumZone   string `env:"ENV_AUTO_ALBUM_TIMEZONE,default=Local"`
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
	readTimeout, err := time.ParseDuration(envVar.HTTPReadTimeout)
	if err != nil || readTimeout <= 0 {
		return fmt.Errorf("ENV_HTTP_READ_TIMEOUT must be a positive duration")
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
	frontendPagesEnabled, err := parseYesNo(envVar.FrontendPages)
	if err != nil {
		return err
	}
	setupService, err := setupapp.NewService(database, envVar.AdminUsername, envVar.AdminPassword, frontendPagesEnabled)
	if err != nil {
		return fmt.Errorf("create setup service: %w", err)
	}
	autoAlbumSchedule, err := autoalbum.NewSchedule(envVar.AutoAlbumRunAt, envVar.AutoAlbumZone)
	if err != nil {
		return err
	}
	autoAlbumService := autoalbum.NewService(database)
	uploaderService, err := uploader.New(
		envVar.TmpDir,
		envVar.MediaDir,
		uploader.WithMaxUploadSize(envVar.MaxUploadSize),
		uploader.WithMaxPendingUploadSize(envVar.MaxPendingSize),
		uploader.WithMaxUserStorageSize(envVar.MaxStorageSize),
		uploader.WithMaxActiveUploads(envVar.MaxActive),
		uploader.WithMediaRepository(database),
	)
	if err != nil {
		return fmt.Errorf("create uploader service: %w", err)
	}
	userService := userapp.NewService(database, envVar.Issuer, envVar.MediaDir, userapp.WithFrontendPages(frontendPagesEnabled))
	galleryService := gallery.NewService(database, envVar.MediaDir)

	// Setup router
	router = gin.Default()
	router.Use(corsMiddleware(envVar.AllowedOrigins))
	setupService.RegisterRoutes(router)
	router.POST("/auth/login", gin.WrapF(authService.Login))
	router.POST("/auth/verify", gin.WrapF(authService.VerifyTOTP))

	protected := router.Group("/")
	protected.Use(authService.RequireAuth())
	protected.GET("/auth/session", gin.WrapF(authService.Session))
	protected.POST("/auth/logout", gin.WrapF(authService.LogoutHandler))

	adminRoutes := protected.Group("/admin")
	adminRoutes.Use(userService.RequireRole("admin"))
	adminRoutes.GET("/config", userService.ConfigurationPage)
	adminRoutes.GET("/users", userService.ListUsers)
	adminRoutes.POST("/users", userService.CreateUser)

	userRoutes := protected.Group("/")
	userRoutes.Use(userService.RequireRole("user"))
	userRoutes.PUT("/users/me/folder", userService.SetMyFolder)
	userRoutes.GET("/media/files/:id/download", userService.DownloadMedia)
	userRoutes.POST("/media/files/:id/shares", userService.ShareMedia)
	userRoutes.DELETE("/media/files/:id/shares", userService.UnshareMedia)
	gallery.RegisterRoutes(userRoutes, galleryService)
	if err := uploader.RegisterRoutes(userRoutes, uploaderService); err != nil {
		return fmt.Errorf("register uploader routes: %w", err)
	}

	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	defer stopScheduler()
	go autoAlbumService.RunScheduled(schedulerCtx, autoAlbumSchedule, func(err error) {
		log.Printf("automatic album reconciliation failed: %v", err)
	})
	log.Printf("automatic albums scheduled for %s in %s", envVar.AutoAlbumRunAt, autoAlbumSchedule.Location)

	// Graceful shutdown
	srv := &http.Server{
		Addr:              ":" + envVar.Port,
		Handler:           router,
		ReadTimeout:       readTimeout,
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
	stopScheduler()
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

func parseYesNo(value string) (bool, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "YES":
		return true, nil
	case "NO":
		return false, nil
	default:
		return false, fmt.Errorf("ENV_FRONTEND_PAGES_ENABLED must be YES or NO")
	}
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
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, DELETE, PUT, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
