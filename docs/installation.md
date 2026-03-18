# Installation Guide

Documentation for installing HermitShell AI Agent OS on a server.

## Quick Install

Run the automated installation script:

```bash
./install.sh
```

This script will:
1. Install system dependencies (Node.js, Go, Docker, cloudflared)
2. Install frontend dependencies
3. Build the application (frontend, Go server, Docker image)
4. Set up the environment

## Manual Installation

If you prefer to install manually, follow these steps:

### Prerequisites

- **Node.js** 20+
- **Bun** (optional, recommended for UI build)
- **Go** 1.25+
- **Docker**
- **SQLite3**
- **cloudflared** (for tunneling)

### Installation Steps

```bash
# 1. Clone the repository
git clone https://github.com/JohnEsleyer/HermitShell.git
cd HermitShell

# 2. Build the application (Recommended)
make setup
make build

# 3. Alternative: Manual build
# Build frontend
cd dashboard && npm install && npm run build && cd ..
# Build Go server
go build -o hermit-server ./cmd/hermit/main.go
# Build Docker image
docker build -t hermit-agent:latest .

# 4. Run the server
./hermit-server
```

## Configuration

Create a `.env` file with your settings:

```bash
# Server
PORT=3000
DATABASE_PATH=./data/hermit.db

# Telegram (optional)
TELEGRAM_BOT_TOKEN=your_token_here

# LLM Provider
LLM_PROVIDER=openrouter

# API Keys
OPENROUTER_API_KEY=sk-or-...
```

## First Login

- URL: http://localhost:3000
- Username: admin
- Password: hermit123 (required to change in the settings dashboard after first login)

## Production Setup

### Systemd Service (Linux)

```bash
sudo cp hermit.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable hermit
sudo systemctl start hermit
```

### Reverse Proxy

For production, run behind nginx with SSL:

```nginx
server {
    listen 80;
    server_name your-domain.com;
    
    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

### Docker

Make sure Docker is running:

```bash
docker run -d \
  --name hermit-agent \
  -v hermit-workspace:/app/workspace \
  hermit-agent:latest
```

## Troubleshooting

### Port 3000 already in use

Change the port in `.env`:
```
PORT=3001
```

### Docker permission denied

Add your user to the docker group:
```bash
sudo usermod -aG docker $USER
# Then logout and login again
```

### Cloudflare tunnel not working

Check cloudflared installation:
```bash
cloudflared --version
```

## CLI Usage

HermitShell includes a CLI tool called `hermitshell` to manage the system from the terminal.

On the first CLI run, HermitShell prints the default credentials (`admin / hermit123`) before prompting for login. After signing in, changing those credentials in the settings dashboard is required before regular use.

### Authentication
The CLI will prompt for credentials on first use and save them to `~/.hermit/credentials`. You can also automate this by setting these environment variables in your `.env` file:
- `HERMIT_API_BASE`: URL of the Hermit server (default: `http://localhost:3000`)
- `HERMIT_CLI_USER`: Username for auto-login
- `HERMIT_CLI_PASS`: Password for auto-login

### Common Commands
- `hermitshell agents list`: List all agents and their status.
- `hermitshell containers list`: View real-time container metrics (CPU/Memory).
- `hermitshell tunnel`: Display the current public tunnel URL.
- `hermitshell logout`: Clear session and cached credentials.

### Service Management
You can control the Hermit server service directly from the CLI:
- **`hermitshell status`**: Check if the server is running and the API is responsive.
- **`hermitshell start`**: Start the Hermit background service (`systemctl`).
- **`hermitshell stop`**: Stop the Hermit background service.
- **`hermitshell restart`**: Restart the Hermit background service.

## Updating

```bash
git pull
make all
```

## Uninstall

```bash
# Stop the server
hermitshell stop

# Remove files (optional)
rm -rf hermit-server data/ hermit-agent:latest
```
