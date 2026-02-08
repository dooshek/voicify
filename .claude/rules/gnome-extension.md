---
paths:
  - "gnome-extension/**"
---

# GNOME Shell Extension - Voicify

UUID: `voicify@dooshek.com`
Shell versions: 48, 49
Deploy path: `~/.local/share/gnome-shell/extensions/voicify@dooshek.com/`

## Krytyczne zasady

### NIGDY nie rób:
- `setTimeout()` / `setInterval()` -> użyj `GLib.timeout_add()`
- Tworzenie obiektów w `constructor()` -> tylko w `enable()`
- Import `Clutter` w `prefs.js` (inny kontekst!)
- `log()` -> użyj `console.debug()`
- `PanelMenu.Button.get_child()` -> nie istnieje, przechowuj referencję do ikony
- `grab_accelerator()` z 1 argumentem -> potrzebuje `Meta.KeyBindingFlags.NONE`
- Hardkodowanie wersji shell w metadata.json
- CSS animations -> używaj JavaScript animations

### ZAWSZE rób:
- `'use strict';` na początku pliku
- Sprzątaj WSZYSTKO w `disable()`: `obj?.destroy(); obj = null;`
- Usuwaj WSZYSTKIE timery: `GLib.Source.remove(timerId); timerId = null;`
- Sprawdzaj logi: `journalctl --user -u gnome-shell -f`
- Testuj po każdej zmianie (deploy + restart extension)

## Wzorzec enable/disable

```javascript
enable() {
    // Twórz WSZYSTKIE obiekty tutaj
    this._indicator = new PanelMenu.Button(0.0, this.metadata.name, false);
    this._icon = new St.Icon({...});
    this._indicator.add_child(this._icon);
    // ...
}

disable() {
    // Niszcz WSZYSTKO
    this._cleanupTimers();
    this._indicator?.destroy();
    this._indicator = null;
    this._icon = null;
    // Ungrab shortcuts
    if (this._action !== null) {
        global.display.ungrab_accelerator(this._action);
    }
}
```

## Timery

```javascript
// Tworzenie
this._timerId = GLib.timeout_add(GLib.PRIORITY_DEFAULT, ms, () => {
    // ...
    return GLib.SOURCE_REMOVE;    // jednorazowy
    return GLib.SOURCE_CONTINUE;  // powtarzalny
});

// Sprzątanie
if (this._timerId) {
    GLib.Source.remove(this._timerId);
    this._timerId = null;
}
```

## Animacje (JavaScript, NIE CSS)

```javascript
// CSS animations w GNOME Shell są zepsute/ograniczone
// Używaj GLib.timeout_add:
this._animTimer = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 50, () => {
    bar.set_pivot_point(0.5, 1.0);  // skaluj od dołu
    bar.scale_y = 0.5 + Math.sin(phase) * 0.5;
    phase += 0.1;
    return GLib.SOURCE_CONTINUE;
});

// Właściwości OK: scale_x, scale_y, opacity
// Właściwości NIE: height, width (ustaw raz, potem skaluj)
```

## D-Bus komunikacja z Go backend

```javascript
const VoicifyInterface = `
<node>
    <interface name="com.dooshek.voicify.Recorder">
        <method name="StartRecording">...</method>
        <method name="StopRecording">...</method>
        <signal name="RecordingStateChanged">...</signal>
    </interface>
</node>`;

this._proxy = Gio.DBusProxy.makeProxyWrapper(VoicifyInterface)(
    Gio.DBus.session,
    'com.dooshek.voicify',
    '/com/dooshek/voicify/Recorder'
);
```

## Keyboard shortcuts

```javascript
// enable():
this._action = global.display.grab_accelerator(
    '<Ctrl><Super>v', Meta.KeyBindingFlags.NONE
);
const name = Meta.external_binding_name_for_action(this._action);
Main.wm.allowKeybinding(name, Shell.ActionMode.ALL);

// disable():
global.display.ungrab_accelerator(this._action);
Main.wm.allowKeybinding(
    Meta.external_binding_name_for_action(this._action),
    Shell.ActionMode.NONE
);
```

## Text injection (X11 vs Wayland)

- **X11**: Virtual keyboard z `Clutter.get_default_backend().get_default_seat().create_virtual_device()`
- **Wayland**: Security blokuje symulację klawiatury - tylko clipboard copy

## Stan aplikacji

```javascript
const State = {
    IDLE: 'idle',
    RECORDING: 'recording',
    UPLOADING: 'uploading',
    FINISHED: 'finished',
    CANCELED: 'canceled'
};
```

20 barów wizualizacji z animacją scaling.

## Testowanie

```bash
# Deploy
rsync -av --delete --exclude=".git*" gnome-extension/ \
    ~/.local/share/gnome-shell/extensions/voicify@dooshek.com/
glib-compile-schemas ~/.local/share/gnome-shell/extensions/voicify@dooshek.com/schemas/

# Restart extension
gnome-extensions disable voicify@dooshek.com && sleep 1 && gnome-extensions enable voicify@dooshek.com

# Sprawdź status
gnome-extensions info voicify@dooshek.com

# Logi
journalctl --user -u gnome-shell -f | grep -i voicify

# X11: Alt+F2 -> r (restart shell)
# Wayland: logout/login (jedyny sposób)
```

## Clutter.Canvas - pułapki

- **NIGDY** nie twórz drugiego `Clutter.Canvas` + `Clutter.Actor` obok istniejącego bgCanvas
  - Powoduje cichy crash w `addChrome()` - widget nie pojawi się, bez error w logach
  - Rozwiązanie: rysuj dodatkowe efekty (np. pixelGrid) w draw handlerze wspólnego bgCanvas
  - Wzorzec: `_drawPixelGridOnCanvas(cr, layer, w, h)` wywoływane z bgCanvas draw callback
- `canvas.invalidate()` może wywołać draw handler synchronicznie - exception propaguje się do callera
- `cr.setOperator(1)` = `Cairo.Operator.SOURCE` - OK w GJS/GNOME Shell
- Owijaj `_createDecorations()` w try/catch - safety net dla widget layer errors

## Screenshotowanie do testów

- D-Bus `org.gnome.Shell.Screenshot.Screenshot` jest **zablokowany** (permission denied)
- Użyj ImageMagick: `import -window root output.png`
- Crop + zoom: `magick input.png -gravity South -crop 400x60+0+40 +repage -resize 400% output.png`

## Troubleshooting

1. **ZAWSZE** najpierw sprawdź logi: `journalctl --user --since "5 min ago" -p err | grep voicify`
2. Przeczytaj DOKŁADNY komunikat błędu - nie zgaduj
3. Sprawdź numer linii w stack trace
4. Extension w stanie ERROR -> sprawdź metadata.json, importy, enable/disable cleanup
5. Extension nie pojawia się na liście -> sprawdź nazwę katalogu = UUID, restartuj shell, lub:
   ```bash
   gdbus call --session --dest=org.gnome.Shell --object-path=/org/gnome/Shell \
       --method=org.gnome.Shell.Eval 'Main.extensionManager.scanExtensions()'
   ```
