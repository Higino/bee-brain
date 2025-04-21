# BeeBrain Slack Bot

A Go-based Slack bot that integrates with Ollama for intelligent conversation handling. This bot can process messages in threads and maintain context-aware conversations.

## Features

- 🤖 Intelligent conversation handling using Ollama
- 🧵 Thread-aware message processing
- 🔒 Secure request verification
- 📝 Clean and maintainable logging
- 🐳 Docker support for easy deployment

## Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- Slack workspace with admin access
- Ollama server (can be run via Docker)

## Environment Variables

Create a `.env` file in the root directory with the following variables:

```bash
SLACK_BOT_TOKEN=your-bot-token
SLACK_SIGNING_SECRET=your-signing-secret
SLACK_VERIFICATION_TOKEN=your-verification-token
OLLAMA_API_URL=http://localhost:11434/api/chat
```

## Local Development

### Using Go

1. Install dependencies:
```bash
go mod download
```

2. Build the application:
```bash
make build
```

3. Run the application:
```bash
make run
```

### Using Docker

1. Build and start the services:
```bash
docker-compose up --build
```

This will start both the BeeBrain bot and Ollama services.

## Docker Services

The application consists of two Docker services:

1. **Ollama Service**
   - Port: 11434
   - Persistent volume for model data
   - Accessible at `http://localhost:11434`

2. **BeeBrain Service**
   - Port: 8080
   - Environment variables from `.env`
   - Accessible at `http://localhost:8080`

## Project Structure

```
go-brain/
├── bin/                    # Compiled binaries
├── cmd/
│   └── beebrain/
│       └── main.go
├── internal/
│   ├── llm/
│   │   └── client.go
│   └── slack/
│       └── handler.go
├── Dockerfile             # Main Dockerfile
├── Dockerfile.beebrain    # BeeBrain-specific Dockerfile
├── Dockerfile.ollama      # Ollama-specific Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
├── Makefile
└── .env.example
```

## Make Commands

- `make build`: Build the application
- `make run`: Run the application
- `make test`: Run tests
- `make clean`: Clean build artifacts
- `make deps`: Download dependencies
- `make docker-build`: Build Docker images
- `make docker-run`: Run the application in Docker
- `make docker-clean`: Clean Docker resources
- `make lint`: Run linter checks
- `make fmt`: Format Go code

## Slack Integration

1. Create a new Slack app in your workspace
2. Add the following bot token scopes:
   - `app_mentions:read`
   - `channels:history`
   - `chat:write`
   - `groups:history`
   - `im:history`
   - `mpim:history`
3. Install the app to your workspace
4. Copy the bot token and signing secret to your `.env` file

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details. 