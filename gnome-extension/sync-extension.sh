#!/bin/bash
# Sync extension to GNOME Shell extensions directory

REPO_DIR="."
GNOME_EXT_DIR="$HOME/.local/share/gnome-shell/extensions/voicify@dooshek.com"

# Record start time for log filtering
START_TIME=$(date +"%Y-%m-%d %H:%M:%S")

echo "🔄 Syncing extension from repo to GNOME..."
rsync -av --delete \
    --exclude="sync-extension.sh" \
    --exclude=".git*" \
    --exclude=".vscode/" \
    --exclude=".cursor/" \
    --exclude="~/" \
    "$REPO_DIR/" "$GNOME_EXT_DIR/"

echo "📁 Files synchronized:"
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

echo -e "\n💡 TIP: After restart, test Ctrl+Win+V or click microphone icon"
echo "     Watch for Voicify DEBUG notifications on screen"
