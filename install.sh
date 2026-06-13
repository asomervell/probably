#!/bin/sh
# Probably - Personal Finance Tracker
# Install script: curl -fsSL https://probably.money/install.sh | sh
set -e

# Colors (if terminal supports it)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Disable colors if not a terminal
if [ ! -t 1 ]; then
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    BOLD=''
    NC=''
fi

print_banner() {
    echo ""
    echo "${BLUE}╔═══════════════════════════════════════════╗${NC}"
    echo "${BLUE}║${NC}   ${BOLD}Probably${NC} - Personal Finance Tracker    ${BLUE}║${NC}"
    echo "${BLUE}╚═══════════════════════════════════════════╝${NC}"
    echo ""
}

info() {
    echo "${BLUE}==>${NC} ${BOLD}$1${NC}"
}

success() {
    echo "${GREEN}==>${NC} ${BOLD}$1${NC}"
}

warn() {
    echo "${YELLOW}==>${NC} ${BOLD}$1${NC}"
}

error() {
    echo "${RED}==>${NC} ${BOLD}$1${NC}"
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
    
    case "$OS" in
        darwin|linux) ;;
        *) error "Unsupported OS: $OS"; exit 1 ;;
    esac
}

# Check if a command exists
has() {
    command -v "$1" >/dev/null 2>&1
}

# Check for Go installation
check_go() {
    if has go; then
        GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
        MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
        
        if [ "$MAJOR" -gt 1 ] || { [ "$MAJOR" -eq 1 ] && [ "$MINOR" -ge 21 ]; }; then
            success "Go $GO_VERSION found"
            return 0
        else
            warn "Go $GO_VERSION found, but 1.21+ is required"
            return 1
        fi
    else
        return 1
    fi
}

# Install Go (provide instructions)
install_go_instructions() {
    echo ""
    error "Go 1.21+ is required but not found."
    echo ""
    echo "Install Go using one of these methods:"
    echo ""
    
    if [ "$OS" = "darwin" ]; then
        echo "  ${BOLD}Homebrew (recommended):${NC}"
        echo "    brew install go"
        echo ""
        echo "  ${BOLD}Official installer:${NC}"
        echo "    https://go.dev/dl/"
    else
        echo "  ${BOLD}Package manager:${NC}"
        echo "    # Ubuntu/Debian"
        echo "    sudo apt install golang-go"
        echo ""
        echo "    # Fedora"
        echo "    sudo dnf install golang"
        echo ""
        echo "    # Arch"
        echo "    sudo pacman -S go"
        echo ""
        echo "  ${BOLD}Official installer:${NC}"
        echo "    https://go.dev/dl/"
    fi
    echo ""
    echo "After installing Go, run this script again:"
    echo "  curl -fsSL https://probably.money/install.sh | sh"
    echo ""
}

# Default install directory
INSTALL_DIR="${PROBABLY_INSTALL_DIR:-$HOME/.probably}"
BIN_DIR="${PROBABLY_BIN_DIR:-$HOME/.local/bin}"

print_banner
detect_platform

info "Detected platform: $OS/$ARCH"

# Check for Go
if ! check_go; then
    install_go_instructions
    exit 1
fi

# Check for git
if ! has git; then
    error "Git is required but not found."
    echo ""
    if [ "$OS" = "darwin" ]; then
        echo "Install with: xcode-select --install"
    else
        echo "Install with your package manager (apt install git, etc.)"
    fi
    exit 1
fi

# Clone or update repository
if [ -d "$INSTALL_DIR" ]; then
    info "Updating existing installation..."
    cd "$INSTALL_DIR"
    git fetch origin
    git reset --hard origin/main
else
    info "Cloning Probably to $INSTALL_DIR..."
    git clone --depth 1 https://github.com/asomervell/probably.git "$INSTALL_DIR"
    cd "$INSTALL_DIR"
fi

# Build the server
info "Building Probably..."
cd "$INSTALL_DIR"
go build -o probably-server ./cmd/server

# Create bin directory and symlink
mkdir -p "$BIN_DIR"
ln -sf "$INSTALL_DIR/probably-server" "$BIN_DIR/probably"

# Check if BIN_DIR is in PATH
case ":$PATH:" in
    *":$BIN_DIR:"*) IN_PATH=true ;;
    *) IN_PATH=false ;;
esac

echo ""
success "Probably installed successfully!"
echo ""

if [ "$IN_PATH" = false ]; then
    warn "$BIN_DIR is not in your PATH"
    echo ""
    echo "Add it to your shell config:"
    echo ""
    
    if [ -f "$HOME/.zshrc" ]; then
        echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.zshrc"
        echo "  source ~/.zshrc"
    elif [ -f "$HOME/.bashrc" ]; then
        echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc"
        echo "  source ~/.bashrc"
    else
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    fi
    echo ""
    echo "Or run directly:"
    echo "  ${BOLD}$INSTALL_DIR/probably-server${NC}"
else
    echo "Run Probably with:"
    echo "  ${BOLD}probably${NC}"
fi

echo ""
echo "Then open: ${BLUE}http://localhost:8080${NC}"
echo ""
echo "─────────────────────────────────────────────"
echo ""
echo "📁 Installation directory: $INSTALL_DIR"
echo "🗄️  Data stored in:"
if [ "$OS" = "darwin" ]; then
    echo "   ~/Library/Application Support/Probably/"
else
    echo "   ~/.local/share/probably/"
fi
echo ""
echo "To uninstall:"
echo "  rm -rf $INSTALL_DIR $BIN_DIR/probably"
echo ""
