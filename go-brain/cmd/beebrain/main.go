package main

import (
	"log"
	"os"

	"beebrain/internal/llm"
	slackhandler "beebrain/internal/slack"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
	slackapi "github.com/slack-go/slack"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Get Slack token
	botToken := os.Getenv("SLACK_BOT_OAUTH_TOKEN")
	if botToken == "" {
		logger.Fatal("SLACK_BOT_OAUTH_TOKEN environment variable is not set")
	}

	// Initialize Slack client
	slackClient := slackapi.New(botToken)

	// Verify Slack authentication
	if _, err := slackClient.AuthTest(); err != nil {
		logger.Fatalf("Failed to verify Slack authentication: %v", err)
	}
	logger.Info("Successfully authenticated with Slack")

	// Initialize LLM client with bot name
	llmClient := llm.NewClient(logger, "BeeBrain")

	// Initialize Slack event handler
	eventHandler := slackhandler.NewEventHandler(
		slackClient,
		llmClient,
		logger,
		os.Getenv("SLACK_SIGNING_SECRET"),
	)

	// Initialize Echo server
	e := echo.New()

	// Add middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Add routes
	e.POST("/", eventHandler.HandleEvents)       // Handle Slack events at root
	e.POST("/events", eventHandler.HandleEvents) // Also handle events at /events

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	logger.Infof("Starting server on port %s", port)
	e.Logger.Fatal(e.Start(":" + port))
}
