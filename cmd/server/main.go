package main

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"os"
	"strconv"

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
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := customValidator.RegisterCustomValidators(v); err != nil {
			log.Fatalf("Failed to register custom validators: %v", err)
		}
	}

	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

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
	productRepo := repository.NewProductRepository(db)
	cartRepo := repository.NewCartRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	orderNotificationRepo := repository.NewOrderNotificationRepository(db)
	ratingRepo := repository.NewRatingRepository(db)
	suggestionRepo := repository.NewSuggestionRepository(db)

	cartService := service.NewCartService(cartRepo, productRepo)
	authService := service.NewAuthService(userRepo, cartService, &cfg.JWT)
	oauthService := service.NewOAuthService(userRepo, socialAuthRepo, cartRepo, authService, &cfg.OAuth)
	profileService := service.NewProfileService(userRepo, &cfg.Upload, routes.UploadURLPrefix)
	categoryService := service.NewCategoryService(categoryRepo)
	productService := service.NewProductService(productRepo, categoryRepo)
	emailNotificationService := service.NewEmailNotificationService(&cfg.Email, orderNotificationRepo)
	chatworkNotificationService := service.NewChatworkNotificationService(&cfg.Chatwork, orderNotificationRepo)
	notifier := service.NewMultiOrderNotifier(emailNotificationService, chatworkNotificationService)
	orderService := service.NewOrderService(orderRepo, cartRepo, productRepo, notifier)
	ratingService := service.NewRatingService(ratingRepo, productRepo)
	suggestionService := service.NewSuggestionService(suggestionRepo, categoryRepo)

	funcMap := template.FuncMap{
		"inc": func(i int) int { return i + 1 },
		"dec": func(i int) int { return i - 1 },
		"formatVND": func(amount float64) string {
			value := int64(math.Round(amount))
			sign := ""
			if value < 0 {
				sign = "-"
				value = -value
			}

			raw := strconv.FormatInt(value, 10)
			n := len(raw)
			if n <= 3 {
				return sign + raw + "đ"
			}

			sepCount := (n - 1) / 3
			buf := make([]byte, n+sepCount)
			read := n - 1
			write := len(buf) - 1
			digitCount := 0

			for read >= 0 {
				buf[write] = raw[read]
				read--
				write--
				digitCount++

				if digitCount%3 == 0 && read >= 0 {
					buf[write] = '.'
					write--
				}
			}

			return sign + string(buf) + "đ"
		},
		"deref": func(s *string) string {
			if s == nil {
				return ""
			}
			return *s
		},
	}

	healthHandler := handler.NewHealthHandler()
	authHandler := handler.NewAuthHandler(authService)
	oauthHandler := handler.NewOAuthHandler(oauthService)
	profileHandler := handler.NewProfileHandler(profileService)
	adminCategoryHandler := handler.NewAdminCategoryHandler(categoryService, funcMap)
	productHandler := handler.NewProductHandler(productService)
	adminProductHandler := handler.NewAdminProductHandler(productService, categoryService, funcMap)
	adminOrderHandler := handler.NewAdminOrderHandler(orderService, funcMap)
	adminOrderStatsHandler := handler.NewAdminOrderStatisticsHandler(orderService, funcMap)
	adminSuggestionHandler := handler.NewAdminSuggestionHandler(suggestionService, funcMap)
	cartHandler := handler.NewCartHandler(cartService)
	orderHandler := handler.NewOrderHandler(orderService)
	ratingHandler := handler.NewRatingHandler(ratingService)
	suggestionHandler := handler.NewSuggestionHandler(suggestionService)

	authMiddleware := middleware.NewAuthMiddleware(authService)

	deps := &routes.RouterDependencies{
		HealthHandler:          healthHandler,
		AuthHandler:            authHandler,
		OAuthHandler:           oauthHandler,
		ProfileHandler:         profileHandler,
		AdminCategoryHandler:   adminCategoryHandler,
		ProductHandler:         productHandler,
		AdminProductHandler:    adminProductHandler,
		AdminOrderHandler:      adminOrderHandler,
		AdminOrderStatsHandler: adminOrderStatsHandler,
		AdminSuggestionHandler: adminSuggestionHandler,
		CartHandler:            cartHandler,
		OrderHandler:           orderHandler,
		RatingHandler:          ratingHandler,
		SuggestionHandler:      suggestionHandler,
		CorsMiddleware:         middleware.CORSConfig(),
		AuthMiddleware:         authMiddleware,
		UploadPath:             cfg.Upload.Path,
	}
	router := routes.SetupRouter(deps)

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	log.Printf("Server %s starting on %s", cfg.App.Name, addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
