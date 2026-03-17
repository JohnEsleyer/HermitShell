# Hermit

A lightweight AI agent orchestrator built with Go, designed for efficient VPS environments.

## Overview

Hermit provides a complete agentic OS with Docker-based agent containers, Telegram integration, and a web dashboard. Each AI agent runs in its own isolated Docker container with a dedicated workspace.

## Architecture

```
hermit/
├── cmd/
│   ├── hermit/           # Main application entry point
│   └── cli/              # Terminal UI interface
├── internal/
│   ├── api/              # HTTP Handlers (Dashboard, Webhooks, Telegram)
│   ├── cloudflare/       # Cloudflare Tunnel integration
│   ├── db/               # SQLite database layer
│   ├── docker/           # Docker orchestration (exec, spawn)
│   ├── llm/              # LLM client (OpenAI, Anthropic, Gemini, OpenRouter)
│   ├── parser/           # XML contract parser
│   ├── telegram/         # Bot API and webhook management
│   └── workspace/        # File I/O operations
├── dashboard/            # React frontend (Vite + Tailwind)
├── context.md            # Base agent context template
├── Dockerfile            # Agent container image definition
└── hermit               # Compiled binary
```

## Quick Start

### Using Makefile (Recommended)

```bash
# First-time setup (builds Docker image)
make setup

# Run the server
make run
```

### Manual Setup

```bash
# Build the Go server
go build -o hermit ./cmd/hermit/main.go

# Build the Docker image for agents
docker build -t hermit-agent:latest .

# Run
./hermit
```

Server starts on port 3000:
- Dashboard: http://localhost:3000/
- API: http://localhost:3000/api/

## CLI Interface

Hermit includes a terminal-based interface for managing agents:

```bash
# Build the CLI
go build -o hermit-cli ./cmd/cli/

# Run
./hermit-cli
```

### Environment Variables

Create a `.env` file (optional):

```bash
HERMIT_API_BASE=http://localhost:3000
HERMIT_CLI_USER=admin
HERMIT_CLI_PASS=your_password
```

### Controls

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate agents |
| `Enter` | View agent details |
| `r` | Refresh |
| `q` | Quit |

The CLI displays:
- Agent list with status
- Detailed view with context window, token count, word count
- Estimated cost (for Gemini models)

## Dashboard Panels

| Panel | Description |
|-------|-------------|
| **Agents** | Create, configure, and manage AI agents |
| **Containers** | Monitor Docker containers with CPU/Memory stats |
| **System Health** | Host metrics (CPU, Memory, Disk) |
| **Logs** | Global system logs (All/System/Agent/Docker/Network) |
| **Calendar** | Scheduled events for all agents |
| **Allowed Users** | Telegram user allowlist management |
| **Settings** | API keys, timezone, tunnel/domain mode |

## Key Features

### Agent Workspace
Each agent runs in an isolated Docker container with:
- `/app/workspace/work/` - Scratch work, scripts, generation
- `/app/workspace/in/` - User-provided input files
- `/app/workspace/out/` - Deliverables (files to give to user)
- `/app/workspace/apps/` - Published web apps
- `calendar.db` - Local scheduling database

### Telegram Integration
- Webhook-based message handling
- Per-agent bot configuration
- User allowlist security
- Commands: `/status`, `/help`, `/clear`, `/reset`, `/takeover`

### XML Contract Parser
Agents use XML tags for actions:
- `<message>...</message>` - Telegram message bubble
- `<terminal>...</terminal>` - Shell command execution
- `<give>filename</give>` - Deliver file from `/app/workspace/out/`
- `<app name="appname">...</app>` - Create web application
- `<calendar><datetime>...</datetime><prompt>...</prompt></calendar>` - Schedule reminder
- `<skill>skill-name</skill>` - Load skill context
- `<system>time|date|memory</system>` - Request system information

### Docker Orchestration
- Auto-create container on agent creation
- Auto-start container on telegram message
- Container reset capability
- Real-time metrics collection

### Usage Tracking
Each agent tracks:
- **LLM API Calls**: Number of requests sent to the LLM provider
- **Context Window**: Maximum token limit for the model
- **Word Count**: Total words in conversation history
- **Estimated Cost**: Cumulative cost estimation based on token usage

These metrics are displayed in the agent card on the dashboard and in the `/status` Telegram command.

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make setup` | First-time setup (builds Docker image) |
| `make build` | Build everything (UI + Server + Docker) |
| `make build-ui` | Build React dashboard |
| `make build-server` | Build Go binary |
| `make build-docker` | Build hermit-agent Docker image |
| `make dev` | Run in development mode |
| `make run` | Build and run production server |
| `make clean` | Remove build artifacts |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 3000 | Server port |
| DATABASE_PATH | ./data/hermit.db | SQLite database path |

API keys are configured through the dashboard Settings panel.

## Authentication

Default credentials:
- Username: `admin`
- Password: `hermit123`

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/agents` | GET, POST | List/Create agents |
| `/api/agents/:id` | GET, PUT, DELETE | Agent CRUD |
| `/api/agents/:id/action` | POST | Start/Stop/Reset container |
| `/api/containers` | GET | List containers with stats |
| `/api/containers/:id/action` | POST | Container actions |
| `/api/metrics` | GET | Host + container metrics |
| `/api/logs` | GET | System logs with category filter |
| `/api/settings` | GET, POST | Get/Set settings |
| `/api/allowlist` | GET, POST, DELETE | Allowed users CRUD |
| `/api/calendar` | GET, POST | Calendar events |
| `/api/telegram/verify` | POST | Verify Telegram bot |
| `/api/images/upload` | POST | Upload agent images |
| `/` | GET | Serve dashboard |
| `/webhook/:agentId` | POST | Telegram webhook |

## Testing

```bash
go test ./... -v
```

## License

MIT
