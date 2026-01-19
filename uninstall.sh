#!/bin/bash
set -e

EXTENSION_UUID="voicify@dooshek.com"
EXTENSION_DIR="$HOME/.local/share/gnome-shell/extensions/$EXTENSION_UUID"
BIN_DIR="$HOME/bin"
DBUS_SERVICE="$HOME/.local/share/dbus-1/services/com.dooshek.voicify.service"

echo "=== Voicify Uninstaller ==="
echo ""

# Disable extension first
echo "Disabling extension..."
gnome-extensions disable "$EXTENSION_UUID" 2>/dev/null || true

# Remove binary
if [ -f "$BIN_DIR/voicify" ]; then
    rm "$BIN_DIR/voicify"
    echo "  Removed: $BIN_DIR/voicify"
fi

# Remove extension
if [ -d "$EXTENSION_DIR" ]; then
    rm -rf "$EXTENSION_DIR"
    echo "  Removed: $EXTENSION_DIR"
fi

# Remove D-Bus service file
if [ -f "$DBUS_SERVICE" ]; then
    rm "$DBUS_SERVICE"
    echo "  Removed: $DBUS_SERVICE"
fi

echo ""
echo "=== Uninstall complete ==="
echo ""
echo "Config files preserved at: ~/.config/voicify/"
echo "To remove config: rm -rf ~/.config/voicify"
echo ""
