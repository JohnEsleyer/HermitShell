# Hermit Technical Architecture

## System Overview

Hermit is an AI Agent Orchestration System that enables autonomous agents to run in isolated Docker containers, interact via Telegram, and expose services through public URLs.

## Core Components

### 1. Dashboard (React + Go Fiber)
- **Frontend:** React 18 with TailwindCSS
- **Backend:** Go Fiber REST API
- **Purpose:** User interface for managing agents, viewing metrics, configuring settings

### 2. Agent Runtime (Docker)
- Each agent runs in an isolated Docker container
- Workspace structure:
  - `/app/workspace/work/` - Scratchpad for agent operations
  - `/app/workspace/in/` - Input files from users
  - `/app/workspace/out/` - Deliverables for users
  - `/app/workspace/apps/` - Web apps published by agents

### 3. LLM Integration
Supports multiple providers:
- **OpenRouter** (free models recommended)
- **OpenAI** (GPT-4, GPT-4o)
- **Anthropic** (Claude)
- **Google Gemini**

### 4. Telegram Bot Integration
- Webhook-based communication
- Commands: `/start`, `/help`, `/clear`, `/tokens`, `/reset`, `/takeover`, `/give_system_prompt`, `/give_context`

## Public URL Strategy

Hermit treats public URLs as a core runtime dependency:

- Telegram webhooks require a publicly reachable endpoint
- Agent apps in `/workspace/apps/<app-name>` are exposed through reverse proxy paths like `/apps/<app-name>`

### Tunnel Mode (Default)
- Uses `cloudflared tunnel --url ...` quick tunnels
- No Cloudflare account token required
- Dashboard gets its own tunnel
- Agents can share dashboard tunnel or have dedicated tunnels

### Domain Mode (Optional)
- Operators provide custom domains
- HTTPS via Let's Encrypt (CertMagic)
- Single entry point for all services

## Tunnel Health Monitoring

Tunnel health is assessed through:

1. Reachability checks to tunnel URLs
2. Telegram webhook diagnostics (last error reported by Telegram)
3. Status displayed in Health panel

## Metrics

The metrics panel reads real host and container data:

- **Host Metrics:** CPU, memory, disk from `/proc` (via gopsutil)
- **Container Metrics:** CPU, memory from Docker stats
- **Network:** Tunnel/domain connectivity status
- Auto-refresh every 2 seconds for near real-time visualization

## Database Schema

SQLite database with tables:
- `agents` - Agent configurations
- `skills` - Agent skill definitions
- `calendar` - Scheduled events
- `allowlist` - Telegram users with access
- `tunnels` - Tunnel configurations
- `users` - Dashboard users
- `settings` - System configuration
- `history` - Conversation history
- `audit_logs` - Action logging

## API Endpoints

### Authentication
- `POST /api/auth/login` - Login
- `POST /api/auth/logout` - Logout
- `GET /api/auth/check` - Check auth status

### Agents
- `GET /api/agents` - List agents
- `POST /api/agents` - Create agent
- `GET /api/agents/:id` - Get agent
- `PUT /api/agents/:id` - Update agent
- `DELETE /api/agents/:id` - Delete agent

### Skills
- `GET /api/skills` - List skills
- `POST /api/skills` - Create skill
- `PUT /api/skills/:id` - Update skill
- `DELETE /api/skills/:id` - Delete skill
- `GET /api/skills/context` - Get context.md
- `POST /api/skills/context/reset` - Reset context.md

### Calendar
- `GET /api/calendar` - List events
- `POST /api/calendar` - Create event
- `PUT /api/calendar/:id` - Update event
- `DELETE /api/calendar/:id` - Delete event

### Allowlist
- `GET /api/allowlist` - List entries
- `POST /api/allowlist` - Add entry
- `DELETE /api/allowlist/:id` - Remove entry

### Metrics & Containers
- `GET /api/metrics` - System metrics
- `GET /api/containers` - Container stats

### Settings
- `GET /api/settings` - Get settings
- `POST /api/settings` - Update settings

### Telegram
- `POST /api/telegram/send-code` - Send verification code
- `POST /api/telegram/verify` - Verify bot
- `POST /api/webhook` - Telegram webhook
- `POST /api/webhook/:agentId` - Agent-specific webhook

## Security

- **Allowlist:** Only Telegram users in allowlist can interact with agents
- **Takeover Mode:** Direct system control when LLM quota exhausted
- **Container Isolation:** Each agent runs in its own Docker container
- **Session Auth:** Cookie-based session for dashboard

## Data Persistence

- **SQLite:** Agent configs, calendar events, allowlist, settings
- **Docker Volumes:** Agent workspaces
- **File System:** Skills (markdown files), context.md
