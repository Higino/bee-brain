# BeeBrain Slack Bot (Go Version)

A Slack bot built with Go that provides intelligent responses using LLM integration.

## Prerequisites

- Go 1.21 or later
- Slack App with appropriate permissions
- Environment variables (see `.env.example`)

## Setup

1. Clone the repository:
```bash
git clone <repository-url>
cd go-brain
```

2. Install dependencies:
```bash
make deps
```

3. Copy `.env.example` to `.env` and fill in your Slack credentials:
```bash
cp .env.example .env
```

4. Build the application:
```bash
make build
```

## Running the Application

To run the application:
```bash
make run
```

The bot will start on port 3000.

## Development

- Build: `make build`
- Run: `make run`
- Test: `make test`
- Clean: `make clean`

## Project Structure

```
go-brain/
├── cmd/
│   └── beebrain/
│       └── main.go
├── internal/
│   ├── slack/
│   │   └── handler.go
│   └── llm/
│       └── client.go
├── pkg/
│   └── utils/
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Environment Variables

- `SLACK_BOT_OAUTH_TOKEN`: Your Slack bot OAuth token
- `SLACK_SIGNING_SECRET`: Your Slack app signing secret
- `SLACK_BOT_USER`: Your Slack bot user ID

## License

MIT 