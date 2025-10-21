# Post-Transcription Mode - Test Flow

## Skróty klawiszowe:

- `CTRL+SUPER+V` - Toggle realtime recording (natychmiastowa transkrypcja)
- `CTRL+SUPER+C` - Toggle post-transcription recording z auto-paste
- `CTRL+SUPER+D` - Toggle post-transcription recording z przekazaniem do routera

Każdy skrót działa jako toggle (start/stop) dla swojego trybu nagrywania.

## Zaimplementowane komponenty:

### Backend (Go)
1. ✅ `internal/state/state.go` - dodano DBus server reference
2. ✅ `internal/dbus/interface.xml` - dodano metody i signal RequestPaste
3. ✅ `internal/dbus/server.go`:
   - Pola `postTranscriptionRouterMode bool` i `postTranscriptionAutoPaste bool`
   - Metoda `TogglePostTranscriptionAutoPaste()` - toggle auto-paste mode
   - Metoda `TogglePostTranscriptionRouter()` - toggle router mode
   - Metoda `EmitRequestPaste(text string)`
   - Funkcje `stopPostTranscriptionAutoPasteAsync()` i `stopPostTranscriptionRouterAsync()`
4. ✅ `pkg/pluginapi/paste.go` - nowy plik z funkcją `RequestPaste(text string)`
5. ✅ `internal/plugin/utils.go` - dodano helper `RequestPaste()` z fallbackiem
6. ✅ `cmd/voicify/main.go` - ustawienie DBus server w global state
7. ✅ `internal/plugin/default.go` i `vscode.go` - użycie nowego RequestPaste()

### Frontend (GNOME Extension)
1. ✅ `gnome-extension/extension.js`:
   - Flagi `_isRealtimeMode`, `_isPostAutoPaste`, `_isPostRouter`
   - Rozszerzony DBus interface o nowe metody toggle
   - Skróty:
     - `CTRL+SUPER+V` - toggle realtime recording
     - `CTRL+SUPER+C` - toggle post-transcription auto-paste
     - `CTRL+SUPER+D` - toggle post-transcription router
   - Usunięto `CTRL+SUPER+X` cancel shortcut (niepotrzebny z toggle)
   - Metody start/stop dla każdego trybu
   - `_onRequestPaste(text)` - handler dla RequestPaste signal
   - `_onTranscriptionReady()` - różne zachowanie dla auto-paste vs router mode

## Post-Transcription Flow:

### Auto-Paste Mode (CTRL+SUPER+C):
1. CTRL+SUPER+C → rozpoczyna nagrywanie w auto-paste mode
2. Mówisz
3. CTRL+SUPER+C → zatrzymuje nagrywanie
4. Backend transkrybuje i kopiuje do clipboard
5. Extension odbiera signal i automatycznie wkleja tekst

### Router Mode (CTRL+SUPER+D):
1. CTRL+SUPER+D → rozpoczyna nagrywanie w router mode
2. Mówisz
3. CTRL+SUPER+D → zatrzymuje nagrywanie
4. Backend transkrybuje i przekazuje do routera (BEZ kopiowania do clipboard)
5. Router wykonuje matching pluginy
6. Plugin może wywołać `pluginapi.RequestPaste(text)`
7. Extension odbiera signal i wkleja tekst

### Realtime Mode (CTRL+SUPER+V):
1. CTRL+SUPER+V → rozpoczyna realtime recording
2. Mówisz, transcription pojawia się na bieżąco
3. CTRL+SUPER+V → zatrzymuje recording i routuje przez router

## Test flow:

### Krok 1: Uruchom daemon
```bash
cd /home/dooshek/projects/voicify/main
./bin/voicify --daemon
```

### Krok 2: Restart GNOME Shell extension
- X11: `Alt+F2` → `r` → Enter
- Wayland: Logout/login
- Sprawdź logi: `journalctl --user -u gnome-shell -f`

### Krok 3: Testuj auto-paste mode
1. Naciśnij `CTRL+SUPER+C` - rozpoczyna recording
2. Powiedz coś (np. "test wiadomości")
3. Naciśnij `CTRL+SUPER+C` - zatrzymuje recording
4. Backend transkrybuje i kopiuje do clipboard
5. Extension automatycznie wkleja tekst

### Krok 4: Testuj router mode
1. Naciśnij `CTRL+SUPER+D` - rozpoczyna recording
2. Powiedz coś (np. "dodaj task test")
3. Naciśnij `CTRL+SUPER+D` - zatrzymuje recording
4. Backend:
   - Transkrybuje audio
   - Przekazuje do routera (NIE kopiuje do clipboard)
   - Router wykonuje matching pluginy
   - Plugin może wywołać `pluginapi.RequestPaste(text)`
5. Extension:
   - Odbiera signal `RequestPaste` z tekstem (jeśli plugin to wywołał)
   - Kopiuje do clipboard
   - Wykonuje auto-paste

### Krok 5: Weryfikuj logi backend
```bash
journalctl --user -u gnome-shell -f | grep -E "(Post-transcription|RequestPaste|TogglePost)"
```

Powinny pokazać dla auto-paste:
- `D-Bus: TogglePostTranscriptionAutoPaste called`
- `D-Bus: Stopping post-transcription auto-paste recording`
- `D-Bus: Post-transcription auto-paste received: ...`

Powinny pokazać dla router:
- `D-Bus: TogglePostTranscriptionRouter called`
- `D-Bus: Stopping post-transcription router recording`
- `D-Bus: Post-transcription router received: ...`
- `Router: Starting routing for transcription: ...`
- `D-Bus: Plugin requesting paste of text: ...`

### Krok 6: Weryfikuj logi extension
```bash
journalctl --user -u gnome-shell -f | grep voicify
```

Powinny pokazać:
- `POST AUTO-PASTE SHORTCUT PRESSED!` lub `POST ROUTER SHORTCUT PRESSED!`
- `Post-autopaste mode - performing auto-paste` lub `Post-router mode - waiting for plugin RequestPaste signal`
- `RequestPaste signal received from plugin` (tylko w router mode)

## Przykład użycia w pluginie:

```go
package plugin

import (
    "github.com/dooshek/voicify/pkg/pluginapi"
)

func (a *MyAction) Execute(transcription string) error {
    // Przetwarzaj transcription...
    modifiedText := processText(transcription)

    // Wyślij do extension żeby wkleił
    return pluginapi.RequestPaste(modifiedText)
}
```

## Fallback behavior:
- Jeśli DBus nie jest dostępny, `plugin.RequestPaste()` automatycznie używa `clipboard.PasteWithReturn()`
- To oznacza że pluginy działają zarówno w daemon mode jak i keyboard monitor mode

