# Legacy Compatibility Guide

This document helps future developers distinguish **current** behaviors from **legacy compatibility** shims.

## Current Contract (Use This)

Agent responses should use deterministic JSON:

```json
{
  "userId": "123456789",
  "message": "Short status",
  "terminal": "cd /app/workspace/work && ls -la",
  "action": "GIVE:report.txt"
}
```

Supported fields:
- `userId`
- `message`
- `terminal`
- `action`

## Legacy (Do Not Use for New Features)

- `panelActions`
- Textual `ACTION: EXECUTE` contracts

These may appear in older logs, tests, or historical docs for migration context.

## Rule of Thumb

If you're adding new behavior, wire it through deterministic `action` values and explicit server-side handlers. Do not add new `panelActions` pathways.
