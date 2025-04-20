# Color definitions
GREEN := $(shell tput setaf 2)
YELLOW := $(shell tput setaf 3)
BLUE := $(shell tput setaf 4)
MAGENTA := $(shell tput setaf 5)
CYAN := $(shell tput setaf 6)
WHITE := $(shell tput setaf 7)
RED := $(shell tput setaf 1)
RESET := $(shell tput sgr0)

.PHONY: build run test clean install lint ngrok tunnel help check deps

# Load environment variables
include .env
export

# Help target
help:
	@echo "$(CYAN)Available targets:$(RESET)"
	@echo "$(GREEN)build$(RESET)      - Build the application"
	@echo "$(GREEN)run$(RESET)        - Run the application"
	@echo "$(GREEN)test$(RESET)       - Run tests"
	@echo "$(GREEN)clean$(RESET)      - Clean build artifacts"
	@echo "$(GREEN)install$(RESET)    - Install development dependencies"
	@echo "$(GREEN)lint$(RESET)       - Run linter"
	@echo "$(GREEN)check$(RESET)      - Run all checks (lint and tests)"
	@echo "$(GREEN)ngrok$(RESET)      - Start ngrok tunnel"
	@echo "$(GREEN)tunnel$(RESET)     - Run application and start ngrok tunnel"

# Download dependencies
deps:
	@echo "$(BLUE)Downloading dependencies...$(RESET)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)Dependencies downloaded successfully!$(RESET)"

# Build the application
build: deps
	@echo "$(BLUE)Building application...$(RESET)"
	@go build -o bin/beebrain cmd/beebrain/main.go
	@echo "$(GREEN)Build complete!$(RESET)"

# Run the application
run: build
	@echo "$(BLUE)Running application...$(RESET)"
	@./bin/beebrain

# Run tests
test: deps
	@echo "$(BLUE)Running tests...$(RESET)"
	@go test -v ./...

# Clean build artifacts
clean:
	@echo "$(YELLOW)Cleaning build artifacts...$(RESET)"
	@rm -rf bin/
	@echo "$(GREEN)Clean complete!$(RESET)"

# Install development dependencies
install: deps
	@echo "$(BLUE)Installing development dependencies...$(RESET)"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "$(GREEN)Installation complete!$(RESET)"

# Run linter
lint: deps
	@echo "$(BLUE)Running linter...$(RESET)"
	@golangci-lint run

# Check target - runs linting and tests
check: deps
	@echo "$(BLUE)Running checks...$(RESET)"
	@echo "$(CYAN)Running linter:$(RESET)"
	@golangci-lint run || (echo "$(RED)Linting failed!$(RESET)" && exit 1)
	@echo "$(GREEN)Linting passed!$(RESET)"
	@echo "\n$(CYAN)Running tests:$(RESET)"
	@go test -v ./... || (echo "$(RED)Tests failed!$(RESET)" && exit 1)
	@echo "$(GREEN)All checks passed!$(RESET)"

# Configure ngrok with auth token
configure-ngrok:
	@if [ -z "$(NGROK_AUTH_TOKEN)" ]; then \
		echo "$(RED)Error: NGROK_AUTH_TOKEN is not set in .env file$(RESET)"; \
		exit 1; \
	fi
	@echo "$(BLUE)Configuring ngrok with auth token...$(RESET)"
	@ngrok config add-authtoken $(NGROK_AUTH_TOKEN)
	@echo "$(GREEN)Ngrok configured successfully!$(RESET)"

# Start ngrok tunnel
tunnel: configure-ngrok
	@echo "$(BLUE)Checking for existing ngrok processes...$(RESET)"
	@if pgrep ngrok > /dev/null; then \
		echo "$(YELLOW)Found existing ngrok process, stopping it...$(RESET)"; \
		pkill ngrok; \
		sleep 2; \
	fi
	@echo "$(BLUE)Starting ngrok tunnel...$(RESET)"
	@ngrok http 8080

# Run application and start ngrok tunnel
app-and-tunnel: build configure-ngrok
	@echo "$(MAGENTA)Starting application and ngrok tunnel...$(RESET)"
	@echo "$(YELLOW)Press Ctrl+C to stop both processes$(RESET)"
	@echo ""
	@trap 'kill -9 $$(lsof -t -i:8080) 2>/dev/null; exit' INT; \
	ngrok http 8080 & \
	./bin/beebrain 