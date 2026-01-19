#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXTENSION_UUID="voicify@dooshek.com"
EXTENSION_DIR="$HOME/.local/share/gnome-shell/extensions/$EXTENSION_UUID"
BIN_DIR="$HOME/bin"

echo "=== Voicify Installer ==="
echo ""

# Check for Go
if ! command -v go &> /dev/null; then
    echo "ERROR: Go is not installed. Please install Go first."
    echo "       https://go.dev/doc/install"
    exit 1
fi

# Check for required system dependencies
echo "Checking dependencies..."
MISSING_DEPS=""

if ! command -v ffmpeg &> /dev/null; then
    MISSING_DEPS="$MISSING_DEPS ffmpeg"
fi

if ! command -v xclip &> /dev/null; then
    MISSING_DEPS="$MISSING_DEPS xclip"
fi

if [ -n "$MISSING_DEPS" ]; then
    echo "WARNING: Missing optional dependencies:$MISSING_DEPS"
    echo "         Install with: sudo dnf install$MISSING_DEPS"
    echo ""
fi

# Build binary
echo "Building voicify daemon..."
cd "$SCRIPT_DIR"
go build -o bin/voicify ./cmd/voicify
echo "  Built: bin/voicify"

# Install binary
echo "Installing binary to $BIN_DIR..."
mkdir -p "$BIN_DIR"
cp bin/voicify "$BIN_DIR/"
chmod +x "$BIN_DIR/voicify"
echo "  Installed: $BIN_DIR/voicify"

# Check if ~/bin is in PATH
if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
    echo ""
    echo "WARNING: $BIN_DIR is not in your PATH"
    echo "         Add this to your ~/.bashrc or ~/.zshrc:"
    echo "         export PATH=\"\$HOME/bin:\$PATH\""
    echo ""
fi

# Install GNOME extension
echo "Installing GNOME Shell extension..."
mkdir -p "$EXTENSION_DIR"
cp -r "$SCRIPT_DIR/gnome-extension/"* "$EXTENSION_DIR/"

# Compile schemas
if [ -d "$EXTENSION_DIR/schemas" ]; then
    glib-compile-schemas "$EXTENSION_DIR/schemas/" 2>/dev/null || true
fi
echo "  Installed: $EXTENSION_DIR"

# Create config directory
CONFIG_DIR="$HOME/.config/voicify"
mkdir -p "$CONFIG_DIR/recordings"
echo "  Config dir: $CONFIG_DIR"

echo ""
echo "=== Installation complete ==="
echo ""
echo "Next steps:"
echo "  1. Restart GNOME Shell (Alt+F2 -> 'r' on X11, or logout/login on Wayland)"
echo "  2. Enable the extension:"
echo "     gnome-extensions enable $EXTENSION_UUID"
echo "  3. Configure API keys by running:"
echo "     voicify --setup"
echo ""
echo "Keyboard shortcuts:"
echo "  Ctrl+Super+c - Record and auto-paste transcription"
echo "  Ctrl+Super+d - Record and route through plugins"
echo "  Ctrl+Super+v - Real-time transcription"
echo ""
