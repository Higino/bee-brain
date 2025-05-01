# Color definitions
GREEN := $(shell tput setaf 2)
YELLOW := $(shell tput setaf 3)
BLUE := $(shell tput setaf 4)
MAGENTA := $(shell tput setaf 5)
CYAN := $(shell tput setaf 6)
WHITE := $(shell tput setaf 7)
RED := $(shell tput setaf 1)
RESET := $(shell tput sgr0)

.PHONY: build run test clean install lint ngrok help check deps fmt docker-build docker-run docker-clean verify-env verify-go app-and-tunnel configure-ngrok docker update-deps

# Load environment variables
include .env
export

# Help target
help:
	@echo "$(CYAN)Available targets:$(RESET)"
	@echo "$(GREEN)build$(RESET)          - Build the application"
	@echo "$(GREEN)run$(RESET)            - Run the application"
	@echo "$(GREEN)test$(RESET)           - Run tests"
	@echo "$(GREEN)clean$(RESET)          - Clean build artifacts"
	@echo "$(GREEN)install$(RESET)        - Install development dependencies"
	@echo "$(GREEN)lint$(RESET)           - Run linter"
	@echo "$(GREEN)check$(RESET)          - Run all checks (lint and tests)"
	@echo "$(GREEN)fmt$(RESET)            - Format Go code"
	@echo "$(GREEN)ngrok$(RESET)          - Start ngrok tunnel"
	@echo "$(GREEN)tunnel$(RESET)         - Run application and start ngrok tunnel"
	@echo "$(GREEN)app-and-tunnel$(RESET) - Run app and ngrok tunnel together"
	@echo "$(GREEN)configure-ngrok$(RESET) - Configure ngrok with auth token"
	@echo "$(GREEN)docker$(RESET)         - Build and run Docker containers"
	@echo "$(GREEN)docker-build$(RESET)   - Build Docker images"
	@echo "$(GREEN)docker-run$(RESET)     - Run application in Docker"
	@echo "$(GREEN)docker-clean$(RESET)   - Clean Docker resources"
	@echo "$(GREEN)verify-env$(RESET)     - Verify environment variables"
	@echo "$(GREEN)verify-go$(RESET)      - Verify Go version compatibility"
	@echo "$(GREEN)deps$(RESET)           - Download dependencies"
	@echo "$(GREEN)update-deps$(RESET)    - Update dependencies and go.sum"
	@echo "$(GREEN)help$(RESET)           - Show this help message"

# Update dependencies and go.sum
update-deps:
	@echo "$(BLUE)Updating dependencies and go.sum...$(RESET)"
	@go mod tidy
	@echo "$(GREEN)Dependencies updated successfully!$(RESET)"

# Verify Go version
verify-go:
	@echo "$(BLUE)Verifying Go version...$(RESET)"
	@if [ "$$(go version | cut -d' ' -f3 | cut -d'.' -f2)" -lt "21" ]; then \
		echo "$(RED)Error: Go version 1.21 or later is required$(RESET)"; \
		exit 1; \
	fi
	@echo "$(GREEN)Go version check passed!$(RESET)"

# Verify environment variables
verify-env:
	@echo "$(BLUE)Verifying environment variables...$(RESET)"
	@for var in SLACK_BOT_TOKEN SLACK_SIGNING_SECRET SLACK_VERIFICATION_TOKEN SLACK_BOT_USER OLLAMA_API_URL; do \
		if [ -z "$$(printenv $$var)" ]; then \
			echo "$(RED)Error: $$var is not set in .env file$(RESET)"; \
			exit 1; \
		fi \
	done
	@echo "$(GREEN)Environment variables check passed!$(RESET)"

verify: verify-go verify-env
	@echo "$(GREEN)All checks passed!$(RESET)"

# Download dependencies
deps: verify-go update-deps
	@echo "$(GREEN)Dependencies downloaded successfully!$(RESET)"

# Build the application
build: deps verify-env
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

# Format Go code
fmt:
	@echo "$(BLUE)Formatting Go code...$(RESET)"
	@go fmt ./...
	@echo "$(GREEN)Formatting complete!$(RESET)"

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
ngrok: configure-ngrok
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

docker-build: update-deps
	@echo "$(BLUE)Building Docker images...$(RESET)"
	@docker-compose build

docker-run: docker-build
	@echo "$(BLUE)Starting Docker services...$(RESET)"
	@docker-compose up -d --build

docker-clean:
	@echo "$(YELLOW)Cleaning Docker resources...$(RESET)"
	@docker-compose down --remove-orphans
	@docker-compose down -v --remove-orphans
	@echo "$(BLUE)Force removing any remaining containers...$(RESET)"
	@docker ps -a --filter name=go-brain --format "{{.ID}}" | xargs -r docker rm -f
	@echo "$(GREEN)Docker cleanup complete!$(RESET)"

docker-pull-llama3:
	@echo "$(BLUE)Pulling llama3 model...$(RESET)"
	@docker-compose --profile pull-models up pull-llama3
	@echo "$(BLUE)Removing pull containers...$(RESET)"
	@docker ps -a --filter name=pull --format "{{.Names}}" | xargs -r docker rm -f
	@echo "$(GREEN)Model pull complete!$(RESET)"