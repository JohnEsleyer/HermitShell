# Hermit Technical Architecture

## Public URL Strategy

Hermit now treats public URLs as a core runtime dependency:

- Telegram webhooks require a publicly reachable endpoint.
- Agent apps in `/workspace/apps/<app-name>` are exposed through reverse proxy paths like `/apps/<app-name>`.
- By default, Hermit uses `cloudflared tunnel --url ...` quick tunnels (no Cloudflare account token required).
- Optional **Domain Mode** lets operators provide dashboard and agents domains with HTTPS via Let's Encrypt.

## Tunnel Health Monitoring

Tunnel health is assessed through:

1. Reachability checks to quick tunnel URLs.
2. Telegram webhook diagnostics (e.g. last error reported by Telegram).
3. Tunnel status updates in DB as `healthy` / `degraded`.

## Metrics

The metrics panel reads real host and container data:

- Host CPU and memory from `/proc`.
- Container CPU and memory from `docker stats --no-stream`.
- Auto-refresh timers in dashboard for near real-time visualization.
