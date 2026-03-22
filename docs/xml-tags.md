# XML Tags Reference

> See also: [Parser Contract](../internal/parser/contract.go)

## Overview

Hermit agents communicate with the system using XML-like tags. These tags are parsed by the LLM and executed by the system.

## Tag Reference

### `<message>` - Send User Message

Send a message to the active user transport. In Telegram mode it goes to Telegram; in HermitChat mode it is pushed into the HermitChat conversation as the agent.

```xml
<message>Hello! I've completed your task.</message>
```

**Response**: Message sent to the active user transport.

---

### `<terminal>` - Execute Terminal Command

Run a command inside the agent's Docker container.

```xml
<terminal>ls -la /app/workspace/work</terminal>
```

**Response**: Command output returned to the agent.

---

### `<give>` - Send File to User

Send a file from the container's `/app/workspace/out/` folder to the active user transport. Telegram sends it as a document/photo/video depending on the file type; HermitChat exposes it as an attachment in the conversation.

```xml
<give>report.pdf</give>
```

The system will:
1. Read `/app/workspace/out/report.pdf` from container
2. Send it as a document to the user

---

### `<app>` - Create Web Application

Create a web application with HTML, CSS, and JavaScript. The system automatically creates the file structure in the agent's container.

```xml
<app name="myapp">
<html>
  <h1>Hello World</h1>
</html>
<style>
  h1 { color: blue; }
</style>
<script>
  console.log('App loaded');
</script>
</app>
```

**What happens:**
1. System creates folder `/app/workspace/apps/myapp/` inside the container.
2. Creates `index.html` with embedded CSS and JS.
3. The app is stored but not yet "published" to the user with a public URL (use `<deploy>` for that).

---

### `<deploy>` - Publish Web Application

Publish a previously created app and generate a public URL.

```xml
<deploy>myapp</deploy>
```

**What happens:**
1. System verifies the app exists in `/app/workspace/apps/myapp/`.
2. Generates a public URL.
3. Sends the URL to the user via Telegram.
4. The app appears in the **Apps** panel of the dashboard.

**Result**:
```
🚀 App Deployed: myapp
Access it here: https://your-tunnel-url/apps/{agent-id}/myapp
```

---

### `<skill>` - Load Skill Context

Load a skill file to provide context to the agent.

```xml
<skill>python-coding</skill>
```

The system will read `data/skills/python-coding.md` and inject it into the conversation.

---

### `<schedule>` - Relative Time Scheduling (Recommended)

Schedule a reminder using relative time. The server automatically calculates the absolute datetime from the current time + your specified duration. This is the **preferred method** for scheduling reminders.

**Attributes:**
- `minutes="N"` - Schedule N minutes from now (optional)
- `hours="N"` - Schedule N hours from now (optional)
- `days="N"` - Schedule N days from now (optional)
- Use any combination: minutes only, hours only, days only, or mix them

**Content:** The reminder text/prompt

**Examples:**

```xml
<!-- In 3 minutes -->
<schedule minutes="3">Time to take a break!</schedule>

<!-- In 2 hours -->
<schedule hours="2">Reminder: Meeting starts soon</schedule>

<!-- In 1 day -->
<schedule days="1">Don't forget your appointment tomorrow</schedule>

<!-- 30 minutes AND 2 hours = 2.5 hours from now -->
<schedule hours="2" minutes="30">Two and a half hour reminder</schedule>

<!-- 1 day and 9 hours from now -->
<schedule days="1" hours="9">Morning reminder for tomorrow</schedule>
```

**Why use `<schedule>` instead of `<calendar>`?**
- No need to calculate absolute datetime yourself
- Works correctly regardless of timezone confusion
- Simpler and less error-prone for relative time requests

---

### `<calendar>` - Absolute Time Scheduling (Alternative)

Schedule a calendar event. Supports multiple events in a single response.

**Create event:**
```xml
<calendar>
<datetime>2025-05-23T09:00:00</datetime>
<prompt>Time to start the daily standup meeting!</prompt>
</calendar>
```

Or with separate date and time (fallback if datetime is missing):
```xml
<calendar>
<date>2025-05-23</date>
<time>09:00</time>
<prompt>Daily standup!</prompt>
</calendar>
```

**Multiple events in one response:**
```xml
<calendar>
<datetime>2026-03-17T13:00</datetime>
<prompt>First reminder</prompt>
</calendar>
<calendar>
<datetime>2026-03-17T13:05</datetime>
<prompt>Second reminder</prompt>
</calendar>
```

**List all events:**
```xml
<calendar action="list"/>
```

**Delete an event:**
```xml
<calendar action="delete" id="123"/>
```

**Update an event:**
```xml
<calendar action="update" id="456"><prompt>Updated prompt</prompt></calendar>
```

---

### `<thought>` - Internal Thought

Internal thought that is logged but not sent to the user.

```xml
<thought>The user wants me to analyze this file. I'll first check its contents.</thought>
```

---

### `<system>` - Request System Information

Request system information.

```xml
<system>time</system>
<system>memory</system>
```

Returns:
- `time`: Current server time
- `memory`: Current memory usage

---

## Legacy Tags (Backward Compatible)

### `<action type="GIVE">filename</action>`

Legacy syntax for giving files. Still works but `<give>` is preferred.

### `<action type="APP">appname</action>`

Legacy syntax for publishing apps. The new `<app>` tag is recommended.

---

## Complete Example

```xml
<thought>Analyzing the user's request to create a calculator app.</thought>

<message>I'll create a simple calculator app for you!</message>

<terminal>mkdir -p /app/workspace/apps/calculator</terminal>

<app name="calculator">
<html>
  <h1>Calculator</h1>
  <input id="a" type="number">
  <select id="op"><option>+</option><option>-</option></select>
  <input id="b" type="number">
  <button onclick="calc()">=</button>
  <p id="res"></p>
</html>
<script>
function calc() {
  const a = parseFloat(document.getElementById('a').value);
  const b = parseFloat(document.getElementById('b').value);
  const op = document.getElementById('op').value;
  document.getElementById('res').innerText = op === '+' ? a + b : a - b;
}
</script>
</app>

<deploy>calculator</deploy>

<message>Your calculator app is ready and deployed!</message>
```

---

## Parsing Flow

1. **LLM Response** → Agent sends XML tags
2. **Parser** (`internal/parser/contract.go`) → Extracts tags
3. **ExecuteXMLPayload** (`internal/api/server.go`) → Processes each tag
4. **Feedback** → Results sent back to agent

---

## Related Files

- Parser: `internal/parser/contract.go`
- Executor: `internal/api/server.go` (ExecuteXMLPayload function)
