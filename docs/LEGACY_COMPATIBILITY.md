# Legacy Compatibility Guide

This document helps future developers distinguish **current** behaviors from **legacy compatibility** shims.

## Current Contract (Use This)

Agent responses should use deterministic XML-style tags:

```text
<thought>Plan</thought>
<message>Short status</message>
<terminal>cd /app/workspace/work && ls -la</terminal>
<action>GIVE:report.txt</action>
```

Canonical fields:
- `<thought>`
- `<message>`
- `<terminal>`
- `<action>`

## Legacy (Do Not Use for New Features)

- `panelActions`
- JSON envelope contract (still parsed but no longer primary)
- Textual `ACTION: EXECUTE` contracts

These may appear in older logs, tests, or historical docs for migration context.


## Runtime Logging Shape

Even when agent output is XML-tagged, orchestrator persistence/history is intentionally normalized to JSON objects:

```json
{"message":"Done","terminal":"","action":"GIVE:file.txt","userId":"123"}
```

This keeps dashboard rendering and downstream tooling deterministic while retaining XML as the agent emission contract.

## Rule of Thumb

If you're adding new behavior, wire it through deterministic `action` values and explicit server-side handlers. Do not add new `panelActions` pathways.
