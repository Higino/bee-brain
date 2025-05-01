package main

import (
	"context"
	"log"
	"os"

	"beebrain/internal/llm"
	slackhandler "beebrain/internal/slack"
	"beebrain/internal/vectordb"

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

	// Initialize VectorDB client
	vectorDB, err := vectordb.NewClient(logger)
	if err != nil {
		logger.Fatalf("Failed to create VectorDB client: %v", err)
	}

	// Initialize VectorDB collection
	if err := vectorDB.InitializeCollection(context.Background()); err != nil {
		logger.Fatalf("Failed to initialize VectorDB collection: %v", err)
	}
	logger.Info("Successfully initialized VectorDB")

	// Create Slack event handler
	slackHandler := slackhandler.NewBeeBrainSlackEventHandler(
		slackClient,
		llmClient,
		vectorDB,
		logger,
		os.Getenv("SLACK_SIGNING_SECRET"),
		verificationToken,
		os.Getenv("LLM_MODE"),
	)

	// Create Echo instance
	e := echo.New()
	// Customize logging middleware to avoid log spamming
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `{"time":"${time_rfc3339}","method":"${method}","uri":"${uri}","status":${status},"latency":"${latency_human}"}` + "\n",
		Output: os.Stdout,
	}))

	// Add other middleware
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
