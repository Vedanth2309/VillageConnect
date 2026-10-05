package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"villageconnect/internal/config"
	"villageconnect/internal/handlers"
	"villageconnect/internal/repository"
	"villageconnect/internal/routes"
	"villageconnect/internal/search"
	"villageconnect/internal/services"

)

func main() {
	if err := godotenv.Load("../.env"); err != nil {
        log.Println("No .env file found, using system environment variables")
    }

    cfg := config.Load()
	mode := os.Getenv("GIN_MODE")
	if err := cfg.Validate(mode); err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := repository.NewStore(ctx, cfg)
	if err != nil {
		if mode == "release" {
			log.Fatalf("MongoDB is required in release mode: %v", err)
		}
		log.Printf("MongoDB unavailable; using fallback data: %v", err)
	}

	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	userService := services.NewUserService(store)
	searchService := search.NewService(store, cfg.ElasticsearchURL, cfg.ElasticsearchIndex)
	catalogService := services.NewCatalogService(store, searchService)
	orderService := services.NewOrderService(store)
	deliveryService := services.NewDeliveryService(store, orderService)
	reviewService := services.NewReviewService(store, searchService)
	notificationService := services.NewNotificationService(store)
	adminService := services.NewAdminService(store)
	api := handlers.NewAPI(userService, catalogService, orderService, deliveryService,
		reviewService, notificationService, adminService, searchService, cfg.JWTSecret)

	router := routes.New(api, func() string {
		if store.HealthStatus() == "fallback" {
			return "fallback"
		}
		if searchService.Enabled() {
			return "mongodb+elasticsearch"
		}
		return "mongodb"
	}, cfg.FrontendURL)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
