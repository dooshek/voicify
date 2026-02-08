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

## Reload na Wayland vs X11

- **X11**: `gnome-shell --replace &` przeładowuje JS w pełni. Po restarcie `gnome-extensions enable`.
- **Wayland**: `disable/enable` **NIE** przeładowuje JS (GJS cachuje ES modules). Tylko CSS się odświeża. Zmiany JS wymagają **logout/login**.
- `Shell.Eval` zablokowany w GNOME 49 - `Meta.restart()` via D-Bus nie działa.
- Nested session (`dbus-run-session -- gnome-shell --nested --wayland`) działa, ale daemon nie startuje (wykrywa istniejącą instancję).

## Shell.BlurEffect (GNOME 49)

Właściwości: `radius`, `brightness`, `mode`, `enabled`.

```javascript
// KRYTYCZNE: bez mode: BACKGROUND blur nic nie robi (domyślnie bluruje aktora, nie tło)
const blurEffect = new Shell.BlurEffect({
    brightness: 0.85,
    mode: Shell.BlurMode.BACKGROUND,  // WYMAGANE dla frosted glass
});
blurEffect.set_radius(40);  // użyj set_radius(), nie .radius =
widget.add_effect_with_name('blur', blurEffect);

// Cleanup w disable():
const effect = widget.get_effect('blur');
if (effect) widget.remove_effect(effect);
widget.destroy();
```

- **KRYTYCZNE: Shell.BlurEffect jest ZAWSZE prostokątny** - nie da się go obciąć do border-radius
  - Próbowano: osobne chrome, child waveWidget, inset, background-color hack - nic nie działa
  - Rectangular blur corners są zawsze widoczne na rounded designs
  - **Rozwiązanie: NIE używaj Shell.BlurEffect na zaokrąglonych widgetach** - symuluj frosted glass z wyższym bgOpacity + Cairo layers
- Jeśli kiedyś blur z border-radius będzie potrzebny, wymaga custom shader (C, nie GJS)

## Cairo w GJS ES modules

**Problem**: `imports.cairo` może nie działać w strict ES module context. `import Cairo from 'cairo'` - nieweryfikowalne bez runtime GNOME Shell.

**Rozwiązanie**: Zamiast Cairo patterns (LinearGradient, RadialGradient), używaj strip-based rendering:

```javascript
// Gradient border - clipowane paski z różnym alpha
const steps = 12;
for (let s = 0; s < steps; s++) {
    const t = s / (steps - 1);
    const a = alphaTop + (alphaBottom - alphaTop) * t;
    cr.save();
    cr.rectangle(x - bw, y + (h * s / steps), w + 2 * bw, h / steps + 1);
    cr.clip();
    cr.setSourceRGBA(r, g, b, a);
    cairoRoundedRect(cr, x + bw/2, y + bw/2, w - bw, h - bw, radius);
    cr.stroke();
    cr.restore();
}
```

- `cr` z `St.DrawingArea.get_context()` - standardowy Cairo context
- `cr.$dispose()` po użyciu!
- St.DrawingArea jest domyślnie przezroczyste - rysuje tylko to co Cairo narysuje
- `cairoRoundedRect()` helper w `layerPainter.js` - arc-based rounded rect

## St.DrawingArea - obcinanie Cairo rendering

**KRYTYCZNE**: St.DrawingArea obcina Cairo rendering do swojej alokacji (set_size). Jeśli rysujesz shadow/glow POZA granicami widgetu, zostanie obcięty do prostokąta → widoczne prostokątne rogi.

**Rozwiązanie**: Powiększ DrawingArea o padding na shadow i offsetuj rysowanie:

```javascript
// Oblicz padding z warstw shadow w designie
const shadowLayers = design.layers.filter(l => l.type === 'shadow');
let shadowPad = 0;
for (const sl of shadowLayers) {
    shadowPad = Math.max(shadowPad, (sl.blur || 8) + Math.abs(sl.x || 0),
                                     (sl.blur || 8) + Math.abs(sl.y || 0));
}

bgWidget.set_size(totalWidth + shadowPad * 2, totalHeight + shadowPad * 2);
bgWidget.set_position(-shadowPad, -shadowPad);

// Rysuj z offsetem (shadow ma miejsce na zanikanie)
LayerPainter.drawAllCanvasLayersAt(cr, design, theme,
    shadowPad, shadowPad, totalWidth, totalHeight);
```

- Inne widgety (bary, blur, pixelGrid) zostają na `totalWidth x totalHeight` i pozycji (0, 0)
- Rodzic (St.Widget) NIE ma `clip_to_allocation` → children mogą overflow

## Clutter.Canvas - pułapki

- **NIGDY** nie twórz drugiego `Clutter.Canvas` + `Clutter.Actor` obok istniejącego bgCanvas
  - Powoduje cichy crash w `addChrome()` - widget nie pojawi się, bez error w logach
  - Rozwiązanie: rysuj dodatkowe efekty (np. pixelGrid) w draw handlerze wspólnego bgCanvas
- `canvas.invalidate()` może wywołać draw handler synchronicznie - exception propaguje się do callera
- `cr.setOperator(1)` = `Cairo.Operator.SOURCE` - OK w GJS/GNOME Shell
- Owijaj `_createDecorations()` w try/catch - safety net dla widget layer errors

## Design System - architektura warstw

Designy: `gnome-extension/designs/*.json` z `layers[]` array.

Dwa typy warstw:
- **Canvas layers** (rysowane Cairo w `layerPainter.js`): shadow, border, innerHighlight, specularHighlight, innerShadow
- **Widget layers** (St.Widget w `extension.js _createDecorations()`): scanlines, frame, highlightStrip, pixelGrid

Kolejność renderingu w `_showWaveWidget()`:
1. `_blurWidget` (osobne chrome, pod spodem) - frosted glass
2. `_bgWidget` (St.DrawingArea) - Cairo canvas layers + tło
3. `_pixelGridWidget` (St.Widget z clip_to_allocation)
4. `_trailContainer` (shadow bars)
5. `_waveContainer` (główne bary wizualizacji)
6. Decoration widgets (scanlines, frame, etc.)

## Screenshotowanie do testów

- D-Bus `org.gnome.Shell.Screenshot.Screenshot` jest **zablokowany** (permission denied)
- Użyj ImageMagick: `import -window root output.png`
- Użytkownik robi screenshoty GNOME → `tmp/screenshots/` (symlink do `~/Pictures/Screenshots/`)
- Skill `/check-screenshot` - otwórz ostatnie screenshoty i oceń wizualnie

## Troubleshooting

1. **ZAWSZE** najpierw sprawdź logi: `journalctl --user --since "5 min ago" -p err | grep voicify`
2. Przeczytaj DOKŁADNY komunikat błędu - nie zgaduj
3. Sprawdź numer linii w stack trace
4. Extension w stanie ERROR -> sprawdź metadata.json, importy, enable/disable cleanup
5. Extension nie pojawia się na liście -> sprawdź nazwę katalogu = UUID, restartuj shell
