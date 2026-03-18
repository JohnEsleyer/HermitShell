#!/bin/bash

# Hermit AI Agent OS - Installation Script
# Docs: See docs/installation.md for detailed installation guide

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored functions
print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    print_warning "Running as root - this is not required but ensure you know what you're doing"
fi

echo ""
echo "=========================================="
echo "  Hermit AI Agent OS - Installer"
echo "=========================================="
echo ""

# Detect OS
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
    else
        OS="linux"
    fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
else
    OS="unknown"
fi

print_info "Detected OS: $OS"

# ============================================
# 1. Install System Dependencies
# ============================================
print_info "Installing system dependencies..."

# Update package list
if command -v apt-get &> /dev/null; then
    sudo apt-get update -qq
    
    # Install essential dependencies
    sudo apt-get install -y -qq \
        curl \
        wget \
        git \
        build-essential \
        sqlite3 \
        ca-certificates \
        gnupg \
        lsb-release \
        unzip \
        2>/dev/null || true
    
    # Install Docker if not present
    if ! command -v docker &> /dev/null; then
        print_info "Installing Docker..."
        curl -fsSL https://get.docker.com | sh
        sudo usermod -aG docker $USER
        print_success "Docker installed. You may need to logout/login for group changes to take effect."
    fi
    
    # Install Bun for frontend builds
    if ! command -v bun &> /dev/null; then
        print_info "Installing Bun..."
        curl -fsSL https://bun.sh/install | bash
    fi
    
    # Add Bun to PATH permanently
    if ! grep -q 'bun/install' ~/.bashrc 2>/dev/null; then
        echo 'export BUN_INSTALL="$HOME/.bun"' >> ~/.bashrc
        echo 'export PATH="$BUN_INSTALL/bin:$PATH"' >> ~/.bashrc
    fi
    export BUN_INSTALL="$HOME/.bun"
    export PATH="$BUN_INSTALL/bin:$PATH"
    
    # Install Go if not present
    if ! command -v go &> /dev/null; then
        print_info "Installing Go..."
        wget -q https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
        rm go1.25.0.linux-amd64.tar.gz
    fi
    
    # Add Go to PATH permanently
    if ! grep -q '/usr/local/go/bin' ~/.bashrc 2>/dev/null; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    fi
    export PATH=$PATH:/usr/local/go/bin

elif [[ "$OS" == "macos" ]]; then
    # Install Homebrew if not present
    if ! command -v brew &> /dev/null; then
        print_info "Installing Homebrew..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    fi
    
    # Install dependencies via Homebrew
    brew install node docker go sqlite3 2>/dev/null || true
    
    # Install Docker Desktop or Colima if Docker not available
    if ! command -v docker &> /dev/null; then
        print_warning "Docker not found. Please install Docker Desktop for Mac:"
        print_warning "  https://www.docker.com/products/docker-desktop"
    fi
else
    print_warning "Unsupported OS. Please install dependencies manually:"
    print_warning "  - Node.js 20+"
    print_warning "  - Go 1.25+"
    print_warning "  - Docker"
    print_warning "  - SQLite3"
fi

# Install cloudflared for tunneling
if ! command -v cloudflared &> /dev/null; then
    print_info "Installing cloudflared..."
    if [[ "$OS" == "linux-gnu"* ]]; then
        wget -q https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -O /tmp/cloudflared
        sudo mv /tmp/cloudflared /usr/local/bin/cloudflared
        sudo chmod +x /usr/local/bin/cloudflared
    elif [[ "$OS" == "macos" ]]; then
        brew install cloudflared
    fi
fi

print_success "System dependencies installed!"

# ============================================
# 2. Install Frontend Dependencies
# ============================================
print_info "Installing frontend dependencies..."

if [ -d "dashboard" ]; then
    cd dashboard
    bun install --silent 2>/dev/null || bun install
    cd ..
    print_success "Frontend dependencies installed!"
else
    print_warning "Dashboard directory not found, skipping bun install"
fi

# ============================================
# 3. Build the Application
# ============================================
print_info "Building the application..."

# Build frontend
if [ -d "dashboard" ]; then
    print_info "Building frontend..."
    cd dashboard
    bun run build
    cd ..
    print_success "Frontend built!"
fi

# Build Go server
print_info "Building Go server..."
go build -o hermit ./cmd/hermit/main.go
print_success "Go server built!"

# Build CLI
print_info "Building CLI..."
go build -o hermit-cli ./cmd/cli/main.go
print_success "CLI built!"

# Build Docker image
print_info "Building Docker image (hermit-agent:latest)..."
docker build -t hermit-agent:latest . --quiet
print_success "Docker image built!"

# ============================================
# 4. Setup Environment
# ============================================
print_info "Setting up environment..."

# Create data directory
mkdir -p data/image data/skills data/agents

# Create .env file if it doesn't exist
if [ ! -f .env ]; then
    cat > .env << 'EOF'
# Hermit Configuration
# Server
PORT=3000
DATABASE_PATH=./data/hermit.db

# Telegram Bot (optional)
# TELEGRAM_BOT_TOKEN=your_bot_token_here

# LLM Provider (openrouter, openai, anthropic, gemini)
LLM_PROVIDER=openrouter

# API Keys (recommended: configure via dashboard Settings panel)
# OPENROUTER_API_KEY=sk-or-...
# OPENAI_API_KEY=sk-...
# ANTHROPIC_API_KEY=sk-ant-...
# GEMINI_API_KEY=AIza...
EOF
    print_success "Created .env file - please configure your API keys"
fi

# Setup systemd for process management
print_info "Setting up systemd service..."

# Get absolute path of current directory
HERMIT_DIR=$(pwd)

# Detect user for systemd service
SERVICE_USER=$(whoami)
if [ "$EUID" -eq 0 ]; then
    SERVICE_USER="ubuntu"
fi

# Create systemd service file
cat > hermit.service << EOF
[Unit]
Description=Hermit AI Agent OS
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=${SERVICE_USER}
WorkingDirectory=${HERMIT_DIR}
ExecStart=${HERMIT_DIR}/hermit
Restart=always
RestartSec=10
StandardOutput=append:${HERMIT_DIR}/data/logs/hermit.log
StandardError=append:${HERMIT_DIR}/data/logs/hermit-error.log

[Install]
WantedBy=multi-user.target
EOF

# Create logs directory
mkdir -p data/logs

# Install systemd service (if running as root)
if [ "$EUID" -eq 0 ]; then
    sudo cp hermit.service /etc/systemd/system/hermit.service
    sudo systemctl daemon-reload
    sudo systemctl enable hermit
    sudo systemctl start hermit
    print_success "Systemd service installed and started!"
else
    print_warning "Not running as root. To enable auto-start on boot, run:"
    echo "  sudo cp hermit.service /etc/systemd/system/hermit.service"
    echo "  sudo systemctl daemon-reload"
    echo "  sudo systemctl enable hermit"
    echo "  sudo systemctl start hermit"
    print_info "Starting Hermit in background..."
    nohup ./hermit > data/logs/hermit.log 2>&1 &
fi

# Wait for Hermit to start and get tunnel URL
print_info "Waiting for Hermit to start..."
sleep 5

# Try to get tunnel URL from API
TUNNEL_URL=""
for i in 1 2 3 4 5 6; do
    TUNNEL_URL=$(curl -s http://localhost:3000/api/tunnel-url 2>/dev/null | grep -o '"url":"[^"]*' | cut -d'"' -f4)
    if [ -n "$TUNNEL_URL" ]; then
        break
    fi
    sleep 2
done

# ============================================
# 5. Summary
# ============================================
echo ""
echo "=========================================="
echo "  Installation Complete!"
echo "=========================================="
echo ""
echo "  🚀 ACCESS YOUR DASHBOARD:"
echo ""
if [ -n "$TUNNEL_URL" ]; then
    echo "     🌐 Public URL: $TUNNEL_URL"
else
    echo "     💻 Local:     http://localhost:3000"
fi
echo "     👤 Username:  admin"
echo "     🔑 Password:  hermit123"
echo ""
echo "  To get tunnel URL later:"
echo "    curl http://localhost:3000/api/tunnel-url"
echo "    journalctl -u hermit -f | grep 'Public URL'"
echo ""
echo "  Systemd Commands (if enabled):"
echo "    sudo systemctl status hermit   - Check status"
echo "    sudo systemctl restart hermit  - Restart"
echo "    journalctl -u hermit -f       - View logs"
echo ""
echo "  Or manually:"
echo "    ./hermit                      - Run directly"
echo "    tail -f data/logs/hermit.log  - View logs"
echo ""
echo "  For production, consider:"
echo "    - Running behind a reverse proxy (nginx/traefik)"
echo "    - Setting up a custom domain"
echo "    - Enabling SSL/TLS"
echo ""
print_success "Ready to start Hermit!"
echo ""
