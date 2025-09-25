#!/bin/bash
# Test script for Voicify - rebuilds backend, syncs extension, and shows logs

REPO_DIR="."
GNOME_EXT_DIR="$HOME/.local/share/gnome-shell/extensions/voicify@dooshek.com"

# Record start time for log filtering
START_TIME=$(date +"%Y-%m-%d %H:%M:%S")

echo "🔄 Testing Voicify - checking for changes and rebuilding..."

echo -e "\n🔄 Checking for Go backend changes..."
# Check if Go files are newer than binary
if [ ! -f "bin/voicify" ] || [ "cmd/voicify/main.go" -nt "bin/voicify" ] || find internal/ -name "*.go" -newer "bin/voicify" | grep -q .; then
    echo "🔨 Go backend changes detected, rebuilding..."
    go build -o bin/voicify ./cmd/voicify
else
    echo "✅ No Go backend changes detected"
fi

systemctl --user start voicify
systemctl --user restart voicify

echo -e "\n🔄 Syncing extension from repo to GNOME..."
rsync -av --delete \
    --exclude="test.sh" \
    --exclude=".git*" \
    --exclude=".vscode/" \
    --exclude=".cursor/" \
    --exclude="~/" \
    --exclude="bin/" \
    --exclude="internal/" \
    --exclude="cmd/" \
    --exclude="pkg/" \
    --exclude="go.mod" \
    --exclude="go.sum" \
    --exclude="README.md" \
    --exclude="docs/" \
    --exclude="prompts/" \
    gnome-extension/ "$GNOME_EXT_DIR/"

echo "📁 Extension files synchronized:"
ls -la "$GNOME_EXT_DIR/"

echo -e "\n🔄 Reloading extension..."
echo "Waiting 10 seconds for extension to reload..."
gnome-extensions disable voicify@dooshek.com 2>/dev/null || true
sleep 3
gnome-extensions enable voicify@dooshek.com

echo "================================================"
echo "🎙️ Now start recording and test the flow!"
echo "================================================"

if [ -f ~/.config/voicify/voicify.log ]; then
    echo "⏳ Waiting 15 seconds for logs to be written..."
    sleep 15
    echo "📄 Recent Voicify logs (last 20 lines):"
    echo "================================================"
    tail -100 ~/.config/voicify/voicify.log
    echo "================================================"
    echo "🔍 Checking extension logs..."
    echo "================================================"
    journalctl --user --since "2 minutes ago" -p err -p warning | grep -A2 -B2 "voicify@dooshek.com\|JS ERROR.*voicify\|extension.*error" | tail -10
    echo "================================================"
    echo ""
    echo "💡 To watch logs live: tail -f ~/.config/voicify/voicify.log"
    echo "💡 Service status: systemctl --user status voicify"
else
    echo "❌ Voicify log file not found at ~/.config/voicify/voicify.log"
    echo "   Make sure Voicify service is running: systemctl --user status voicify"
fi

echo -e "\n💡 TIP: After restart, test Ctrl+Win+V or click microphone icon"
echo "     Watch for Voicify DEBUG notifications on screen"
