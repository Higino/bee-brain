# BeeBrain Slack Bot

A Go-based Slack bot that integrates with Ollama for intelligent conversation handling and Qdrant for vector storage. This bot can process messages in threads and maintain context-aware conversations.

## Features

- 🤖 Intelligent conversation handling using Ollama
- 🧵 Thread-aware message processing
- 🔒 Secure request verification
- 📝 Clean and maintainable logging
- 🐳 Docker support for easy deployment
- 🔄 Alternative `/generate` command for direct text generation
- 📊 Vector storage for message history using Qdrant

## Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- Slack workspace with admin access
- Ollama server (can be run via Docker)
- Qdrant server (can be run via Docker)
- ngrok (for local development with Slack)

## Environment Variables

Create a `.env` file in the root directory with the following variables:

```bash
SLACK_BOT_TOKEN=your-bot-token
SLACK_SIGNING_SECRET=your-signing-secret
SLACK_VERIFICATION_TOKEN=your-verification-token
SLACK_BOT_USER=your-bot-user-id
QDRANT_HOST=qdrant
QDRANT_PORT=6334
```

## Local Development

### Using Go

1. Install dependencies:
```bash
make deps
```

2. Build the application:
```bash
make build
```

3. Run the application:
```bash
make run
```

4. For local development with Slack, start the ngrok tunnel:
```bash
make tunnel
```

### Using Docker

1. Build and start the services:
```bash
docker-compose up --build
```

This will start the BeeBrain bot, Ollama, and Qdrant services.

## Docker Services

The application consists of three Docker services:

1. **Ollama Service**
   - Port: 11434
   - Persistent volume for model data (`ollama_data`)
   - Accessible at `http://localhost:11434`

2. **Qdrant Service**
   - Ports: 6333 (HTTP), 6334 (gRPC)
   - Persistent volume for vector data (`qdrant_data`)
   - Accessible at `http://localhost:6333`

3. **BeeBrain Service**
   - Port: 8080
   - Environment variables from `.env`
   - Accessible at `http://localhost:8080`

## Project Structure

```
go-brain/
├── bin/                    # Compiled binaries
├── cmd/
│   ├── beebrain/
│   │   └── main.go
│   └── check-message/
│       └── main.go
├── internal/
│   ├── llm/
│   │   └── client.go
│   ├── slack/
│   │   └── handler.go
│   └── vectordb/
│       └── client.go
├── Dockerfile             # Main Dockerfile
├── Dockerfile.beebrain    # BeeBrain-specific Dockerfile
├── Dockerfile.check-message # Check-message tool Dockerfile
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
- `make install`: Install development dependencies
- `make lint`: Run linter checks
- `make check`: Run all checks (lint and tests)
- `make ngrok`: Configure ngrok with auth token
- `make tunnel`: Start ngrok tunnel
- `make app-and-tunnel`: Run application and start ngrok tunnel

## Slack Integration

1. Create a new Slack app in your workspace
2. Add the following bot token scopes:
   - `app_mentions:read`
   - `channels:history`
   - `chat:write`
   - `groups:history`
   - `im:history`
   - `mpim:history`
   - `commands` (for slash commands)
3. Create a new slash command:
   - Command: `/generate`
   - Request URL: `https://your-domain.com/slack/events`
   - Short Description: Generate text using the LLM
   - Usage Hint: `[prompt]`
4. Install the app to your workspace
5. Copy the bot token, signing secret, and bot user ID to your `.env` file

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details. 