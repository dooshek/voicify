#!/bin/bash
# Dev deploy: build Go -> ~/.local/bin, refresh systemd unit + extension, restart daemon.
# For first-time / end-user setup use install.sh instead.
set -e

cd /home/dooshek/projects/voicify/main

BIN_DST="$HOME/.local/bin/voicify"
SYSTEMD_DST="$HOME/.config/systemd/user/voicify.service"
EXT_UUID="voicify@dooshek.com"
EXT_DST="$HOME/.local/share/gnome-shell/extensions/$EXT_UUID"
LEGACY_DBUS_ACTIVATION="$HOME/.local/share/dbus-1/services/com.dooshek.voicify.service"

echo "=== Building Go ==="
mkdir -p "$(dirname "$BIN_DST")"
go build -o "$BIN_DST" ./cmd/voicify/main.go

echo "=== Deploying extension ==="
mkdir -p "$EXT_DST"
rsync -a --delete --exclude=".git*" --exclude="TODO.md" gnome-extension/ "$EXT_DST/"
glib-compile-schemas "$EXT_DST/schemas/"

echo "=== Refreshing systemd unit ==="
mkdir -p "$(dirname "$SYSTEMD_DST")"
install -m 0644 packaging/voicify.service "$SYSTEMD_DST"
# Drop legacy D-Bus activation file (would race with systemd if stale path)
rm -f "$LEGACY_DBUS_ACTIVATION"
systemctl --user daemon-reload

echo "=== Restarting daemon ==="
# Kill any rogue daemon (legacy nohup launch from old deploy.sh) before
# letting systemd take over - prevents "address already in use" / D-Bus
# name conflicts during the brief overlap.
pkill -f "voicify --daemon" 2>/dev/null || true
sleep 1
systemctl --user enable --now voicify.service >/dev/null 2>&1 || true
systemctl --user restart voicify.service

if [ "$XDG_SESSION_TYPE" = "x11" ]; then
    echo "=== Restarting GNOME Shell (X11) ==="
    gnome-shell --replace &>/dev/null &
    sleep 4
    gnome-extensions enable "$EXT_UUID" 2>/dev/null || true
else
    echo "=== Restarting extension (Wayland - disable/enable) ==="
    gnome-extensions disable "$EXT_UUID" 2>/dev/null || true
    sleep 1
    gnome-extensions enable "$EXT_UUID"
fi

# Wait for D-Bus name to appear
for _ in {1..20}; do
    if gdbus call --session --dest com.dooshek.voicify \
        --object-path /com/dooshek/voicify/Recorder \
        --method com.dooshek.voicify.Recorder.GetStatus >/dev/null 2>&1; then
        break
    fi
    sleep 0.25
done

EXT_STATE=$(gnome-extensions info "$EXT_UUID" 2>/dev/null | awk '/^  State:/ {print $2}')
DAEMON_STATE=$(systemctl --user is-active voicify.service 2>/dev/null || echo "unknown")

# Sync handsfree config from GSettings to fresh daemon (extension JS can't on Wayland reload)
SCHEMA_DIR="$EXT_DST/schemas/"
HF_ENABLED=$(GSETTINGS_SCHEMA_DIR="$SCHEMA_DIR" gsettings get org.gnome.shell.extensions.voicify handsfree-enabled 2>/dev/null)
HF_DEVICE=$(GSETTINGS_SCHEMA_DIR="$SCHEMA_DIR" gsettings get org.gnome.shell.extensions.voicify handsfree-device 2>/dev/null | tr -d "'")
if [ "$HF_ENABLED" = "true" ] && [ -n "$HF_DEVICE" ]; then
    gdbus call --session --dest com.dooshek.voicify \
        --object-path /com/dooshek/voicify/Recorder \
        --method com.dooshek.voicify.Recorder.SetHandsfreeConfig true "$HF_DEVICE" >/dev/null 2>&1
    echo "Handsfree config synced: $HF_DEVICE"
fi

echo ""
echo "=== Done: Extension=$EXT_STATE Daemon=$DAEMON_STATE ==="
if [ "$XDG_SESSION_TYPE" = "wayland" ]; then
    echo "UWAGA: Na Wayland zmiany JS wymagają logout/login!"
fi
