# The Agent Runtime

The **HermitShell Agent** is the core logic running inside the Docker Cubicle. It's a TypeScript/Node.js script (`agent.js`) that acts as the bridge between the LLM and the local Linux environment.

## 🤖 Core Functionality

The agent runtime is the brain of the container. It's responsible for:
- **Prompt Engineering**: Building the system prompt and context for the LLM.
- **LLM Communication**: Sending requests to the Orchestrator's internal proxy.
- **Action Extraction**: Parsing the LLM's response to identify and execute actions.
- **Safety Checks**: Validating commands to prevent dangerous or unintended operations.
- **Execution**: Running bash commands natively and capturing their output.
- **State Management**: Handling conversation history.
- **Status Tracking**: Reporting idle/active status to the orchestrator.

## ⚙️ How it Works

The agent follows a standard Loop:
1.  **Receive Message**: Accepts a user message and conversation history.
2.  **Call LLM**: Sends the prompt and history to the Orchestrator's internal LLM proxy (`/api/internal/llm`).
3.  **Process Actions**:
    - **No Action**: Returns the LLM's text response to the user.
    - **EXECUTE Action**:
        - Extracts the bash command.
        - Checks whether the command requests internet egress (`curl`, `wget`, package install, clone, etc.).
        - If internet access is requested, enters **HITL (Human-in-the-Loop)** and waits for operator Yes/No approval.
        - Executes the command natively in the shell.
        - Captures the command's stdout and stderr.
        - Appends the output to the conversation history.
4.  **Repeat**: Sends the command output back to the LLM and repeats until the task is complete or a maximum iteration limit is reached.

## 🛠️ Key Components in `agent.js`

- `buildSystemPrompt()`: Dynamically creates the system instruction based on the agent's name, role, and personality. Includes instructions for:
  - Workspace directory structure (work/, in/, out/, www/)
  - Calendar events for scheduling future tasks (stored in calendar.db)
  - Asset procurement for requesting files from the internet
  - Telegram message optimization (concise outputs)
  - Web app creation (vanilla HTML/CSS/JS, index.html required)
  - Video creation capabilities
- `callLlm()`: Uses Node.js `fetch` to securely call the Orchestrator proxy. Since the container is air-gapped, this is the **only** way the agent can communicate with the outside world.
- `extractCommand()`: Prioritizes tagged `<terminal>` command; JSON and legacy `ACTION: EXECUTE` are fallback compatibility paths.
- `extractPanelActions()`: Legacy parser for deprecated control panel actions.
- `isInternetRequest()`: A list of network egress commands that trigger Human-in-the-Loop checks.
- `waitForApproval()`: Monitors lock files (`/tmp/hermit_approval.lock`, `/tmp/hermit_deny.lock`) that the Orchestrator writes after operator Yes/No Telegram replies.

## 📂 File Delivery via the Agent

The agent is instructed to use the `/app/workspace/out/` directory for file delivery:
- Simply save any file (PDF, CSV, image, video) to `/app/workspace/out/`
- The Orchestrator's **File Watcher** detects new files and delivers to Telegram automatically

Workspace structure:
- `/app/workspace/work/`: Primary working directory (always cd here first)
- `/app/workspace/in/`: Files uploaded by user via Telegram
- `/app/workspace/out/`: Files auto-delivered to user via Telegram
- `/app/workspace/www/`: Web apps (each subfolder = separate app with index.html)

## 📝 Contract Actions

Agents are expected to return deterministic tagged output:

```text
<thought>Plan</thought>
<message>Task completed</message>
<terminal></terminal>
<action>GIVE:report.pdf</action>
```

> Legacy compatibility: old ad-hoc action channels may appear in historical logs/docs but should not be used for new implementations.

### Available Actions:
- **File Delivery**: `GIVE:<filename>`
- **Web App Publish**: `APP:<app_name>`

Calendar scheduling should be performed via SQL writes to `/app/workspace/data/calendar.db` (`agent_calendar`), not legacy ad-hoc action strings.

## ⚠️ Legacy Compatibility

- Legacy ad-hoc action channels are removed from new execution paths.
- Legacy `ACTION: EXECUTE` and JSON parsing exist only for backward compatibility; tagged `<terminal>` + `<action>` is the canonical contract.

## 🌐 Web App Creation

Agents create web apps in `/app/workspace/www/[app_name]/`:
- **Each subfolder is a separate web app**
- **Required**: Each web app MUST have an `index.html` file
- Use **vanilla** HTML, CSS, and JavaScript only (no frameworks like React/Vue)
- Images go in `assets/` subfolder
- Return `<action>APP:<app_name></action>` after creating `index.html` so the panel publishes and shares URL

## 📝 Logging & Auditing

All command executions and agent thoughts are streamed to the `.hermit.log` file in the agent's workspace. This file is used by the Orchestrator to provide real-time logs in the dashboard and to maintain a permanent audit trail of every action the agent took.
