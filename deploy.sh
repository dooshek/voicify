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

echo "=== Bumping interface version ==="
# Hash the D-Bus interface XML literal in extension.js. Both daemon (Go const)
# and extension (JS const) get this value baked in. At enable() the extension
# calls GetInterfaceVersion on the daemon and warns the user if they diverge -
# this catches the SyncThing case where extension JS is copied to a host whose
# daemon binary was not rebuilt.
WAS_DIRTY=$(git status --porcelain 2>/dev/null | wc -l)
IFACE_HASH=$(awk '/const VoicifyDBusInterface = `/,/`;/' gnome-extension/extension.js | sha256sum | cut -c1-12)
sed -i "s/^const InterfaceVersion = \".*\"$/const InterfaceVersion = \"$IFACE_HASH\"/" internal/dbus/version.go
sed -i "s/^const EXPECTED_INTERFACE_VERSION = '.*';$/const EXPECTED_INTERFACE_VERSION = '$IFACE_HASH';/" gnome-extension/extension.js
echo "Interface version: $IFACE_HASH"

if ! git diff --quiet internal/dbus/version.go gnome-extension/extension.js 2>/dev/null; then
    # Auto-commit only when the tree was clean before the bump. If the user
    # had other in-progress edits (including to extension.js itself), leave
    # the bump unstaged so they can review and fold it into their own commit.
    if [ "$WAS_DIRTY" -eq 0 ]; then
        git add internal/dbus/version.go gnome-extension/extension.js
        git commit -m "chore(dbus): bump interface version to $IFACE_HASH" >/dev/null
        echo "Auto-committed version bump"
    else
        echo "Working tree had pending changes - version bump left unstaged"
    fi
fi

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
