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

	// Setup Echo
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/health", notifHandler.HealthHandler)
	e.POST("/notifications/send", notifHandler.SendNotification)
	e.GET("/notifications/:id", notifHandler.GetNotificationStatus)

	// Start server
	log.Printf("Server starting on port %s", port)
	if err := e.Start(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
