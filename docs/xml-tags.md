# XML Tags Reference

> See also: [Parser Contract](../internal/parser/contract.go)

## Overview

Hermit agents communicate with the system using XML-like tags. These tags are parsed by the LLM and executed by the system.

## Tag Reference

### `<message>` - Send Telegram Message

Send a message to the user via Telegram.

```xml
<message>Hello! I've completed your task.</message>
```

**Response**: Message sent to Telegram user.

---

### `<terminal>` - Execute Terminal Command

Run a command inside the agent's Docker container.

```xml
<terminal>ls -la /app/workspace/work</terminal>
```

**Response**: Command output returned to the agent.

---

### `<give>` - Send File to User

Send a file from the container's `/app/workspace/out/` folder to the user via Telegram.

```xml
<give>report.pdf</give>
```

The system will:
1. Read `/app/workspace/out/report.pdf` from container
2. Send it as a document to the user

---

### `<app>` - Create Web Application

Create a web application with HTML, CSS, and JavaScript. The system automatically creates the file structure.

```xml
<app name="myapp">
<html>
<!DOCTYPE html>
<html>
<head>
    <title>My App</title>
</head>
<body>
    <h1>Hello World</h1>
    <button id="btn">Click me</button>
</body>
</html>
</html>
<style>
body {
    font-family: sans-serif;
    padding: 20px;
}
h1 { color: #333; }
button {
    padding: 10px 20px;
    background: blue;
    color: white;
    border: none;
    cursor: pointer;
}
</style>
<script>
document.getElementById('btn').addEventListener('click', function() {
    alert('Button clicked!');
});
</script>
</app>
```

**What happens:**
1. System creates folder `/app/workspace/apps/myapp/`
2. Creates `index.html` with embedded CSS and JS
3. Returns a public URL to access the app

**Result**:
```
🚀 App Created: myapp
Access it here: https://your-tunnel-url/api/apps/{agent-id}/myapp
```

---

### `<skill>` - Load Skill Context

Load a skill file to provide context to the agent.

```xml
<skill>python-coding</skill>
```

The system will read `data/skills/python-coding.md` and inject it into the conversation.

---

### `<calendar>` - Schedule Event

Schedule a calendar event. Supports multiple events in a single response.

**Create event:**
```xml
<calendar>
<datetime>2025-05-23T09:00:00</datetime>
<prompt>Time to start the daily standup meeting!</prompt>
</calendar>
```

Or with separate date and time:
```xml
<calendar>
<date>2025-05-23</date>
<time>09:00</time>
<prompt>Daily standup!</prompt>
</calendar>
```

**Multiple events:**
```xml
<calendar>
<datetime>2026-03-17T13:00:00</datetime>
<prompt>First lesson</prompt>
</calendar>
<calendar>
<datetime>2026-03-17T13:05:00</datetime>
<prompt>Second lesson</prompt>
</calendar>
<calendar>
<datetime>2026-03-17T13:10:00</datetime>
<prompt>Third lesson</prompt>
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
<!DOCTYPE html>
<html>
<head><title>Calculator</title></head>
<body>
    <input id="a" type="number" placeholder="First number">
    <select id="op"><option>+</option><option>-</option><option>*</option><option>/</option></select>
    <input id="b" type="number" placeholder="Second number">
    <button onclick="calculate()">=</button>
    <div id="result"></div>
</body>
</html>
</html>
<style>
body { font-family: sans-serif; padding: 20px; text-align: center; }
input, select, button { padding: 10px; margin: 5px; }
#result { font-size: 24px; margin-top: 20px; }
</style>
<script>
function calculate() {
    const a = parseFloat(document.getElementById('a').value);
    const b = parseFloat(document.getElementById('b').value);
    const op = document.getElementById('op').value;
    let result = 0;
    switch(op) { case '+': result = a + b; break; case '-': result = a - b; break; case '*': result = a * b; break; case '/': result = a / b; break; }
    document.getElementById('result').innerText = result;
}
</script>
</app>

<message>Your calculator app is ready!</message>
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
