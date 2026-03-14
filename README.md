# Hermit

A lightweight AI agent orchestrator built with Go, designed for efficient VPS environments.

## Overview

Hermit provides a complete agentic OS with Docker-based agent containers, Telegram integration, and a web dashboard. It uses a compiled Go binary for minimal resource usage.

## Architecture

```
hermit/
├── cmd/hermit/           # Main application entry point
├── internal/
│   ├── api/              # HTTP Handlers (Dashboard, Webhooks, Simulator)
│   ├── cloudflare/       # Cloudflare Tunnel integration
│   ├── db/               # SQLite database layer
│   ├── docker/           # Docker orchestration (exec, spawn)
│   ├── llm/              # Proxy client for OpenAI/Anthropic/Gemini/etc.
│   ├── parser/           # Regex-based XML contract parser
│   ├── telegram/         # Bot API and webhook management
│   └── workspace/        # File I/O, fsnotify (Portal watcher)
├── dashboard/              # React frontend (Vite + Bun)
├── context.md              # Base agent context template (immutable runtime skill)
├── docs/                # Technical docs and scenarios
└── hermit               # Compiled binary
```

## Key Features

### Forgiving XML Parser
A regex-based parsing engine that extracts AI intent without crashing on LLM formatting hallucinations. Supports:
- `<thought>` - Agent reasoning
- `<message>` - User-facing status updates
- `<terminal>` - Shell commands to execute
- `<action type="GIVE">filename</action>` - Deliver files
- `<action type="APP">appname</action>` - Publish web apps
- `<action type="SKILL">filename.md</action>` - Load skill files
- `<calendar>` - Scheduled events

### Docker Orchestration
Manages container lifecycles natively from the host:
- Container Spawning: `docker run -d --name hermit_agent_X alpine/debian sleep infinity`
- Command Execution: `docker exec -w /app/workspace/work <container> sh -c "<cmd>"`
- Each agent runs in an isolated Docker container with its own workspace

### Canonical Workspace Map
- `/app/workspace/work/` - Scratch work, scripts, generation
- `/app/workspace/in/` - User-provided files
- `/app/workspace/out/` - Deliverables (files to give to user)
- `/app/workspace/apps/` - Web apps (each app in subfolder with index.html)

### LLM Proxy & Autonomous Loop
1. Receive User Message → Append to History
2. Call LLM (OpenRouter, OpenAI, Anthropic, Google)
3. Parse Output via XML Parser
4. Send `<message>` to Telegram
5. If `<terminal>` exists: Execute, append output, loop
6. If `<action>` exists: Process side-effects
7. If `<calendar>` exists: Schedule future reminder

### Dashboard (6 Panels)

**1. Agents Dashboard**
- Grid of all agents with profile picture, name, role, status
- Create, start, stop, delete agents
- Tunnel URL for each agent

**2. Containers Panel**
- Real-time Docker container status
- CPU/Memory usage, disk usage
- File manager with in/work/out/apps folders
- Download files from containers

**3. Calendar Panel**
- Monthly calendar view
- Events from all agents with date/time, agent name, prompt
- Delete/cancel future events

**4. Tunnels Panel**
- Cloudflare Tunnel management
- Status, public hostname, UUID
- Create/delete tunnels per agent

**5. AllowLists Panel**
- Telegram user allowlisting (security boundary)
- CRUD for permitted users

**6. Settings Panel**
- Direct provider selection (OpenAI, Anthropic, Gemini)
- Provider API key input
- Domain mode toggle (custom domains vs cloudflared quick tunnels)
- Time zone configuration

### Agent Creation Flow (4-Step Modal)

1. **Agent Details**: Name, Role, Personality, LLM Model, Profile Picture
2. **Telegram Bot Verification**: Paste bot token, verify with 6-digit code
3. **Allowed Users**: Select from AllowList (security boundary)
4. **Cloudflare Tunnel**: Auto-provision dedicated public endpoint

### Telegram Integration
- Webhook server for incoming messages
- File upload handling to `/app/workspace/in/`
- Outbound Portal using fsnotify for automatic file delivery
- Bot verification flow with 6-digit codes
- Per-agent bot linking with allowlist security

### Public URL Integration
- Auto-provision dedicated cloudflared quick tunnels per agent
- Optional custom domain mode with Let's Encrypt
- Reverse proxy for apps at `/apps/{appname}`
- Tunnel/webhook health monitoring

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

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 3000 | Server port |
| DATABASE_PATH | ./data/hermit.db | SQLite database path |
| LLM_PROVIDER | openrouter | LLM provider (openrouter, gemini, openai, anthropic) |
| LLM_API_KEY | - | Fallback LLM API key |
| OPENROUTER_API_KEY | - | OpenRouter API key |
| OPENAI_API_KEY | - | OpenAI API key (optional fallback) |
| ANTHROPIC_API_KEY | - | Anthropic API key |
| GEMINI_API_KEY | - | Gemini API key |
| LLM_MODEL | openai/gpt-5.2 | LLM model (OpenRouter format by default) |
| TELEGRAM_BOT_TOKEN | - | Telegram bot token |

## Authentication

Default credentials:
- Username: `admin`
- Password: `hermit123`

**First login requires changing password.**

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/test-contract` | POST | Test contract parser |
| `/api/agents` | GET, POST | List/Create agents |
| `/api/agents/:id` | GET, PUT, DELETE, POST | Agent CRUD + start/stop |
| `/api/settings` | GET, POST | Get/Set settings |
| `/api/workspace/out` | GET | List output files |
| `/api/docker/exec` | POST | Execute docker command |
| `/api/docker/containers` | GET | List containers with live CPU/memory usage |
| `/api/metrics` | GET | Host + container real-time metrics |
| `/api/docker/files` | GET | List workspace files |
| `/api/docker/download` | GET | Download file |
| `/api/allowlist` | GET, POST | AllowList CRUD |
| `/api/calendar` | GET, POST | Calendar events |
| `/api/tunnels` | GET, POST | Tunnel management |
| `/api/telegram/verify` | POST | Verify Telegram bot |
| `/` | GET | Serve dashboard |
| `/webhook/` | POST | Telegram webhook |

## Testing

```bash
# Run all tests
go test ./... -v
```

## License

MIT
