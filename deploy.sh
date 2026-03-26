#!/bin/bash
# Full deploy: build Go + deploy extension + restart only voicify + restart daemon
set -e

cd /home/dooshek/projects/voicify/main

echo "=== Building Go ==="
go build -o bin/voicify ./cmd/voicify/main.go

echo "=== Deploying extension ==="
rsync -a --delete --exclude=".git*" --exclude="TODO.md" \
    gnome-extension/ ~/.local/share/gnome-shell/extensions/voicify@dooshek.com/
glib-compile-schemas ~/.local/share/gnome-shell/extensions/voicify@dooshek.com/schemas/

echo "=== Killing daemon ==="
pkill -f "voicify --daemon" 2>/dev/null || true
sleep 1

if [ "$XDG_SESSION_TYPE" = "x11" ]; then
    echo "=== Restarting GNOME Shell (X11) ==="
    gnome-shell --replace &>/dev/null &
    sleep 4
    gnome-extensions enable voicify@dooshek.com 2>/dev/null || true
else
    echo "=== Restarting extension (Wayland - disable/enable) ==="
    gnome-extensions disable voicify@dooshek.com 2>/dev/null || true
    sleep 1
    gnome-extensions enable voicify@dooshek.com
fi

echo "=== Starting daemon ==="
nohup /home/dooshek/bin/voicify --daemon --log-level=debug \
    --log-filename=/home/dooshek/.config/voicify/voicify.log >/dev/null 2>&1 &
sleep 2

STATUS=$(gnome-extensions info voicify@dooshek.com 2>/dev/null | grep State | awk '{print $2}')
DAEMON=$(pgrep -f "voicify --daemon" >/dev/null 2>&1 && echo "running" || echo "dead")

# Sync handsfree config from GSettings to new daemon (Wayland: old extension JS can't send it)
SCHEMA_DIR="$HOME/.local/share/gnome-shell/extensions/voicify@dooshek.com/schemas/"
HF_ENABLED=$(GSETTINGS_SCHEMA_DIR="$SCHEMA_DIR" gsettings get org.gnome.shell.extensions.voicify handsfree-enabled 2>/dev/null)
HF_DEVICE=$(GSETTINGS_SCHEMA_DIR="$SCHEMA_DIR" gsettings get org.gnome.shell.extensions.voicify handsfree-device 2>/dev/null | tr -d "'")
if [ "$HF_ENABLED" = "true" ] && [ -n "$HF_DEVICE" ]; then
    gdbus call --session --dest com.dooshek.voicify \
        --object-path /com/dooshek/voicify/Recorder \
        --method com.dooshek.voicify.Recorder.SetHandsfreeConfig true "$HF_DEVICE" >/dev/null 2>&1
    echo "Handsfree config synced: $HF_DEVICE"
fi

echo ""
echo "=== Done: Extension=$STATUS Daemon=$DAEMON ==="
[ "$XDG_SESSION_TYPE" = "wayland" ] && echo "UWAGA: Na Wayland zmiany JS wymagają logout/login!"
