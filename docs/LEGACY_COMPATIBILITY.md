# Legacy Compatibility Guide

This document helps future developers distinguish **current** behaviors from **legacy compatibility** shims.

## Current Contract (Use This)

Agent responses should use deterministic XML-style tags:

```text
<thought>Plan</thought>
<message>Short status</message>
<action>TERMINAL:cd /app/workspace/work && ls -la</action>
<calendar>
<datetime>2026-03-10T08:00:00Z</datetime>
<prompt>Remind user to drink water</prompt>
</calendar>
```

Canonical fields:
- `<thought>`
- `<message>`
- `<action>` (with type prefixes: `TERMINAL:`, `GIVE:`, `APP:`, `SKILL:`)
- `<calendar>`

**Current System Time (UTC):** `{{CURRENT_UTC_TIME}}` is injected at runtime.

## Legacy (Do Not Use for New Features)

- `<terminal>` tag (deprecated - use `<action>TERMINAL:...</action>` instead)
- JSON envelope contract (still parsed but no longer primary)
- Textual `ACTION: EXECUTE` contracts
- SQL-based calendar scheduling (deprecated - use `<calendar>` tags)

These may appear in older logs, tests, or historical docs for migration context.


## Runtime Logging Shape

Even when agent output is XML-tagged, orchestrator persistence/history is intentionally normalized to JSON objects:

```json
{"message":"Done","action":"TERMINAL:ls -la","userId":"123"}
```

This keeps dashboard rendering and downstream tooling deterministic while retaining XML as the agent emission contract. Agent Test dashboard includes a help (`?`) control explaining this dual-format flow.

## Rule of Thumb

If you're adding new behavior, wire it through deterministic `action` values (with type prefixes) and explicit server-side handlers. Do not add new ad-hoc panel action pathways.
