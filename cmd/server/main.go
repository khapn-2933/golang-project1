package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/handler"
	"github.com/kha/foods-drinks/internal/middleware"
	"github.com/kha/foods-drinks/internal/repository"
	"github.com/kha/foods-drinks/internal/routes"
	"github.com/kha/foods-drinks/internal/service"
	"github.com/kha/foods-drinks/pkg/database"
	customValidator "github.com/kha/foods-drinks/pkg/validator"
)

func main() {
	// Load config
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Register custom validators
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := customValidator.RegisterCustomValidators(v); err != nil {
			log.Fatalf("Failed to register custom validators: %v", err)
		}
	}

	// Set Gin mode based on environment
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Connect to database
	db, err := database.NewConnection(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	log.Println("Database connected successfully!")

	// Ensure upload directory exists (only when a path is configured)
	if cfg.Upload.Path != "" {
		if err := os.MkdirAll(cfg.Upload.Path, 0755); err != nil {
			log.Fatalf("Failed to create upload directory: %v", err)
		}
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	socialAuthRepo := repository.NewSocialAuthRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)

	// Initialize services
	authService := service.NewAuthService(userRepo, &cfg.JWT)
	oauthService := service.NewOAuthService(userRepo, socialAuthRepo, authService, &cfg.OAuth)
	profileService := service.NewProfileService(userRepo, &cfg.Upload, routes.UploadURLPrefix)
	categoryService := service.NewCategoryService(categoryRepo)

	// Initialize handlers
	healthHandler := handler.NewHealthHandler()
	authHandler := handler.NewAuthHandler(authService)
	oauthHandler := handler.NewOAuthHandler(oauthService)
	profileHandler := handler.NewProfileHandler(profileService)
	categoryHandler := handler.NewCategoryHandler(categoryService)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService)

	// Setup router with dependencies
	deps := &routes.RouterDependencies{
		HealthHandler:   healthHandler,
		AuthHandler:     authHandler,
		OAuthHandler:    oauthHandler,
		ProfileHandler:  profileHandler,
		CategoryHandler: categoryHandler,
		CorsMiddleware:  middleware.CORSConfig(),
		AuthMiddleware:  authMiddleware,
		UploadPath:      cfg.Upload.Path,
	}
	router := routes.SetupRouter(deps)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.App.Port)
	log.Printf("Server %s starting on %s", cfg.App.Name, addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
