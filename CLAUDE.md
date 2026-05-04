# Voicify

Voice-to-text system for GNOME: **Go backend** + **GNOME Shell extension (JavaScript)**.

Shortcut -> Extension -> D-Bus -> Go backend -> OpenAI Whisper -> text -> clipboard/paste

## Kluczowe ścieżki

| Ścieżka | Opis |
|---------|------|
| `cmd/voicify/main.go` | Entry point, CLI flags (--daemon, --wizard, --log-level) |
| `internal/` | Cały Go backend (audio, dbus, llm, plugin, tts, ...) |
| `pkg/pluginapi/` | Publiczny interfejs pluginów (VoicifyPlugin, PluginAction) |
| `gnome-extension/` | GNOME Shell extension (JS, GJS, St, Clutter) |
| `prompts/` | Szablony promptów |
| `bin/voicify` | Skompilowany binary |

## Build & Run

```bash
# Kompilacja
go build -o bin/voicify ./cmd/voicify/main.go

# Uruchomienie (daemon mode - z GNOME extension)
./bin/voicify --daemon

# Uruchomienie (keyboard mode - standalone)
./bin/voicify

# Testy
go vet ./...
staticcheck ./...

# Deploy extension
rsync -av --delete --exclude=".git*" gnome-extension/ ~/.local/share/gnome-shell/extensions/voicify@dooshek.com/
glib-compile-schemas ~/.local/share/gnome-shell/extensions/voicify@dooshek.com/schemas/
gnome-extensions disable voicify@dooshek.com && sleep 1 && gnome-extensions enable voicify@dooshek.com

# Logi GNOME Shell
journalctl --user -u gnome-shell -f
```

## Krytyczne konwencje

### Logger (Go) - ZAWSZE sprawdzaj sygnatury!

```go
logger.Error(msg string, err error)                       // WYMAGA error jako 2. arg
logger.Errorf(format string, err error, v ...interface{})  // error jest 2. arg, NIE ostatni!
logger.Debugf(format string, v ...interface{})             // BEZ error
logger.Infof(format string, v ...interface{})              // BEZ error
```

### GNOME Extension - krytyczne zasady

- **NIE** używaj `setTimeout()` -> używaj `GLib.timeout_add()`
- **NIE** twórz obiektów w `constructor()` -> tylko w `enable()`
- **NIE** importuj `Clutter` w `prefs.js`
- **ZAWSZE** sprzątaj w `disable()`: `obj?.destroy(); obj = null;`
- **ZAWSZE** usuwaj timery: `GLib.Source.remove(timerId)`
- Animacje JavaScript (GLib.timeout_add), **NIE** CSS animations

### Go

- Formatter: `goimports` (auto po edycji via hook)
- Linter: `staticcheck`
- Error handling: zawsze propaguj, loguj na granicy

### Git

- Conventional commits: `type(scope): description`
- **NIE** commituj automatycznie - przygotuj komendy do review
- Wyjątek: `deploy.sh` sam bumpuje + commituje wersję D-Bus interface (tylko gdy tree clean)

### D-Bus interface version compat

Daemon (Go) i extension (JS) trzymają identyczny hash XML interface. Ważne bo SyncThing kopiuje extension.js out-of-band i daemon na drugim hoście może być starszy.

- `internal/dbus/version.go` - `const InterfaceVersion`
- `gnome-extension/extension.js` - `const EXPECTED_INTERFACE_VERSION`
- `deploy.sh` liczy SHA256 z `VoicifyDBusInterface` XML literal, sed-uje do obu plików, auto-commit gdy tree clean
- Extension przy `enable()` woła `GetInterfaceVersion` na daemon. Mismatch → notyfikacja systemowa + czerwona ikona w panelu + "⚠ Version mismatch" item w menu
- Po edycji XML interface: `bash deploy.sh` zbumpuje hash, na drugim hoście (SyncThing) extension wykryje rozjazd przy następnym logout/login

## Skróty klawiszowe

| Skrót | Tryb |
|-------|------|
| `Ctrl+Super+V` | Realtime (stream) |
| `Ctrl+Super+C` | Post-processing z auto-paste |
| `Ctrl+Super+D` | Post-processing z routerem |

## Szczegółowe reguły

Patrz `.claude/rules/` - ładowane automatycznie wg kontekstu edytowanego pliku.
