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

	// Set log level from environment variable
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info" // Default to info if not set
	}
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logger.Warnf("Invalid LOG_LEVEL '%s', defaulting to 'info'", logLevel)
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Get Slack tokens
	botToken := os.Getenv("SLACK_BOT_TOKEN")
	if botToken == "" {
		logger.Fatal("SLACK_BOT_TOKEN environment variable is not set")
	}

	verificationToken := os.Getenv("SLACK_VERIFICATION_TOKEN")
	if verificationToken == "" {
		logger.Fatal("SLACK_VERIFICATION_TOKEN environment variable is not set")
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

	// Create Slack event handler
	slackHandler := slackhandler.NewBeeBrainSlackEventHandler(
		slackClient,
		llmClient,
		logger,
		os.Getenv("SLACK_SIGNING_SECRET"),
		verificationToken,
	)

	// Initialize Echo server
	e := echo.New()

	// Add middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Add routes
	e.POST("/", slackHandler.HandleSlackEvents)       // Handle Slack events at root
	e.POST("/events", slackHandler.HandleSlackEvents) // Also handle events at /events

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	logger.Infof("Starting server on port %s", port)
	e.Logger.Fatal(e.Start(":" + port))
}
