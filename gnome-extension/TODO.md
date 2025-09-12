# Voicify GNOME Extension - TODO & Implementation Plan

## 🎯 Project Status Overview

### ✅ COMPLETED - GNOME Extension Frontend
- [x] **Basic Extension Structure** - metadata.json, extension.js, stylesheet.css, prefs.js
- [x] **Global Keyboard Shortcuts** - `<Ctrl><Super>v` with configurable GSettings
- [x] **Panel Icon Integration** - System tray icon with state-based styling
- [x] **Wave Equalizer Visualization** - 10 animated bars with proper positioning
- [x] **State Management** - IDLE → RECORDING → UPLOADING → FINISHED states
- [x] **Animation System** - JavaScript-based animations (recording, upload wave, finished)
- [x] **Text Injection** - Clipboard + Ctrl+V simulation for X11
- [x] **Preferences Panel** - Configurable keyboard shortcuts via GNOME settings
- [x] **Development Workflow** - `sync-extension.sh` for deployment and debugging
- [x] **Error Handling** - Proper cleanup in disable(), timer management, logging

### ✅ COMPLETED - D-Bus Integration Layer
- [x] **D-Bus Communication** - Extension connected with Go daemon
  - [x] Define D-Bus interface (`com.dooshek.voicify.Recorder`)
  - [x] Implement D-Bus proxy in extension.js with VoicifyProxy wrapper
  - [x] Add D-Bus server to Go daemon (`internal/dbus/server.go`)
  - [x] Test bidirectional communication (ToggleRecording, GetStatus work)

### ✅ COMPLETED - Go Daemon D-Bus Integration  
- [x] **Add D-Bus Server** - Daemon mode preserves keyboard monitoring
  - [x] Create D-Bus service interface with godbus/v5
  - [x] Implement ToggleRecording method (start/stop toggle)
  - [x] Add TranscriptionReady and RecordingError signals
  - [x] Add `--daemon` flag to main.go (preserves original functionality)
  - [x] Test service registration (`com.dooshek.voicify` registered)

### ⚠️ PRESERVED - Legacy Components (NOT REMOVED)
- ✅ **Keyboard monitoring code kept** - `voicify` works as before
- ✅ **Clipboard operations kept** - all original functionality preserved  
- ✅ **Dual mode operation**: `voicify` = keyboard, `voicify --daemon` = D-Bus

### 🔄 IN PROGRESS - Final Polish & Bug Fixes
- [x] **Basic D-Bus Integration Testing**
  - [x] Test keyboard shortcut → D-Bus call (✅ working)
  - [x] Test ToggleRecording method calls (✅ working)
  - [x] Test state transitions IDLE → RECORDING → UPLOADING (✅ working)
  - [ ] **Fix D-Bus signal reception** - TranscriptionReady signal not received
  - [ ] **Fix UI errors** - `this._waveBars is null` errors in console
  
- [ ] **End-to-End Flow Testing**  
  - [x] Test daemon audio recording (✅ works with D-Bus calls)
  - [ ] Test transcription → D-Bus signal → extension text injection
  - [ ] Verify all animation states work correctly during real flow
  - [ ] Test error scenarios (no audio, API failure, daemon offline)

- [x] **Configuration & Polish**
  - [x] Extension enable/disable lifecycle works
  - [x] GSettings work correctly (shortcuts configurable)
  - [x] Proper cleanup implemented  
  - [ ] Test on fresh GNOME Shell restart with daemon

## 🔧 Technical Architecture

### Current Flow (IMPLEMENTED ✅):
```
User Presses Ctrl+Win+V → Extension receives global shortcut ✅
                       → Extension calls ToggleRecording via D-Bus ✅  
                       → Go daemon starts audio recording ✅
                       → Extension shows recording animation ✅
                       → User presses Ctrl+Win+V again ✅
                       → Extension calls ToggleRecording to stop ✅
                       → Go daemon stops, processes transcription ✅  
                       → Extension shows upload animation ✅
                       → [❌ MISSING: TranscriptionReady signal to extension]
                       → [❌ MISSING: Extension text injection]
```

### Target Flow (90% COMPLETE):
```
User Presses Ctrl+Win+V → Extension receives global shortcut ✅
                       → Extension calls Go daemon via D-Bus ✅
                       → Go daemon starts OpenAI recording ✅  
                       → Daemon emits RecordingStarted signal ✅
                       → Extension shows recording animation ✅
                       → User stops recording (Ctrl+Win+V) ✅
                       → Go daemon receives transcription ✅
                       → Daemon emits TranscriptionReady signal ❌ (needs fix)
                       → Extension receives signal & injects text ❌ (needs impl)
                       → Extension shows finished animation ✅
```

## 🚀 Implementation Priority

### Phase 1: D-Bus Integration (Current Priority)
1. **Design D-Bus Interface**
   ```xml
   <interface name="com.dooshek.voicify">
       <method name="StartRecording" />
       <method name="StopRecording">
           <arg name="transcription" type="s" direction="out" />
       </method>
       <signal name="TranscriptionReady">
           <arg name="text" type="s" />
       </signal>
       <signal name="RecordingError">
           <arg name="error" type="s" />
       </signal>
   </interface>
   ```

2. **Extension D-Bus Client** (`extension.js`)
   - Replace mock `_startRecording()` with real D-Bus call
   - Listen for TranscriptionReady signal
   - Handle RecordingError signal

3. **Go Daemon D-Bus Server** (`main.go`)
   - Replace keyboard monitoring with D-Bus service
   - Implement recording methods
   - Emit signals for transcription results

### Phase 2: Testing & Polish
1. **Integration Testing**
   - Full recording workflow
   - Error handling scenarios
   - Extension lifecycle (enable/disable/restart)

2. **Performance & Stability**
   - Memory leak testing
   - Timer cleanup verification
   - GNOME Shell restart resilience

## 📁 File Structure Status

```
gnome-extension/
├── extension.js          ✅ Complete - D-Bus integration + UI animations
├── metadata.json         ✅ Complete - extension metadata  
├── stylesheet.css        ✅ Complete - UI styling
├── prefs.js             ✅ Complete - preferences panel
├── schemas/
│   ├── *.gschema.xml    ✅ Complete - GSettings schema
│   └── gschemas.compiled ✅ Auto-generated
├── sync-extension.sh    ✅ Complete - deployment & debugging
└── TODO.md              ✅ Updated - current status

../main/ (Go daemon)  
├── cmd/voicify/main.go           ✅ Updated - added --daemon flag
├── internal/dbus/
│   ├── interface.xml             ✅ New - D-Bus interface definition
│   └── server.go                 ✅ New - D-Bus server implementation
├── internal/keyboard/monitor.go  ✅ Kept - original functionality preserved
├── internal/clipboard/clipboard.go ✅ Kept - original functionality preserved  
├── internal/audio/              ✅ Unchanged - works with D-Bus
├── internal/transcriptionrouter/ ✅ Unchanged - works with D-Bus
├── internal/transcriber/        ✅ Unchanged - works with D-Bus
└── internal/config/             ✅ Unchanged - works with both modes
```

**Key Changes:**
- ✅ Added D-Bus server as new option, NOT replacement
- ✅ `voicify` = original keyboard monitoring (unchanged)
- ✅ `voicify --daemon` = new D-Bus mode for GNOME extension
- ✅ All original functionality preserved and working

## 🐛 Critical Notes & Lessons Learned

### GNOME Extension Development
- **Always restart GNOME Shell** after code changes (`Alt+F2` → `r`)
- **Use `journalctl --user -u gnome-shell -f`** for real-time debugging
- **JavaScript animations only** - CSS animations don't work reliably
- **Proper timer cleanup** is critical - use `GLib.Source.remove()`
- **State management** - use enum pattern for complex UI states
- **Never use `setTimeout()`** - always `GLib.timeout_add()`

### Specific API Gotchas
- `PanelMenu.Button` has NO `get_child()` method - store icon reference
- `grab_accelerator()` needs `Meta.KeyBindingFlags.NONE` as 2nd parameter
- `Clutter` imports forbidden in `prefs.js`
- Widget animations: use `scale_y` with `set_pivot_point(0.5, 1.0)`

### Development Workflow
- Use `rsync --delete` for exact deployment synchronization
- Include schema compilation in deployment script
- Filter logs by timestamp and extension name
- Test extension lifecycle (enable/disable) thoroughly

---

## 🎯 Next Action Items

**✅ COMPLETED (This Session):**
1. ✅ Created D-Bus interface definition (`internal/dbus/interface.xml`)
2. ✅ Implemented D-Bus proxy in extension.js with VoicifyProxy  
3. ✅ Added D-Bus server to Go daemon (`internal/dbus/server.go`)
4. ✅ Added --daemon flag preserving original voicify functionality
5. ✅ Tested D-Bus methods (ToggleRecording, GetStatus work perfectly)

**🔄 IMMEDIATE (Next Session):**
1. **Debug D-Bus signals** - TranscriptionReady signal not reaching extension
2. **Fix extension UI bugs** - resolve `this._waveBars is null` errors  
3. **Implement text injection** - handle TranscriptionReady signal in extension
4. **Test full end-to-end flow** - recording → transcription → text injection

**SHORT TERM:**
1. **Error handling improvements** - daemon offline, API failures, audio errors
2. **Performance testing** - memory leaks, extension lifecycle, GNOME restarts  
3. **User experience polish** - better error messages, smoother animations

**LONG TERM:**
1. **Documentation** - setup guide for users wanting GNOME extension integration
2. **Deployment improvements** - maybe .deb package with systemd service for daemon  
3. **Feature additions** - configurable text processing, custom commands

---

## 🎉 **MAJOR MILESTONE ACHIEVED** 
**D-Bus integration is 90% complete!** Voicify now has dual-mode operation:
- `voicify` - original keyboard monitoring (unchanged)
- `voicify --daemon` - new D-Bus service for GNOME extension integration

**Communication protocol works:** Extension ↔ Daemon via D-Bus calls

