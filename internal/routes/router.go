package routes

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kha/foods-drinks/internal/handler"
	"github.com/kha/foods-drinks/internal/middleware"
)

// UploadURLPrefix is the public route prefix under which uploaded files are served.
// It must match the first argument passed to router.Static below.
const UploadURLPrefix = "/uploads"

// RouterDependencies holds all dependencies for router setup
type RouterDependencies struct {
	HealthHandler  *handler.HealthHandler
	AuthHandler    *handler.AuthHandler
	OAuthHandler   *handler.OAuthHandler
	ProfileHandler *handler.ProfileHandler
	CorsMiddleware gin.HandlerFunc
	AuthMiddleware *middleware.AuthMiddleware
	UploadPath     string
}

func SetupRouter(deps *RouterDependencies) *gin.Engine {
	router := gin.New()

	// Global middleware - order matters!
	router.Use(deps.CorsMiddleware) // CORS first
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Serve uploaded files as static content, but only when UploadPath is
	// configured AND resolves to a path within the current working directory.
	// This prevents a misconfigured UploadPath (e.g. "." or "/") from
	// accidentally exposing source code or system files.
	if deps.UploadPath != "" {
		if safe, absPath := isSafeUploadPath(deps.UploadPath); safe {
			router.Static(UploadURLPrefix, absPath)
		} else {
			log.Fatalf("Unsafe upload path configured (%q): must be a subdirectory of the working directory", deps.UploadPath)
		}
	}

	// Health check (public)
	router.GET("/health", deps.HealthHandler.HealthCheck)
	router.GET("/", deps.HealthHandler.Welcome)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", deps.AuthHandler.Register)
			auth.POST("/login", deps.AuthHandler.Login)

			// OAuth routes - use specific prefix to avoid routing conflicts
			oauth := auth.Group("/oauth")
			{
				oauth.GET("/providers", deps.OAuthHandler.GetProviders)
				oauth.GET("/:provider", deps.OAuthHandler.InitiateOAuth)
				oauth.GET("/:provider/callback", deps.OAuthHandler.HandleCallback)
			}
		}

		// Public routes
		public := v1.Group("")
		{
			_ = public // TODO: Add public routes (products list, etc.)
		}

		// Protected routes (require authentication)
		protected := v1.Group("")
		protected.Use(deps.AuthMiddleware.RequireAuth())
		{
			// Profile routes
			protected.GET("/profile", deps.AuthHandler.GetProfile)
			protected.PUT("/profile", deps.AuthHandler.UpdateProfile)

			// Avatar routes
			protected.POST("/profile/avatar", deps.ProfileHandler.UploadAvatar)
			protected.DELETE("/profile/avatar", deps.ProfileHandler.DeleteAvatar)
		}

		// Admin routes (require admin role)
		admin := v1.Group("/admin")
		admin.Use(deps.AuthMiddleware.RequireAuth())
		admin.Use(deps.AuthMiddleware.RequireAdmin())
		{
			_ = admin // TODO: Add admin routes
		}
	}

	return router
}

// isSafeUploadPath resolves uploadPath to an absolute path and verifies it is a
// strict subdirectory of the current working directory. Returns (false, "") when
// the path is unsafe (e.g. ".", "/", or anything that escapes the working tree).
func isSafeUploadPath(uploadPath string) (bool, string) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, ""
	}

	absUpload, err := filepath.Abs(uploadPath)
	if err != nil {
		return false, ""
	}

	// Must be a strict subdirectory – not equal to cwd itself
	if !strings.HasPrefix(absUpload, cwd+string(filepath.Separator)) {
		return false, ""
	}

	return true, absUpload
}
