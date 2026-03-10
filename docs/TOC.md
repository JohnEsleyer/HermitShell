# HermitShell Documentation - Table of Contents

Welcome to the official documentation for **HermitShell**, the autonomous AI agent orchestrator. This guide will walk you through the system architecture, its core components, and how the various parts interact to create a secure, air-gapped environment for AI agents.

## 📖 Core Documentation

1.  **[System Architecture](./ARCHITECTURE.md)**
    *   High-level overview of the Orchestrator, Docker Cubicles, and Communication Proxy.
2.  **[The Orchestrator Shell](./ORCHESTRATOR.md)**
    *   Details on the Node.js Fastify server, API endpoints, and the internal proxy.
3.  **[Docker Cubicles & Isolation](./DOCKER_CUBICLES.md)**
    *   How containers are managed, workspace isolation (`/out`, `/in`, `/www`), and lifecycle hooks.
4.  **[The Agent Runtime](./AGENT_RUNTIME.md)**
    *   The core TypeScript/Node.js logic running inside the cubicle: command execution, LLM calls via proxy, and safety guards.
5.  **[Telegram Integration & The Portal](./TELEGRAM_INTEGRATION.md)**
    *   Handling Telegram webhooks, file uploads/downloads, and real-time interaction logs.
6.  **[Database & Storage](./DATABASE_AND_STORAGE.md)**
    *   libSQL schema overview and audit trail persistence.
7.  **[Security & HITL](./SECURITY.md)**
    *   Authentication, session management, and the Human-in-the-Loop approval system.

## 🚀 Operations

*   **[Deployment Guide](./DEPLOYMENT.md)**
    *   Setting up the orchestrator, configuring Cloudflare Tunnels, and initializing agents.
*   **[Contributing](./CONTRIBUTING.md)**
    *   Code style, testing procedures (Vitest), and adding new features.

## 📝 Notes

- The dashboard terminology uses **Apps** (formerly "Sites").
- Agent/controller interaction is hybrid by design: agents emit XML-tag contracts (`<thought>`, `<message>`, `<action>`, `<calendar>`), while orchestrator histories/logs are normalized JSON for deterministic UI/search. The `action` tag now includes type prefixes (`TERMINAL:`, `GIVE:`, `APP:`, `SKILL:`). JSON/labeled input parsing remains compatibility-only, and legacy ad-hoc panel action channels are removed.
- Calendar scheduling can use `<calendar>` tags (recommended) or workspace `calendar.db` with `agent_calendar` + `task_history` as the primary self-CRON schema.
