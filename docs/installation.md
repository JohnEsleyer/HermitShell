# Installation Guide

Documentation for installing Hermit AI Agent OS on a server.

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
- **Go** 1.25+
- **Docker**
- **SQLite3**
- **cloudflared** (for tunneling)

### Installation Steps

```bash
# 1. Clone the repository
git clone https://github.com/JohnEsleyer/hermit.git
cd hermit

# 2. Install frontend dependencies
cd dashboard
npm install
npm run build
cd ..

# 3. Build Go server
go build -o hermit ./cmd/hermit/main.go

# 4. Build Docker image
docker build -t hermit-agent:latest .

# 5. Run the server
./hermit
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
- Password: hermit123 (change on first login!)

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

## Updating

```bash
git pull
make all
```

## Uninstall

```bash
# Stop the server
pkill hermit

# Remove files (optional)
rm -rf hermit data/ hermit-agent:latest
```
