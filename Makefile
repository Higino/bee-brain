.PHONY: all install test lint run clean build deps

# Default target
all: install

# Install dependencies
install: deps

# Run tests
test:
	go test ./...

# Run linter
lint:
	golangci-lint run

# Run the application
run:
	go run cmd/beebrain/main.go

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Build the application
build:
	mkdir -p bin
	go build -o bin/beebrain cmd/beebrain/main.go
	go build -o bin/check-message cmd/check-message/main.go

# Install dependencies
deps:
	go mod download
	go mod tidy

# Start ngrok tunnel for local development
tunnel:
	ngrok http 8080 