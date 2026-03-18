# HermitShell - AI Agent OS

<p align="center">
  <img src="docs/images/banner.svg" alt="HermitShell AI Agent OS" width="100%">
</p>

<p align="center">
  <a href="https://github.com/JohnEsleyer/HermitShell">
    <img src="https://img.shields.io/badge/HermitShell-AI%20Agent%20OS-000000?style=for-the-badge&logo=github" alt="HermitShell">
  </a>
  <a href="https://github.com/JohnEsleyer/HermitShell/releases">
    <img src="https://img.shields.io/github/v/release/JohnEsleyer/HermitShell?include_prereleases&style=for-the-badge" alt="Release">
  </a>
  <a href="https://github.com/JohnEsleyer/HermitShell/stargazers">
    <img src="https://img.shields.io/github/stars/JohnEsleyer/HermitShell?style=for-the-badge" alt="Stars">
  </a>
</p>

<p align="center">
  <b>A lightweight AI agent orchestrator built with Go, designed for efficient VPS environments.</b>
</p>

---

<p align="center">
  <img src="docs/images/agents-dashboard.png" alt="Dashboard - Agents Panel" width="45%">
  <img src="docs/images/system-health-dashboard.png" alt="Dashboard - Health Panel" width="45%">
</p>

---

## Overview

Hermit provides a complete agentic OS with Docker-based agent containers, Telegram integration, and a web dashboard. Each AI agent runs in its own isolated Docker container with a dedicated workspace.

### Key Features

- **🤖 AI Agents**: Autonomous agents with LLM capabilities (OpenAI, Anthropic, Google Gemini, OpenRouter)
- **📦 Containers**: Isolated Docker workspaces for each agent
- **💬 Telegram**: User interaction via Telegram Bot API
- **📅 Scheduler**: Event-driven agent automation with calendar reminders
- **🌐 Web Apps**: Agents can build and publish web applications
- **📊 Dashboard**: Real-time monitoring and control panel

---

## Quick Start

### Automated Installation (Linux)

```bash
# Download and run the installation script
./install.sh
```

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
go build -o hermit-server ./cmd/hermit/main.go

# Build the Docker image for agents
docker build -t hermit-agent:latest .

# Run
./hermit-server
```

Server starts on port 3000:
- Dashboard: http://localhost:3000/
- API: http://localhost:3000/api/

---

## Environment Variables

Create a `.env` file (optional):

```bash
# Server Configuration
PORT=3000
DATABASE_PATH=./data/hermit.db

# API Keys (configure via dashboard Settings panel)
OPENROUTER_API_KEY=sk-or-...
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=AIza...
```

### CLI Credentials

```bash
HERMIT_API_BASE=http://localhost:3000
HERMIT_CLI_USER=admin
HERMIT_CLI_PASS=hermit123
```

---

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
└── hermit-server        # Compiled binary
```

---

## Dashboard Panels

| Panel | Description |
|-------|-------------|
| **Agents** | Create, configure, and manage AI agents |
| **Containers** | Monitor Docker containers with CPU/Memory stats |
| **System Health** | Host metrics (CPU, Memory, Disk) |
| **Published Apps** | Web apps created by agents |
| **Settings** | API keys, timezone, tunnel/domain mode |
| **Docs** | Documentation and guides |

---

## Agent Workspace

Each agent runs in an isolated Docker container with:
- `/app/workspace/work/` - Scratch work, scripts, generation
- `/app/workspace/in/` - User-provided input files
- `/app/workspace/out/` - Deliverables (files to give to user)
- `/app/workspace/apps/` - Published web apps
- `calendar.db` - Local scheduling database

---

## Telegram Integration

- Webhook-based message handling
- Per-agent bot configuration
- User allowlist security
- Commands: `/status`, `/help`, `/clear`, `/reset`, `/takeover`

### Example /status Response

```
🤖 *Agent Status: Rain*

• Model: `gemini-3.1-flash-lite-preview`
• Provider: `gemini`
• Context Window: `1048576` tokens
• LLM API Calls: `42`
• Container: `agent-rain` (Running ✅)
• Webhook: Active ✅
```

---

## XML Contract Parser

Agents use XML tags for actions:

```xml
<!-- Send message to user -->
<message>Hello! I've completed your task.</message>

<!-- Execute terminal command -->
<terminal>ls -la /app/workspace/work</terminal>

<!-- Send file to user -->
<give>report.pdf</give>

<!-- Create web application -->
<app name="myapp">
<html>
<!DOCTYPE html>
<html>
<head><title>My App</title></head>
<body><h1>Hello World</h1></body>
</html>
</html>
<style>h1 { color: #333; }</style>
<script>alert('Hello!');</script>
</app>

<!-- Schedule multiple reminders -->
<calendar>
<datetime>2026-03-17T13:43:25</datetime>
<prompt>Japanese Lesson 1: 'Komorebi' - Sunlight filtering through trees</prompt>
</calendar>
<calendar>
<datetime>2026-03-17T13:45:25</datetime>
<prompt>Japanese Lesson 2: 'Mono no aware' - The pathos of beautiful things</prompt>
</calendar>

<!-- List all calendar events -->
<calendar action="list"/>

<!-- Delete a calendar event -->
<calendar action="delete" id="123"/>

<!-- Update a calendar event -->
<calendar action="update" id="456"><prompt>Updated reminder prompt</prompt></calendar>

<!-- Load skill context -->
<skill>python-coding</skill>

<!-- Request system information -->
<system>time</system>
<system>memory</system>
```

---

## Docker Orchestration

- Auto-create container on agent creation
- Auto-start container on telegram message
- Container reset capability
- Real-time metrics collection

---

## Usage Tracking

Each agent tracks:
- **LLM API Calls**: Number of requests sent to the LLM provider
- **Context Window**: Maximum token limit for the model
- **Word Count**: Total words in conversation history
- **Estimated Cost**: Cumulative cost estimation based on token usage

These metrics are displayed in the agent card on the dashboard and in the `/status` Telegram command.

---

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

---

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/agents` | GET, POST | List/Create agents |
| `/api/agents/:id` | GET, PUT, DELETE | Agent CRUD |
| `/api/agents/:id/action` | POST | Start/Stop/Reset container |
| `/api/agents/:id/stats` | GET | Agent statistics |
| `/api/containers` | GET | List containers with stats |
| `/api/metrics` | GET | Host + container metrics |
| `/api/settings` | GET, POST | Get/Set settings |
| `/api/telegram/verify` | POST | Verify Telegram bot |
| `/api/apps/:id/:name` | GET | Serve agent web apps |
| `/` | GET | Serve dashboard |

---

## Testing

```bash
go test ./... -v
```

---

## License

MIT
