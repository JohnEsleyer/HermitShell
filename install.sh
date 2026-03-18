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
    
    # Install Node.js if not present
    if ! command -v node &> /dev/null; then
        print_info "Installing Node.js..."
        curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
        sudo apt-get install -y nodejs
    fi
    
    # Install Go if not present
    if ! command -v go &> /dev/null; then
        print_info "Installing Go..."
        wget -q https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
        rm go1.25.0.linux-amd64.tar.gz
        export PATH=$PATH:/usr/local/go/bin
    fi

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
    npm install --silent 2>/dev/null || npm install
    cd ..
    print_success "Frontend dependencies installed!"
else
    print_warning "Dashboard directory not found, skipping npm install"
fi

# ============================================
# 3. Build the Application
# ============================================
print_info "Building the application..."

# Build frontend
if [ -d "dashboard" ]; then
    print_info "Building frontend..."
    cd dashboard
    npm run build
    cd ..
    print_success "Frontend built!"
fi

# Build Go server
print_info "Building Go server..."
go build -o hermit ./cmd/hermit/main.go
print_success "Go server built!"

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

# Create systemd service file (optional, for Linux)
if [[ "$OS" == "linux-gnu"* ]] && [ "$EUID" -eq 0 ]; then
    print_info "Creating systemd service..."
    
    cat > /etc/systemd/system/hermit.service << 'EOF'
[Unit]
Description=Hermit AI Agent OS
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu/hermitclaw/hermit
ExecStart=/home/ubuntu/hermitclaw/hermit/hermit
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
    
    sudo systemctl daemon-reload
    print_success "Systemd service created! You can enable with: sudo systemctl enable hermit"
fi

# ============================================
# 5. Summary
# ============================================
echo ""
echo "=========================================="
echo "  Installation Complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Configure your API keys in .env (or via dashboard)"
echo "  2. Run: ./hermit"
echo "  3. Open: http://localhost:3000"
echo "  4. Login with: admin / hermit123"
echo ""
echo "For production, consider:"
echo "  - Running behind a reverse proxy (nginx/traefik)"
echo "  - Setting up a custom domain"
echo "  - Enabling SSL/TLS"
echo "  - Configuring firewall rules"
echo ""
print_success "Ready to start Hermit!"
echo ""
