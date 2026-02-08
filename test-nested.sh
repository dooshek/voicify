#!/bin/bash
# Nested GNOME Shell session for extension development
# Starts daemon + gnome-shell in isolated D-Bus session

# Find current Xwayland auth
XAUTH=$(ls -t /run/user/1000/.mutter-Xwaylandauth.* 2>/dev/null | head -1)
if [ -z "$XAUTH" ]; then
    echo "ERROR: No Xwayland auth file found"
    exit 1
fi

echo "Using XAUTHORITY: $XAUTH"
echo "Starting nested GNOME Shell + voicify daemon..."
echo "Close the window to stop."

# Deploy latest extension first
rsync -av --delete --exclude=".git*" --exclude="TODO.md" \
    /home/dooshek/projects/voicify/main/gnome-extension/ \
    ~/.local/share/gnome-shell/extensions/voicify@dooshek.com/
glib-compile-schemas ~/.local/share/gnome-shell/extensions/voicify@dooshek.com/schemas/

XAUTHORITY="$XAUTH" dbus-run-session -- bash -c '
    ~/bin/voicify --daemon --log-level=debug &
    DAEMON_PID=$!
    sleep 2
    gnome-shell --nested --wayland
    kill $DAEMON_PID 2>/dev/null
'
