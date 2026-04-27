#!/usr/bin/env bash
# Voicify installer - works on Fedora, Ubuntu, Arch, etc. (anything with systemd-user + GNOME)
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DST="$HOME/.local/bin/voicify"
SYSTEMD_DST="$HOME/.config/systemd/user/voicify.service"
EXT_UUID="voicify@dooshek.com"
EXT_DST="$HOME/.local/share/gnome-shell/extensions/$EXT_UUID"
LEGACY_DBUS_ACTIVATION="$HOME/.local/share/dbus-1/services/com.dooshek.voicify.service"
LEGACY_BIN="$HOME/bin/voicify"
CONFIG_DIR="$HOME/.config/voicify"

log() { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mwarn:\033[0m %s\n' "$*"; }
err() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; }

# Preflight
command -v go >/dev/null || { err "go not installed (need 1.23+)"; exit 1; }
command -v gnome-extensions >/dev/null || { err "gnome-extensions CLI not found - is GNOME Shell installed?"; exit 1; }
command -v systemctl >/dev/null || { err "systemctl not found - need systemd"; exit 1; }
command -v glib-compile-schemas >/dev/null || { err "glib-compile-schemas not found - install glib2-devel / libglib2.0-dev"; exit 1; }

# Optional runtime deps - warn but don't fail
command -v ffmpeg >/dev/null || warn "ffmpeg missing (audio conversion). dnf install ffmpeg / apt install ffmpeg"
command -v xclip >/dev/null || command -v wl-copy >/dev/null || warn "no clipboard tool (xclip on X11 / wl-clipboard on Wayland)"

log "Building voicify -> $BIN_DST"
mkdir -p "$(dirname "$BIN_DST")"
( cd "$REPO_DIR" && go build -o "$BIN_DST" ./cmd/voicify/main.go )

# Drop legacy D-Bus activation file - systemd is now the source of truth.
# (Keeping it would race with systemd: D-Bus tries to Exec stale path, fails.)
if [[ -f "$LEGACY_DBUS_ACTIVATION" ]]; then
    log "Removing legacy D-Bus activation file"
    rm -f "$LEGACY_DBUS_ACTIVATION"
fi

# Warn about legacy ~/bin/voicify so user doesn't get confused which binary runs
if [[ -e "$LEGACY_BIN" ]]; then
    warn "legacy $LEGACY_BIN still exists. Safe to remove (canonical is $BIN_DST)."
fi

# PATH check
case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;;
    *) warn "$HOME/.local/bin not in PATH. Add: export PATH=\"\$HOME/.local/bin:\$PATH\"" ;;
esac

log "Installing systemd user unit -> $SYSTEMD_DST"
mkdir -p "$(dirname "$SYSTEMD_DST")"
install -m 0644 "$REPO_DIR/packaging/voicify.service" "$SYSTEMD_DST"

log "Installing GNOME extension -> $EXT_DST"
mkdir -p "$EXT_DST"
rsync -a --delete --exclude='.git*' --exclude='TODO.md' "$REPO_DIR/gnome-extension/" "$EXT_DST/"
glib-compile-schemas "$EXT_DST/schemas/"

mkdir -p "$CONFIG_DIR/recordings"

log "Reloading systemd, enabling daemon"
systemctl --user daemon-reload
# Stop any rogue daemon (legacy launches, zombies) before systemd takes over
pkill -f "voicify --daemon" 2>/dev/null || true
sleep 1
systemctl --user enable --now voicify.service

# Wait for D-Bus name to appear (daemon ready)
for _ in {1..20}; do
    if gdbus call --session --dest com.dooshek.voicify \
        --object-path /com/dooshek/voicify/Recorder \
        --method com.dooshek.voicify.Recorder.GetStatus >/dev/null 2>&1; then
        break
    fi
    sleep 0.25
done

log "Enabling GNOME extension"
gnome-extensions enable "$EXT_UUID" 2>/dev/null || true

# Status summary
DAEMON_STATE=$(systemctl --user is-active voicify.service 2>/dev/null || echo "unknown")
EXT_STATE=$(gnome-extensions info "$EXT_UUID" 2>/dev/null | awk '/^  State:/ {print $2}')

echo
log "Done. daemon=$DAEMON_STATE extension=$EXT_STATE"
echo
echo "Next:"
echo "  - First-time config: $BIN_DST --wizard"
echo "  - Logs: journalctl --user -u voicify.service -f"
echo "  - Restart daemon: systemctl --user restart voicify.service"
if [[ "${XDG_SESSION_TYPE:-}" = "wayland" ]]; then
    echo "  - Wayland: GNOME extension JS changes need logout/login"
fi
