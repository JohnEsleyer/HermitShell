# Agent Flow Scenarios

This document describes the various scenarios and flows that an agent can experience in the Hermit Agent OS.

## Scenario 1: Normal Chat with Tooling

1. User sends a message from Telegram
2. Hermit forwards context to LLM
3. LLM returns XML contract
4. Parser extracts tags (`message`, `terminal`, `action`, `calendar`, `system`)
5. Hermit executes side effects and returns updates

---

## Scenario 2: User Requests a File

**Trigger:** User asks for a file (e.g., "Write me a song and put it in a txt file")

**Agent Expected Behavior:**
```xml
<message>I wrote the song and I'm sending it now.</message>
<give>song.txt</give>
```

**System Response:**
1. Checks `/app/workspace/out/` for `song.txt`
2. If exists, sends file to Telegram user
3. Returns success/failure status

**Key Points:**
- Texts without XML tags don't appear in Telegram
- `<message>` creates a chat bubble
- `<give>` is the delivery mechanism

---

## Scenario 3: User Requests a Reminder

**Trigger:** User says "Remind me at 4:00 AM to workout and walk 3KM"

**Agent Expected Behavior:**
```xml
<message>Got it — I'll remind you at 4:00 AM.</message>
<system>time</system>
<system>date</system>
<calendar>
  <datetime>2026-03-13T04:00:00</datetime>
  <prompt>It is 4:00 AM. Remind the user now to workout and walk 3KM.</prompt>
</calendar>
```

**System Response:**
1. Injects current time/date from `<system>` tags
2. Creates calendar event in `calendar.db`
3. Scheduler monitors database and triggers at specified time

---

## Scenario 4: Multiple Terminal Commands

If an LLM response contains multiple terminal tags, Hermit parses all of them and places them into a queue.

**Example:**
```xml
<terminal>npm install</terminal>
<terminal>npm test</terminal>
```

**Processing behavior:**
- Queue order is preserved
- First command starts immediately
- Next command starts only when previous command reaches terminal completion state
- Status transitions include `ONGOING` then `SUCCESS` or `FAILED`

**Checkpoint Algorithm:**
```xml
<terminal>cmd1</terminal>    <!-- Queue: [cmd1] -->
<terminal>cmd2</terminal>    <!-- Queue: [cmd1, cmd2] -->
<end>                       <!-- Queue cleared, commands executed -->
<terminal>cmd3</terminal>    <!-- Queue: [cmd3] -->
```

---

## Scenario 5: Takeover Mode

`/takeover` toggles takeover mode.

When enabled, Telegram user XML is treated as **system-control input**, useful for LLM quota exhaustion scenarios.

**Supported examples:**
```xml
<terminal>cd out</terminal>
<give>report.pdf</give>
```

**Notes:**
- Re-running `/takeover` exits takeover mode
- The system should always announce whether takeover mode is ON/OFF

---

## Scenario 6: System Information Retrieval

Agents can request runtime info without shelling out:

- `<system>time</system>` → localized runtime time
- `<system>memory</system>` → container/system memory snapshot
- `<system>date</system>` → current date

This enables safer and more deterministic status checks.

---

## Scenario 7: Loading Skills

**Trigger:** Agent determines a skill is relevant to the task

**Agent Expected Behavior:**
```xml
<skill>remotion.md</skill>
```

**System Response:**
1. Reads skill file content
2. Appends to context window
3. Continues with enhanced context

**Skill Selection:**
- System injects all skill titles/descriptions at start
- Agent chooses relevant skills
- Multiple skills can be loaded

---

## Scenario 8: Publishing an App

**Trigger:** Agent creates web app in workspace

**Agent Expected Behavior:**
```xml
<app name="my-todo-app">
<html>
<!DOCTYPE html>
<html>
<head>
    <title>Todo App</title>
</head>
<body>
    <h1>My Tasks</h1>
    <input id="task" placeholder="Add task">
    <button onclick="addTask()">Add</button>
    <ul id="list"></ul>
</body>
</html>
</html>
<style>
body { font-family: sans-serif; padding: 20px; }
input, button { padding: 10px; margin: 5px; }
</style>
<script>
function addTask() {
    const input = document.getElementById('task');
    const list = document.getElementById('list');
    const li = document.createElement('li');
    li.textContent = input.value;
    list.appendChild(li);
    input.value = '';
}
</script>
</app>
```

**System Response:**
1. Creates `/app/workspace/apps/my-todo-app/` folder
2. Creates `index.html` with embedded CSS/JS
3. Makes available via Traefik at `{dashboard-url}/apps/{agent-name}/my-todo-app`
4. Returns success with URL to agent

**Multi-Tenant Routing:**
- Traefik routes requests: `/apps/{container-id}/{app-name}` → container's internal port 80
- Apps appear in the Published Apps panel on the dashboard

---

## Polling & Tunnel Resilience

HermitShell uses long polling for Telegram (no webhook needed). Tunnels are optional for remote dashboard access.

On startup (`./hermit-server`):

1. Dashboard starts on localhost:3000
2. Optional: Dashboard public URL created (cloudflared tunnel)
3. Telegram polling starts for agents with tokens
4. Health monitor checks tunnel reachability (if enabled)

Telegram communication works regardless of tunnel status.

---

## Execution Flow Diagram

```
User Message
     |
     v
Parse XML Tags
     |
     +-- <message> ----> Queue for display
     |
     +-- <terminal> --> Queue for execution
     |
     +-- <give> -----> Send file to user
     |
     +-- <app> -----> Create web application
     |
     +-- <calendar> --> Insert to DB
     |
     +-- <system> ---> Inject response
     |
     +-- <skill> ---> Append to context
     |
     v
Find <end> tag
     |
     v
Execute Queue
     |
     v
Return Results
```
