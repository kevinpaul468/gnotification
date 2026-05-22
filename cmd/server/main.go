package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/swecha/notifications/internal/handlers"
	"github.com/swecha/notifications/pkg/database"
	"github.com/swecha/notifications/pkg/queue"
)

func main() {
	// Load .env file if exists
	godotenv.Load()

	// Get config from environment
	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		log.Fatal("DATABASE_URL env var not set")
	}

	rabbitmqDSN := os.Getenv("RABBITMQ_URL")
	if rabbitmqDSN == "" {
		rabbitmqDSN = "amqp://guest:guest@localhost:5672/"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize database
	db, err := database.NewDB(dbDSN)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Initialize queue
	q, err := queue.NewQueue(rabbitmqDSN)
	if err != nil {
		log.Fatalf("failed to connect to queue: %v", err)
	}
	defer q.Close()

	// Setup queues
	if err := q.DeclareQueues(nil); err != nil {
		log.Fatalf("failed to declare queues: %v", err)
	}

	// Create handlers
	notifHandler := handlers.NewNotificationHandler(db, q)
	adminHandler := handlers.NewAdminHandler(db, q)

	// Setup Echo
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/health", notifHandler.HealthHandler)
	e.POST("/notifications/send", notifHandler.SendNotification)
	e.GET("/notifications/:id", notifHandler.GetNotificationStatus)

	// Root route - API Key Request Form
	e.GET("/", adminHandler.GetRequestPage)
	e.POST("/api-key-request", adminHandler.RequestAPIKey)

	// Admin Routes
	e.GET("/admin", adminHandler.AdminDashboard)
	e.GET("/admin/stats", adminHandler.GetDashboardStats)
	e.GET("/admin/notifications", adminHandler.GetNotifications)
	e.POST("/admin/api-keys", adminHandler.CreateAPIKey)
	e.GET("/admin/api-keys", adminHandler.GetAPIKeys)
	e.DELETE("/admin/api-keys", adminHandler.RevokeAPIKey)
	e.POST("/admin/provider-configs", adminHandler.SaveProviderConfig)
	e.GET("/admin/provider-configs", adminHandler.GetProviderConfigs)
	e.DELETE("/admin/provider-configs", adminHandler.DeleteProviderConfig)
	e.GET("/admin/providers/available", adminHandler.GetAvailableProviders)

	// API Key Request Routes (admin endpoints)
	e.GET("/admin/api-key-requests", adminHandler.GetAPIKeyRequests)
	e.POST("/admin/api-key-requests/approve", adminHandler.ApproveAPIKeyRequest)

	// Start server
	log.Printf("Server starting on port %s", port)
	if err := e.Start(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
