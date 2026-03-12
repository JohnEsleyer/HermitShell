# Agent Flow Scenarios

## 1) Normal Chat + Tooling

1. User sends a message from Telegram.
2. Hermit forwards context to LLM.
3. LLM returns XML contract.
4. Parser extracts tags (`message`, `terminal`, `action`, `calendar`, `system`).
5. Hermit executes side effects and returns updates.

## 2) Multiple `<terminal>` Tags

If an LLM response contains multiple terminal tags, Hermit parses all of them and places them into a queue.

Example:

```xml
<terminal>npm install</terminal>
<terminal>npm test</terminal>
```

Processing behavior:

- Queue order is preserved.
- First command starts immediately.
- Next command starts only when previous command reaches terminal completion state.
- Status transitions include `ONGOING` then `SUCCESS` or `FAILED`.

## 3) Telegram Takeover Mode

`/takeover` toggles takeover mode.

When enabled, Telegram user XML is treated as **system-control input**, useful for LLM quota exhaustion scenarios.

Supported examples:

```xml
<terminal>cd out</terminal>
<action type="GIVE">report.pdf</action>
```

Notes:

- Re-running `/takeover` exits takeover mode.
- The system should always announce whether takeover mode is ON/OFF.

## 4) `<system>` Tag Usage

Agents can request runtime info without shelling out:

- `<system>time</system>` → localized runtime time.
- `<system>memory</system>` → container/system memory snapshot.

This enables safer and more deterministic status checks.

## 5) Webhook & Tunnel Resilience

On startup (`./hermit`):

1. Dashboard starts.
2. Dashboard public URL is created.
3. Agent start creates per-agent tunnel.
4. Telegram webhook is set to agent public URL.
5. Health monitor periodically validates tunnel + webhook behavior.

If degraded, status is surfaced in the tunnels table and should trigger remediation logic.
