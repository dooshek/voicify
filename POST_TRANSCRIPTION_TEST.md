# Post-Transcription Mode - Test Flow

## Skróty klawiszowe:

- `CTRL+SUPER+V` - Start/stop realtime recording (natychmiastowa transkrypcja)
- `CTRL+SUPER+C` - Start post-transcription recording
- `CTRL+SUPER+D` - Stop post-transcription recording i przekaż do routera
- `CTRL+SUPER+X` - Cancel bieżącego nagrywania (anuluje bez przetwarzania)

## Zaimplementowane komponenty:

### Backend (Go)
1. ✅ `internal/state/state.go` - dodano DBus server reference
2. ✅ `internal/dbus/interface.xml` - dodano metody i signal RequestPaste
3. ✅ `internal/dbus/server.go`:
   - Pole `postTranscriptionMode bool`
   - Metoda `StartPostTranscriptionRecording()`
   - Metoda `StopPostTranscriptionRecording()`
   - Metoda `EmitRequestPaste(text string)`
   - Zmodyfikowano `stopRecordingAsync()` aby routowało przez router w post-transcription mode
4. ✅ `pkg/pluginapi/paste.go` - nowy plik z funkcją `RequestPaste(text string)`
5. ✅ `internal/plugin/utils.go` - dodano helper `RequestPaste()` z fallbackiem
6. ✅ `cmd/voicify/main.go` - ustawienie DBus server w global state
7. ✅ `internal/plugin/default.go` i `vscode.go` - użycie nowego RequestPaste()

### Frontend (GNOME Extension)
1. ✅ `gnome-extension/extension.js`:
   - Flaga `_isPostTranscriptionMode`
   - Rozszerzony DBus interface o nowe metody i signal
   - Zmodyfikowano `_startPostRecordRecording()` - używa nowej metody DBus
   - Zmodyfikowano `_stopPostRecordRecording()` - używa nowej metody DBus
   - Zmodyfikowano `_onCancelPressed()` - obsługuje post-transcription mode
   - Dodano `_onRequestPaste(text)` - handler dla RequestPaste signal
   - Zmodyfikowano `_onTranscriptionReady()` - nie wkleja w post-transcription mode
   - Resetowanie flagi w wielu miejscach
   - Dodano nowy shortcut `CTRL+SUPER+D` do zatrzymania post-transcription recording

## Post-Transcription Flow:

1. CTRL+SUPER+C → ustawia `_isPostTranscriptionMode = true`, rozpoczyna nagrywanie
2. Mówisz
3. CTRL+SUPER+D → wywołuje `StopPostTranscriptionRecording`
4. Backend transkrybuje i przekazuje do routera (BEZ kopiowania do clipboard)
5. Router wykonuje matching pluginy
6. Plugin może wywołać `pluginapi.RequestPaste(text)`
7. Extension odbiera signal i wkleja tekst

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

### Krok 3: Testuj post-transcription mode
1. Naciśnij `CTRL+SUPER+C` - rozpoczyna post-transcription recording
2. Powiedz coś (np. "test wiadomości")
3. Naciśnij `CTRL+SUPER+D` - zatrzymuje i routuje przez router
4. Backend:
   - Transkrybuje audio
   - Przekazuje do routera (NIE kopiuje do clipboard)
   - Router wykonuje matching pluginy
   - Plugin może wywołać `pluginapi.RequestPaste(text)`
5. Extension:
   - Odbiera signal `RequestPaste` z tekstem
   - Kopiuje do clipboard
   - Wykonuje auto-paste

### Krok 4: Weryfikuj logi backend
```bash
journalctl --user -u gnome-shell -f | grep -E "(Post-transcription|RequestPaste)"
```

Powinny pokazać:
- `D-Bus: StartPostTranscriptionRecording called`
- `D-Bus: StopPostTranscriptionRecording called`
- `D-Bus: Post-transcription received: ...`
- `Router: Starting routing for transcription: ...`
- `D-Bus: Plugin requesting paste of text: ...`

### Krok 5: Weryfikuj logi extension
```bash
journalctl --user -u gnome-shell -f | grep voicify
```

Powinny pokazać:
- `POST-RECORD SHORTCUT PRESSED!`
- `post-transcription mode: true`
- `CANCEL PRESSED! ... post-transcription mode: true`
- `Post-transcription mode - stopping and routing`
- `RequestPaste signal received from plugin`

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

