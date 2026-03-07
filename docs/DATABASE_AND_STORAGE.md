# Database & Storage

HermitShell uses **libSQL** (a SQLite-compatible database) for structured data and the host filesystem for agent workspaces.

## 🗄️ Database Schema (`hermitshell.db`)

The database is managed in `shell/src/db.ts`. Key tables include:

### 1. `agents`
Stores the configuration for each AI persona.
- `id`: Primary Key.
- `name`: Display name.
- `role`: System role (e.g., "DevOps Expert").
- `telegram_token`: Unique bot token.
- `docker_image`: The base image (default: `hermit/base`).
- `require_approval`: Legacy per-agent field; internet HITL is now enforced universally for network egress patterns.
- `llm_provider` / `llm_model`: Provider-specific overrides.
- `status`: Agent status - `idle` or `active`.
- `last_active_at`: Timestamp of last activity.

### 2. `budgets`
Tracks daily spending limits to prevent runaway LLM costs.
- `daily_limit_usd`: Max spend per 24h.
- `current_spend_usd`: Accumulated cost since last reset.
- `last_reset_date`: Used to auto-reset at midnight.

### 3. `audit_logs`
A permanent record of every command executed by every agent.
- `command`: The raw shell command.
- `output_snippet`: Truncated output (first 500 chars).
- `response_text`: Full agent response.
- `action_type`: Type of action (e.g., "command", "calendar", "asset_request").
- `status`: `Executed`, `Pending Network`, `Network Allowed`, `Network Blocked`, or `Error`.
- `approved_by`: Telegram ID of the operator/admin who allowed or blocked network access.

### 4. `calendar_events`
CRON-based scheduling for future agent tasks.
- `agent_id`: The agent this event belongs to.
- `title`: Event title.
- `prompt`: The prompt that will be sent when event triggers.
- `start_time`: When the event should fire (ISO 8601).
- `end_time`: Optional end time.
- `color`: Display color for the calendar UI.
- `symbol`: Symbol/emoji for the event.
- `status`: `scheduled`, `running`, `completed`, `failed`, or `cancelled`.

### 5. `asset_requests`
Tracks requests for files from the internet.
- `agent_id`: The agent making the request.
- `user_id`: The user who needs to approve.
- `description`: What the asset is for.
- `url`: Source URL (optional).
- `file_type`: Type of file (e.g., "csv", "image").
- `status`: `pending`, `approved`, or `declined`.

### 6. `site_screenshots`
Captured screenshots of web apps.
- `agent_id`: The agent that owns the site.
- `user_id`: The user workspace.
- `site_name`: Name of the web app.
- `screenshot_path`: Path to the screenshot file.

### 7. `site_tunnels`
Temporary tunnel links for sharing sites.
- `agent_id`: The agent that owns the site.
- `user_id`: The user workspace.
- `site_name`: Name of the web app.
- `tunnel_url`: The temporary tunnel URL.
- `expires_at`: When the link expires.
- `is_active`: Whether the tunnel is currently active.

### 8. `agent_memory` (RAG)
Used for long-term "context" beyond the immediate chat history.
- `content`: The text snippet.
- `embedding`: JSON-serialized vector (for future semantic search).

### 9. `allowlist`
Manages access control for the Telegram bots.
- `user_id`: Telegram User ID.
- `is_operator`: Boolean for administrative privileges.

## 📂 Filesystem Storage (`data/`)

Persistent data that doesn't fit in the DB is stored in the `data/` directory:

- `data/db/`: Main control panel DB files:
  - `hermitshell.db`: Main application database (agents, users, settings, etc.)
- `data/workspaces/{agentId}_{userId}/data/`: Per-container prebuilt libSQL files:
  - `calendar.db`: Scheduled actions/events for that specific agent-user workspace
  - `rag.db`: Long-term RAG memory for that specific workspace
  
- `data/workspaces/`: **Crucial Area.**
  - Folders are named `{agentId}_{userId}/`.
  - Content includes the `out/`, `in/`, `www/`, and `work/` portals:
    - `work/`: Agent's sandbox - use for all tasks
    - `in/`: Files uploaded by user via Telegram
    - `out/`: Files auto-delivered to user via Telegram
    - `www/`: Web apps (each subfolder = separate app with index.html)
- `data/screenshots/`: Captured site screenshots.
- `data/history/`: JSON files containing the rolling conversation history for each agent/user pair.
- `data/certs/`: (Optional) SSL certificates if not using the Cloudflare tunnel.

## 📅 Calendar Events (calendar.db)

The calendar system allows agents to schedule future tasks that trigger automatically.

Primary tables in each workspace `calendar.db`:
- `agent_calendar`: source of scheduled tasks (`task_name`, `instructions`, `scheduled_at`, recurrence fields, status)
- `task_history`: execution records (`calendar_id`, `executed_at`, `agent_response`, `success`)

HermitShell also keeps compatibility tables (`calendar_events`, `calendar_event_runs`) for existing dashboard APIs.

Behavior:
- Agent inserts/upserts rows in `agent_calendar` using SQL (from XML contract command flow).
- Scheduler claims due rows (`status='pending'` and `scheduled_at <= now`) and runs `instructions` as the prompt.
- Run output is normalized to JSON logs/history and persisted.
- For recurring tasks (`is_recurring=1` + `cron_expression`), scheduler computes and stores next run, then resets status to `pending`.
- Calendar dashboard reads scheduled tasks and current state.

## 🧠 RAG Memory (rag.db)

Persistent long-term memory for each agent:

- Facts, rules, and knowledge are stored here
- Managed via the **Memories (RAG)** tab in the Dashboard
- Relevant memories are automatically injected into the agent's system prompt
- Survives container restarts and workspace deletions

## 🧠 Memory Management

- **Short-term Memory**: The last ~10-20 messages are loaded from `data/history/` and passed to the LLM with every request.
- **Long-term Memory**: Managed via the `agent_memory` table / `rag.db`. The Orchestrator can inject relevant snippets into the system prompt based on the user's query.
