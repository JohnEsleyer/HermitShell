# Telegram Commands

Hermit Agent OS supports the following Telegram commands for managing agents, contexts, and controlling the system.

## Basic Commands

### /start
Welcome message. Shows a brief introduction to Hermit and how to get help.

### /help
Displays all available commands with brief descriptions.

## Context Management

### /clear
Clears the current context window. This removes all conversation history and starts fresh.

**Usage:**
```
/clear
```

**Response:**
```
Context window cleared!
```

**When to use:**
- When the context becomes too large and you want to start fresh
- When the agent seems confused or stuck
- When you want to reset the conversation flow

### /tokens
Shows the approximate token count of the current context window.

**Usage:**
```
/tokens
```

**Response:**
```
Current context size: ~1234 tokens
```

**When to use:**
- To monitor context size and prevent exceeding limits
- Before using /clear to understand how much context you have

## Container Control

### /reset
Resets the container by destroying the existing one and replacing it with a fresh container. This clears all workspace files and starts with a clean state.

**Usage:**
```
/reset
```

**Response:**
```
Container reset initiated...
Container has been reset with fresh state.
```

**When to use:**
- When the container is in a broken state
- When you need a completely clean workspace
- When you want to start over with files

## Takeover Mode

### /takeover
Toggles "takeover mode" on or off. When enabled, you can send XML commands directly to control the system instead of the AI agent. This is useful when LLM API usage is exhausted or when you need direct system control.

**Usage:**
```
/takeover
```

**Response (when enabling):**
```
Takeover mode ENABLED. You can now send XML commands directly.

Example:
<terminal>ls -la</terminal>
<action type="GIVE">file.txt</action>

Use /takeover again to disable.
```

**Response (when disabling):**
```
Takeover mode DISABLED. Returning to AI agent control.
```

### Available XML Commands in Takeover Mode

#### Terminal Command
Execute shell commands in the container:
```xml
<terminal>ls -la</terminal>
<terminal>cd /app/workspace/work</terminal>
<terminal>npm install</terminal>
```

#### Give File
Deliver a file from `/app/workspace/out/` to the user:
```xml
<action type="GIVE">report.pdf</action>
<action type="GIVE">song.txt</action>
```

#### Publish App
Publish a web app from `/app/workspace/apps/`:
```xml
<action type="APP">myapp</action>
```

#### System Info
Request system information:
```xml
<system>time</system>
<system>memory</system>
```

### Takeover Mode Examples

**List files in workspace:**
```
<terminal>ls -la /app/workspace/</terminal>
```

**Create and deliver a file:**
```
<terminal>echo "Hello World" > /app/workspace/out/greeting.txt</terminal>
<action type="GIVE">greeting.txt</action>
```

**Check memory usage:**
```
<system>memory</system>
```

## File Retrieval

### /give_system_prompt
Sends the current agent system prompt as a text file attachment.

**Usage:**
```
/give_system_prompt
```

**Response:**
A text file containing the full system prompt is sent to the user.

**When to use:**
- To review what instructions the agent follows
- To understand agent capabilities

### /give_context
Sends the current full context window as a text file attachment.

**Usage:**
```
/give_context
```

**Response:**
A text file containing the current conversation context is sent to the user.

**When to use:**
- To review conversation history
- To save conversation for later
- To debug agent behavior

**Note:** If the context is empty, shows "Context is empty."

## Command Flow Examples

### Normal Conversation Flow
1. User sends message
2. AI agent processes and responds with XML tags
3. System executes tags and returns results
4. Response sent to user

### Takeover Flow
1. User types `/takeover`
2. System enables takeover mode
3. User sends XML commands directly
4. System executes and returns results
5. User types `/takeover` again to disable
