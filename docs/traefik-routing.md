# Traefik Multi-Tenant Routing

> See also: [Container Management](./container-management.md), [Docker Client](./internal/docker/cubicle.go)

## Overview

Hermit uses Traefik as a reverse proxy to route traffic to multiple agent containers. Each agent container hosts web applications, and Traefik routes requests based on path prefixes.

## Architecture

```
User Request
     │
     ▼
┌─────────────────────────────────────────┐
│           Traefik (Port 3000)           │
│                                         │
│  /agent/{container-name}/{app-path}    │
└─────────────┬───────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────┐
│     hermit-network (Docker Network)     │
│                                         │
│  ┌──────────────┐  ┌──────────────┐   │
│  │ agent-rain   │  │ agent-john   │   │
│  │ :80          │  │ :80          │   │
│  └──────────────┘  └──────────────┘   │
└─────────────────────────────────────────┘
```

## Routing Format

```
http://localhost:3000/agent/{container-name}/{app-path}

Examples:
http://localhost:3000/agent/agent-rain/calculator/
http://localhost:3000/agent/agent-john/todo/
```

## Setup

### 1. Start Traefik

```bash
docker network create hermit-network
docker compose -f docker-compose.traefik.yml up -d
```

### 2. Traefik Configuration

**docker-compose.traefik.yml**:
```yaml
services:
  traefik:
    image: traefik:v3.0
    command:
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--providers.docker.network=hermit-network"
    ports:
      - "3000:80"      # HTTP
      - "3001:8080"    # Dashboard
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks:
      - hermit-network
```

### 3. Container Labels

When an agent container is created, Traefik labels are automatically added:

**File: `internal/docker/cubicle.go`**
```go
labels := map[string]string{
    "traefik.enable":                        "true",
    "traefik.http.routers.agent.rule":       "PathPrefix(`/agent/" + name + "/`)",
    "traefik.http.routers.agent.entrypoints": "web",
    "traefik.http.services.agent.loadbalancer.server.url": "http://" + name + ":80",
    "traefik.docker.network":                "hermit-network",
}
```

## How It Works

### 1. Container Creation

When `docker.Run(name, image, detach)` is called:
1. Creates container with Traefik labels
2. Connects to `hermit-network`
3. Traefik automatically discovers the container

### 2. App Publishing

When an agent uses `<app name="calculator">`:
1. System creates `/workspace/apps/calculator/` in container
2. Agent's web server (if running) serves on port 80
3. Traefik routes `/agent/{container-name}/calculator/*` to the container

### 3. Request Flow

```
GET /agent/agent-rain/calculator/index.html
    │
    ▼
Traefik (localhost:3000)
    │
    │ Path prefix: /agent/agent-rain/
    ▼
agent-rain container:80/calculator/index.html
```

## Files Created by `<app>` Tag

```
/app/workspace/apps/{app-name}/
└── index.html    # Contains embedded CSS and JS
```

The agent can also run a simple HTTP server to serve these files:

```xml
<terminal>cd /app/workspace/apps/calculator && python3 -m http.server 80</terminal>
```

## Troubleshooting

### Check Traefik Dashboard
```
http://localhost:3001
```

### Check Routes
```bash
curl http://localhost:3000/api/http/routers
```

### Container Network
Ensure containers are on `hermit-network`:
```bash
docker network inspect hermit-network
```

### Logs
```bash
docker logs hermit-traefik
```

## Cheatsheet

| Command | Description |
|---------|-------------|
| `docker compose -f docker-compose.traefik.yml up -d` | Start Traefik |
| `docker logs -f hermit-traefik` | View logs |
| `curl localhost:3001/api/http/routers` | Check routes |

## Related Files

- Docker Client: `internal/docker/cubicle.go`
- Traefik Compose: `docker-compose.traefik.yml`
- Dynamic Config: `traefik-dynamic.yml`
