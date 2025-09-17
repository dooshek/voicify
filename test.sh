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
    if [ $? -eq 0 ]; then
        echo "✅ Build successful, restarting Voicify service..."
        systemctl --user restart voicify
        sleep 2
        echo "✅ Service restarted"
    else
        echo "❌ Build failed!"
        exit 1
    fi
else
    echo "✅ No Go backend changes detected"
fi

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

echo -e "\n🔄 Reloading extension... Press ALT+F2 and type r"
read -n 1 -s -r -p "Press any key to continue"
gnome-extensions disable voicify@dooshek.com 2>/dev/null || true
sleep 1
gnome-extensions enable voicify@dooshek.com

echo -e "\n📊 Extension status:"
gnome-extensions info voicify@dooshek.com | grep -E "(State|Enabled)"

echo -e "\n🔍 Checking for errors in logs..."
ERROR_LOG=$(journalctl --user --since "2 minutes ago" -p err -p warning | grep -A2 -B2 "voicify@dooshek.com\|JS ERROR.*voicify\|extension.*error" | tail -10)

if [ ! -z "$ERROR_LOG" ]; then
    echo "❌ ERRORS FOUND:"
    echo "$ERROR_LOG"
else
    echo "✅ No errors found in logs"
fi

echo -e "\n📋 ALL LOGS SINCE SCRIPT START ($START_TIME):"
echo "================================================"
ALL_LOGS=$(journalctl --user --since "$START_TIME" | grep -E "(🔥|voicify|Voicify|extension.*enabled|extension.*disabled|JS ERROR|JS WARNING)" | tail -20)

if [ ! -z "$ALL_LOGS" ]; then
    echo "$ALL_LOGS"
else
    echo "❌ No extension logs found - check if extension is really working"
fi

echo -e "\n🔍 Checking Voicify backend logs..."
if [ -f ~/.config/voicify/voicify.log ]; then
    echo "⏳ Waiting 5 seconds for processing to complete..."
    sleep 5
    echo "📄 Recent Voicify logs (last 20 lines):"
    echo "================================================"
    tail -20 ~/.config/voicify/voicify.log
    echo ""
    echo "💡 To watch logs live: tail -f ~/.config/voicify/voicify.log"
    echo "💡 Service status: systemctl --user status voicify"
else
    echo "❌ Voicify log file not found at ~/.config/voicify/voicify.log"
    echo "   Make sure Voicify service is running: systemctl --user status voicify"
fi

echo -e "\n💡 TIP: After restart, test Ctrl+Win+V or click microphone icon"
echo "     Watch for Voicify DEBUG notifications on screen"
