#!/bin/bash
# Hook: uruchom go vet po edycji plików .go
# Wywoływany jako PostToolUse hook - dane przychodzą przez stdin (JSON)

set -euo pipefail

cd /home/dooshek/projects/voicify/main

# Odczytaj JSON z stdin
INPUT=$(cat)

# Wyciągnij file_path z tool_input
FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Sprawdź czy zmieniony plik to .go
if [ -n "$FILE" ] && echo "$FILE" | grep -q '\.go$'; then
    # Uruchom go vet (cichy jeśli OK)
    if ! go vet ./... 2>/tmp/go-vet-output.txt; then
        echo "go vet znalazł problemy:" >&2
        cat /tmp/go-vet-output.txt >&2
        exit 2
    fi
fi

exit 0
