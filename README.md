# HermitShell

"Intelligence in a Persistent Shell" - A secure, multi-agent AI orchestration platform where each agent lives in its own Docker "cubicle" with persistent workspaces.

## Overview

HermitShell is a **Secure Agentic Operating System**. Each AI agent runs in a Docker container ("Cubicle") with its own Telegram bot identity, role, and budget. Containers run continuously until manually stopped, preserving state in persistent workspaces.

### Key Features

- **Multi-Agent Support**: Create multiple AI agents with different personalities, roles, and Docker images
- **Continuous Containers**: Containers run `sleep infinity` and execute commands via `docker exec` - instant response times
- **Persistent Workspaces**: Each agent+user pair gets a persistent workspace that survives container restarts
- **Auto Cloudflare Tunnel**: Automatic public URL generation on startup - no manual ngrok required
- **Automatic File Delivery**: Files dropped in `/workspace/out/` are automatically detected and sent via Telegram
- **Built-in RAG Memory**: Persistent factual memory per agent, configurable via the web UI
- **Agent Verification Handshake**: Verify Telegram tokens before creating agents via 6-digit code
- **Auto-Webhook Registration**: Webhooks automatically registered when agents are created
- **Sync Bots Button**: Re-register all webhooks with one click (useful when public URL changes)
- **Operator Auto-Allowlist**: Setting Operator ID automatically adds you to the allowlist
- **API Key Validation**: Dashboard blocks agent creation if API keys are missing
- **Per-Agent Budgeting**: Track and limit spending for each agent individually
- **Per-Agent LLM Selection**: Choose provider/model per agent (or inherit global defaults)
- **Persistent Chat Context**: Dashboard and Telegram keep context with explicit clear controls (`Clear` button and `/clear`)
- **Human-in-the-Loop (HITL)**: Internet-only approval gate (e.g., `curl`, `wget`, package installs, `git clone`)
- **Delegation HITL**: Agent collaboration requires operator approval
- **Async Telegram Processing**: Instant responses with background processing and status updates
- **Audit Logs**: Complete searchable history of all agent commands and responses
- **Web Terminal**: Attach to running agent containers via xterm.js
- **Web Dashboard**: Manage agents, users, and settings via a built-in GUI
- **Improved Agent Cards**: Less crowded cards with better action button fit and readability
- **File Browser**: Browse and download agent workspace files from dashboard
- **Manual Container Controls**: Start/Stop/Delete containers from the dashboard
- **Container Labels**: Track cubicles with `hermitshell.*` Docker labels
- **Agent Status Indicator**: Green (active) or amber/yellow (idle) status dots
- **Calendar Events**: Schedule future tasks with CRON-based event system
- **Apps Dashboard**: View and manage web apps created by agents
- **Site Preview Modal**: Preview sites in a modal with webview
- **Tunnel Sharing**: Share temporary tunnel links for Telegram access
- **Asset Procurement**: Request files from internet with operator Yes/No approval
- **App Thumbnails**: Scan apps and capture Playwright screenshots for card thumbnails

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     HermitShell                               │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Web Dashboard (Port 3000)              │   │
│  │  - Agent Management  - Budget Tracking  - Settings   │   │
│  │  - Audit Logs       - Web Terminal    - Test Agent  │   │
│  │  - Cubicles View    - Sync Bots       - File Browser│   │
│  │  - Calendar Events  - Apps Dashboard  - Memories    │   │
│  └─────────────────────────────────────────────────────┘   │
│                           │                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Node.js Shell (Orchestrator)             │   │
│  │  - libSQL DB  - Docker Management  - Webhooks        │   │
│  │  - HITL Controller  - Audit Logger  - API Key Check  │   │
│  │  - Auto-Webhook Registration  - Cloudflare Tunnel    │   │
│  │  - File Delivery   - Web Preview Proxy              │   │
│  │  - Calendar Scheduler - Asset Procurement           │   │
│  │  - Site Screenshots - Tunnel Manager                │   │
│  └─────────────────────────────────────────────────────┘   │
│                           │                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │           Cloudflare Quick Tunnel (cloudflared)      │   │
│  │  - Auto-generated public URL (trycloudflare.com)     │   │
│  │  - Webhook delivery  - Web preview access            │   │
│  │  - Site tunnel sharing - Temporary links             │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│  Agent A      │   │  Agent B      │   │  Agent C      │
│  "Sherlock"   │   │  "DevOps"     │   │  "Researcher" │
│ hermitshell/base │   │hermitshell/python│   │hermitshell/netsec│
│  [Status:Idle]│   │  [Status:Active]│  │  [Status:Idle] │
│  [HITL: ON]   │   │  [HITL: OFF]  │   │  [HITL: ON]   │
│  [Workspace]  │   │  [Workspace]  │   │  [Workspace]  │
│  [Port 8080]  │   │  [Port 8080]  │   │  [Port 8080]  │
│  [Running]    │   │  [Running]    │   │  [Stopped]    │
│  (continuous) │   │  (continuous) │   │  (can start)  │
└───────────────┘   └───────────────┘   └───────────────┘
        │                   │                   │
        └───────────────────┴───────────────────┘
                            │
                    Agent Meetings (DELEGATE)
                    Requires Operator Approval
```
┌─────────────────────────────────────────────────────────────┐
│                     HermitShell                               │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Web Dashboard (Port 3000)              │   │
│  │  - Agent Management  - Budget Tracking  - Settings   │   │
│  │  - Audit Logs       - Web Terminal    - Test Agent   │   │
│  │  - Cubicles View    - Sync Bots       - File Browser │   │
│  └─────────────────────────────────────────────────────┘   │
│                           │                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Node.js Shell (Orchestrator)             │   │
│  │  - libSQL DB  - Docker Management  - Webhooks        │   │
│  │  - HITL Controller  - Audit Logger  - API Key Check  │   │
│  │  - Auto-Webhook Registration  - Cloudflare Tunnel    │   │
│  │  - File Delivery   - Web Preview Proxy               │   │
│  └─────────────────────────────────────────────────────┘   │
│                           │                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │           Cloudflare Quick Tunnel (cloudflared)      │   │
│  │  - Auto-generated public URL (trycloudflare.com)     │   │
│  │  - Webhook delivery  - Web preview access            │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│  Agent A      │   │  Agent B      │   │  Agent C      │
│  "Sherlock"   │   │  "DevOps"     │   │  "Researcher" │
│ hermitshell/base │   │hermitshell/python│   │hermitshell/netsec│
│  [HITL: ON]   │   │  [HITL: OFF]  │   │  [HITL: ON]   │
│  [Workspace]  │   │  [Workspace]  │   │  [Workspace]  │
│  [Port 8080]  │   │  [Port 8080]  │   │  [Port 8080]  │
│  [Running]    │   │  [Running]    │   │  [Stopped]    │
│  (continuous) │   │  (continuous) │   │  (can start)  │
└───────────────┘   └───────────────┘   └───────────────┘
        │                   │                   │
        └───────────────────┴───────────────────┘
                            │
                    Agent Meetings (DELEGATE)
                    Requires Operator Approval
```

## Directory Structure

```
hermitshell/
├── docker-compose.yml    # Docker Compose setup (optional)
├── shell/                # Node.js orchestrator
│   ├── src/
│   │   ├── server.ts     # Fastify server + webhook handler + preview proxy
│   │   ├── db.ts         # libSQL database + vector memory + meetings
│   │   ├── docker.ts     # Docker orchestration + continuous containers
│   │   ├── telegram.ts   # Telegram handler + HITL + file delivery
│   │   ├── tunnel.ts     # Cloudflare tunnel management
│   │   └── auth.ts       # User validation + operator management
│   ├── dashboard/        # Built web GUI (auto-synced from dashboard/)
│   └── package.json
├── dashboard/            # Dashboard source (TypeScript/HTML)
│   ├── src/
│   │   └── public/       # Source files
│   ├── dist/             # Built files (synced to shell/dashboard/)
│   └── package.json
├── crab/                 # Python AI agent
│   ├── agent.py          # Unified Python agent daemon
│   └── Dockerfile
├── data/
│   ├── db/               # libSQL database
│   ├── workspaces/       # Persistent agent workspaces
│   │   └── {agent_id}_{user_id}/
│   └── cache/            # pip/npm cache for faster builds
└── config/               # Configuration files
```

## Quick Start

### 1. Run the Installer

```bash
chmod +x install.sh
./install.sh
```

### 2. Configure Environment

Edit `shell/.env`:
```bash
OPENROUTER_API_KEY=your_openrouter_key_here
OPENAI_API_KEY=your_openai_key_here  # Optional
WEBHOOK_SECRET=your_random_secret_here  # For webhook security
```

### 3. Build and Start the System

```bash
cd shell && npm run build
```

The build command will:
1. Compile TypeScript to JavaScript
2. Automatically sync dashboard files from `dashboard/src/public/` to `shell/dashboard/dist/`

Then start the server:
```bash
cd shell && npm start
```

The server will:
1. Start listening on port 3000
2. Automatically start Cloudflare tunnel
3. Generate a public URL (e.g., `https://random.trycloudflare.com`)
4. Sync all agent webhooks with the new URL

### Updating After Pulling from Main

If you pulled new changes from GitHub, rebuild to sync the dashboard:

```bash
git pull origin main
cd shell && npm run build
cd shell && npm start
```

### 4. Initial Setup (First Run)

On first run, the dashboard will show an **Initialization Screen**:

1. **Admin Username**: Your admin login
2. **Admin Password**: Your admin password
3. **Operator Telegram ID**: Your Telegram User ID (get from @userinfobot)

After logging in, go to **Settings** and configure:

4. **Public URL**: Auto-generated by Cloudflare tunnel (or set manually for custom domains)
5. **API Keys**: Enter your LLM provider API keys (OpenRouter, OpenAI, etc.)
6. Click **"Save All Settings"**

> **Note:** Setting your Operator ID automatically adds you to the Allowlist. The Cloudflare tunnel starts automatically on server launch.

### 5. Access the Dashboard

Open http://localhost:3000/dashboard/ in your browser

### 6. Create Your First Agent

The dashboard uses a **3-Step Verification Wizard**:

**Step 1: Configuration**
- Enter agent **Name** (e.g., "Sherlock")
- Enter **Role** description (e.g., "Security Researcher")
- Paste your **Telegram Token** (from @BotFather)
- Select **Docker Image** (hermitshell/base, hermitshell/python, or hermitshell/netsec)
- Select **LLM Provider/Model** for this agent (or keep `default` to inherit global settings)
- Toggle **HITL** if you want approval for dangerous commands

**Step 2: Verification Handshake**
- Click **"Send Verification Code"** - a 6-digit code will be sent to your Operator Telegram
- Wait for the code to arrive

**Step 3: Enter Code**
- Enter the 6-digit verification code
- Click **"Create Agent"** to complete setup
- Webhook is automatically registered!

**Step 4: Start the Bot**
- Go to your bot on Telegram and send `/start`
- Use `/clear` any time to explicitly reset the conversation context for that user+agent chat

> **Why verification?** This ensures your bot token is valid. The webhook is automatically registered so Telegram can send messages to your bot.

## Core Concepts

### Continuous Containers

Containers run continuously using `sleep infinity`:

- **Instant Response**: No container boot time - commands execute immediately via `docker exec`
- **State Preservation**: Background tasks, downloaded tools, and memory states persist between messages
- **No File Mount Issues**: History passed via Base64 ENV instead of file mounts
- **Manual Control**: Start/Stop/Delete containers from the Cubicles dashboard

| State | Condition | Action |
|-------|-----------|--------|
| **Running** | Container active | Commands execute instantly |
| **Stopped** | Manually stopped | Click "Start" to wake up |
| **Deleted** | Manually removed | Will spawn fresh on next message |

### Persistent Workspaces

Each agent+user pair gets a dedicated workspace:

- **Host Path**: `data/workspaces/{agent_id}_{user_id}`
- **Container Path**: `/app/workspace`
- **Persistence**: Survives container restarts, stops, and deletions
- **Shared Files**: Download once, reference across multiple conversations

### Automatic Cloudflare Tunnel

On startup, HermitShell automatically creates a public tunnel:

- **Zero Configuration**: No need for ngrok or manual port forwarding
- **Auto-generated URL**: Random `trycloudflare.com` URL (e.g., `https://random-name-1234.trycloudflare.com`)
- **Auto-sync**: Webhooks are automatically registered with the tunnel URL
- **Free**: Cloudflare Quick Tunnels require no account

The tunnel URL is saved to the database and used for all webhook registrations.

### Web App Previews

Current app-sharing flow is workspace based:

```
User accesses via:
  https://<tunnel>.trycloudflare.com/app/<agent_id>_<user_id>/<app_name>/
```

**How it works (current):**
1. Agent creates web app in `/app/workspace/www/<app_name>/index.html`
2. Shell publishes it via workspace app routing
3. User receives and opens the stable `/app/...` URL

> Legacy note: older runtime preview used `/preview/<agent_id>/8080/` proxying from a running server process.

### Automatic File Delivery (The Portal)

HermitShell uses a "Portal" architecture for file transfers. Instead of parsing LLM output, the orchestrator actively monitors a specific directory.

**How it works:**
1. Agent saves a file to the outgoing portal:
   ```bash
   echo "Report content" > /workspace/out/report.pdf
   ```
2. The Orchestrator (`shell`) detects the new file instantly via `chokidar`.
3. The file is automatically uploaded to the user's Telegram chat.

**Folder Organization:**
- `/workspace/work/`: 📂 **Sandbox**. Your primary working directory. Always cd here before starting work.
- `/workspace/in/`: 📥 **Input**. Files uploaded by the user via Telegram land here.
- `/workspace/out/`: 📤 **Output**. Files placed here are delivered to Telegram immediately.
- `/workspace/www/`: 🌐 **Web Apps**. Each subfolder is a separate web app with index.html.

**Agent execution flow standard:**
1. `ls -l /workspace/in`
2. `cd /workspace/work` and execute task
3. Move final files to `/workspace/out` and return `<action>GIVE:<file></action>`
4. For web apps, build under `/workspace/www/<unique_app_name>` and return `<action>APP:<unique_app_name></action>`

**Host Data Directory** (for database operations):
- `calendar.db`: Stores scheduled events. When time arrives, your prompt triggers automatically.
- `rag.db`: Persistent RAG memory for facts/knowledge.

### Agent Status Indicator

Each agent displays a status indicator:

- 🟢 **Green dot**: Agent is active (processing a request)
- 🟡 **Amber/Yellow dot**: Agent is idle (waiting for requests)

The status is automatically updated based on agent activity.

### Calendar Events (CRON-based Scheduling)

Agents can schedule future tasks using calendar events:

- **How it works**: Agent uses `CALENDAR_CREATE` panel action to schedule events
- **CRON-like behavior**: Events trigger at specified times automatically
- **Future prompts**: The prompt becomes the new user message when the event fires
- **Recurring tasks**: Schedule the NEXT event in your response to create loops
- **Display**: Upcoming events shown in Calendar Dashboard in the control panel
- **Management**: User can manually manage events through the dashboard

```
Example: User says "Tomorrow at 9AM, analyze the data"
Agent creates: CALENDAR_CREATE:Data Analysis|Analyze the CSV file|2026-02-28T09:00:00Z|
```

### Apps Dashboard (Web Apps)

The Apps dashboard shows web apps created by agents:

- **Location**: Agents create web apps in `/workspace/www/[app_name]/`
- **Each subfolder is a separate web app**
- **Required**: Each web app MUST have an `index.html` file
- **Vanilla Web**: Use plain HTML/CSS/JS (no frameworks like React/Vue)
- **Scan**: Click "Scan" to detect apps from each workspace `www/` folder
- **Preview**: Click "Open" to open a modal with the app
- **Share**: Generate temporary tunnel links for Telegram sharing (30 min expiry)
- **Thumbnail**: Apps can be screenshotted and displayed as card thumbnails
- **Endpoint**: Every app gets a stable endpoint like `/apps/<appname-random>` for tunnel sharing
- **Delete**: Remove one app folder or an entire workspace www set from the dashboard

### Asset Procurement System

Since agents are air-gapped, they can request assets from the internet:

1. Agent sends `ASSET_REQUEST` panel action with URL and description
2. User receives notification to approve/decline
3. Approved assets are downloaded to `/workspace/in/`

### Screenshot Capture

Apps can have Playwright screenshots captured:

- Click "Retake Shot" on an app card in the Apps dashboard
- Screenshots are stored in `data/screenshots/`
- Available for preview in the dashboard

### Deterministic Agent Tag Contract

Agent replies should use XML-style tags so the orchestrator can deterministically route execution:

```text
<thought>
Brief internal plan
</thought>
<message>
Done
</message>
<terminal>
python3 /app/workspace/work/script.py
</terminal>
<action>
GIVE:report.pdf
</action>
```

- `<message>`: sent to Telegram chat bubble (plain text, minimal)
- `<action>`: optional routing instruction. Use `GIVE:<filename>` for file delivery from `/app/workspace/out/`, or `APP:<app_name>` to publish a web app and share endpoint URL.
- `<terminal>`: optional shell command for container execution

**Compatibility**
- ✅ Primary: XML tags (`thought`, `message`, `terminal`, `action`)
- ⚠️ Fallback-only: JSON envelope and labeled `message:/terminal:/action:` formats
- ❌ Deprecated for new behavior: `panelActions`, text `ACTION: EXECUTE` formats

### Long-Term RAG Memory

Every agent has access to a dedicated RAG (Retrieval-Augmented Generation) memory store.

- **Storage**: Facts and rules are stored in `rag.db` (LibSQL database).
- **Management**: Use the **"Memories (RAG)"** tab in the Dashboard to manually add or prune memories.
- **Injection**: Relevant memories are automatically injected into the agent's system prompt before every request.
- **Persistence**: Memories survive container resets and workspace deletions.

### Dashboard File Browser

Browse and download agent workspace files from the dashboard:

- **Access**: Dashboard → Agents → Files button
- **Download**: Click any file to download
- **Directory tree**: Navigate nested directories

### Operator Security

The Operator is the primary human controller:

- **Configuration**: Set Operator Telegram ID in Settings
- **Auto-Allowlist**: Setting Operator ID automatically adds you to the Allowlist
- **Verification Codes**: Sent to Operator's Telegram for agent creation
- **HITL Approvals**: Internet-access approvals go to Operator (Yes/No reply flow)
- **Delegation Control**: Agent collaboration requires Operator approval

### API Key Validation

The system validates API keys at multiple levels:

1. **Dashboard Prevention**: Cannot create agents if API key for selected provider is missing
2. **Pre-flight Check**: Container won't spawn if API key is missing
3. **Error Interception**: 401 Unauthorized errors are caught and shown as friendly messages

### Auto-Webhook Registration

Webhooks are automatically registered when:

- An agent is created via verification
- You click **"Sync Bots"** button in the dashboard header

Use **Sync Bots** when:
- Your public URL changes (ngrok restart, Cloudflare tunnel changes)
- Bot stops responding to messages
- After server restarts with a new URL

## Human-in-the-Loop (HITL)

HITL is internet-only. Local workspace commands execute autonomously; internet egress commands pause for operator confirmation via Telegram:

1. Agent attempts internet command (`curl`, `wget`, `npm install`, `pip install`, `git clone`, `apt-get`, `apk add`)
2. Shell logs an internet HITL event and sends an approval prompt to the Operator
3. Operator replies **Yes** (allow) or **No** (block)
4. If allowed, command executes; if blocked, execution is aborted and returned as an error

### Delegation HITL

When an agent delegates to another agent:

```
ACTION: DELEGATE
AGENT_ROLE: Python Expert
TASK: Analyze the CSV file
```

The Operator receives:
- Delegation request with target role and task
- Operator approval prompt
- On approval: New cubicle spawned for sub-task

## Built-in Telegram Commands

| Command | Description |
|---------|-------------|
| `/start` | Start the bot and show welcome message |
| `/status` | Check cubicle status (running/stopped) |
| `/reset` | Delete current cubicle (fresh start next message) |
| `/budget` | Check remaining daily budget |
| `/debug` | Show detailed debug info |
| `/logs` | View recent container logs |
| `/workspace` | List files in persistent workspace |
| `/help` | Show all commands |

**Operator-only commands:**
| Command | Description |
|---------|-------------|
| `/containers` | List all running containers |
| `/agents` | List all registered agents |

## Agent Output Syntax

Agents use special syntax for enhanced features:

### File Delivery
```
GIVE: /app/workspace/report.pdf
GIVE: /app/workspace/analysis.csv
```
Files are automatically sent to the user via Telegram.

### Web App Preview
Agents should run web servers on port 8080:
```bash
python3 -m http.server 8080
streamlit run app.py --server.port 8080
flask run --port 8080
```
Access at: `https://<tunnel>/app/<agent_id>_<user_id>/<app_name>/`

> Legacy note: `/preview/<agent_id>/8080/` is older preview behavior and should not be used for new app-sharing flows.

### Delegation
```
ACTION: DELEGATE
TARGET_ROLE: Python Expert
TASK: Analyze the data file
```
Requires operator approval before spawning sub-agent.

### Legacy Controls (for migration context only)
Older versions used `panelActions` (calendar, asset requests, ClawMotion).

These are now **legacy/deprecated** and should not be used for new implementations.
For new work, rely on deterministic XML-tag contract fields plus explicit server-side APIs.

### Telegram Message Optimization
Due to Telegram's message limit, agents should:
- Keep responses concise
- Use bullet points
- Prefer file delivery over pasting content

## Container Labels

All cubicles are tagged with Docker labels for tracking:

```yaml
hermitshell.agent_id: "1"
hermitshell.user_id: "123456789"
hermitshell.last_active: "2026-02-20T12:00:00Z"
hermitshell.status: "active"
hermitshell.created_at: "2026-02-20T10:00:00Z"
```

## Agent Images

| Image | Tools | Use Case |
|-------|-------|----------|
| `hermitshell/base:latest` | curl, jq, sed, awk, bash | General tasks |
| `hermitshell/python:latest` | python3, pandas, numpy | Data analysis |
| `hermitshell/netsec:latest` | nmap, dig, openssl | Security research |

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | 3000 |
| `SESSION_SECRET` | Session cookie secret | `hermitshell-secret-change-in-production` |
| `WEBHOOK_SECRET` | Telegram webhook secret | `hermitshell-webhook-secret` |
| `OPENROUTER_API_KEY` | OpenRouter API key | Required |
| `OPENAI_API_KEY` | OpenAI API key (optional) | - |

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/auth/status` | GET | Check auth status |
| `/api/auth/setup` | POST | Create initial admin + operator |
| `/api/auth/login` | POST | Admin login |
| `/api/agents` | GET | List all agents |
| `/api/agents/request-verification` | POST | Send verification code |
| `/api/agents/confirm-verification` | POST | Verify and create agent |
| `/api/test-agent/:id` | POST | Test agent with message |
| `/api/containers/:id/start` | POST | Start a stopped container |
| `/api/containers/:id/stop` | POST | Stop a running container |
| `/api/containers/:id/remove` | POST | Delete a container |
| `/api/webhooks/sync` | POST | Re-register all agent webhooks |
| `/api/settings/batch` | POST | Save settings (trims values, auto-allowlist) |
| `/api/files/:agentId/:userId` | GET | List workspace files |
| `/api/files/:agentId/:userId/download/*` | GET | Download a file |
| `/preview/:agentId/:port/*` | GET | Legacy proxy preview endpoint (kept for compatibility) |
| `/webhook/:token` | POST | Telegram webhook (returns 202) |

### Database Schema

- **agents**: id, name, role, telegram_token, system_prompt, docker_image, is_active, require_approval, status, last_active_at, created_at
- **budgets**: agent_id, daily_limit_usd, current_spend_usd, last_reset_date
- **allowlist**: user_id, username, first_name, is_operator, added_at
- **settings**: key, value (includes operator_telegram_id, public_url, api keys, etc.)
- **admins**: id, username, password_hash, salt, created_at
- **audit_logs**: id, agent_id, container_id, command, output_snippet, response_text, action_type, approved_by, approved_at, status, created_at
- **calendar_events**: id, agent_id, title, prompt, start_time, end_time, target_user_id, color, symbol, status, created_at
- **asset_requests**: id, agent_id, user_id, description, url, file_type, status, requested_at, reviewed_by, reviewed_at
- **site_screenshots**: id, agent_id, user_id, site_name, screenshot_path, created_at
- **site_tunnels**: id, agent_id, user_id, site_name, tunnel_url, expires_at, is_active, created_at
- **agent_memory**: id, agent_id, content, embedding, created_at
- **meetings**: id, initiator_id, participant_id, topic, transcript, status, created_at
- **agent_runtime_logs**: id, agent_id, level, source, message, context, created_at

## Security

- **Air-Gapped Cubicles**: Containers have NO direct internet access. All LLM requests are proxied through the host orchestrator.
- **Centralized API Keys**: API keys never enter the container; they stay safely on the host system.
- **Admin Authentication**: Dashboard protected with session-based auth
- **Operator-First Bootstrap**: Primary admin required during setup
- **Agent Verification**: Telegram tokens verified before agent creation
- **Webhook Secret**: All webhooks validated with secret token
- **Human-in-the-Loop**: Internet-access commands require approval
- **Budget Guards**: Per-agent spending limits prevent runaway costs
- **Audit Trail**: Complete logging of all executed commands
- **File Path Validation**: File downloads restricted to workspace directory

## Troubleshooting

### Initial page shows a locked login page instead of setup
**Issue:** After cleaning and reinstalling the app, the initial page shows a locked login page instead of user credentials setup. How can I log in if I don't have credentials setup yet?

**Reason:** This issue occurs because the HermitShell server detects that an entry already exists in the `admins` table of your database. Even after a "clean reinstall," the persistent data stored in the `data/` directory often survives unless explicitly deleted.

**Solution:** To fix this and trigger the **Initialization Screen** (Setup), follow these steps:

#### 1. Wipe the existing database
The database file is stored in `data/db/`. You need to delete this file to force the system to return to "Setup Mode."

Run this command from the root of the `hermitshell` folder:
```bash
rm -rf data/db/*.db
```
*(Note: The database file is named `hermitshell.db`. Deleting everything in that folder ensures it is gone.)*

#### 2. Restart the Shell server
Once the database file is deleted, you must restart the Node.js process so it can re-initialize the schema and detect that there are zero admins.

```bash
cd shell
npm start
```

#### 3. Refresh the Dashboard
Open your browser to: `http://localhost:3000/dashboard/`

You should now see the **"INITIALIZE SYSTEM"** screen with the **"First Time Setup"** notice, allowing you to create your admin username, password, and Operator ID.

---

**Why did this happen?**
In your `shell/src/server.ts` file, the logic that decides which screen to show is:
```typescript
const adminCount = await getAdminCount();
if (adminCount === 0) {
    return { status: 'setup_required' }; // Shows the Registration page
}
```
If you didn't delete the `data/` folder during your "clean reinstall," the old `hermitshell.db` file was still there. Since that file contained your old admin account, the system skipped setup and went straight to the login (Locked) screen.

**Troubleshooting persistence (Docker)**
If you are running HermitShell via **Docker Compose**, the data is likely trapped in a Docker Volume. To truly wipe it, run:
```bash
docker-compose down -v
```
The `-v` flag deletes the volumes associated with the containers, ensuring the database is actually destroyed.


### Docker permission denied
```bash
sudo usermod -aG docker $USER
newgrp docker
```

### Build errors
```bash
sudo apt install build-essential python3 pkg-config libssl-dev
```

### Database issues
```bash
rm -rf data/db/hermitshell.db
```

### Find your Telegram User ID
Send `/start` to @userinfobot on Telegram

### Bot not responding to messages
1. Check server logs for tunnel URL (should show `✅ Tunnel active: https://...`)
2. If no tunnel, verify cloudflared is installed: `cloudflared --version`
3. Click **"Sync Bots"** button in the dashboard header
4. Verify the webhook was registered (check server logs for `[Tunnel] Synced X webhooks`)

### 401 Unauthorized error
1. Go to **Settings → API Keys**
2. Enter your API key for the selected provider
3. Click **"Save All Settings"**
4. Send `/reset` to your bot or delete the container from Cubicles tab
5. Try again

### Container not waking up
```bash
docker ps -a --filter "label=hermitshell.agent_id"
```
Or use the **Start** button in the Cubicles dashboard tab.

## Technology Stack

- **Orchestrator**: Node.js + Fastify + TypeScript
- **Database**: libSQL (SQLite-compatible)
- **Agent Runtime**: Python 3.10+
- **Container Runtime**: Docker with labels + exec
- **Frontend**: Vanilla JS + Tailwind CSS + xterm.js
- **LLM Providers**: OpenRouter / OpenAI / Anthropic / Google / Groq / Mistral / DeepSeek / xAI

## License

MIT

## Testing

HermitShell includes automated tests for API endpoints, Telegram webhooks, and Cloudflare tunnel connectivity.

### Running Tests

```bash
cd shell
npm test
```

### Test Coverage

- **API Tests**: Health checks, settings, webhook sync, agents API
- **Telegram Tests**: Webhook endpoints, secret validation, bot commands
- **Tunnel Tests**: Cloudflare tunnel URL validation, accessibility checks, Telegram webhook reachability

### Test Results

```
 Test Files  3 passed (3)
      Tests  15 passed (15)
```

### Test Environment

- Tests run against `http://localhost:3000` by default
- Override with `TEST_BASE_URL` environment variable
- Tests require the server to be running
- Tunnel tests validate external accessibility (port 530 = tunnel down)
