---
paths:
  - "gnome-extension/designs/**"
  - "gnome-extension/layerPainter.js"
  - "gnome-extension/designLoader.js"
---

# Design System - Voicify

## Architektura

Designy zdefiniowane w `gnome-extension/designs/*.json`. Każdy design ma:

```json
{
    "name": "Display Name",
    "sortOrder": 1,
    "defaultTheme": "ocean",
    "uploadHueShift": 120,
    "container": { "borderRadius", "bgColor", "bgOpacity", "blur?" },
    "bars": { "borderRadius", "shadow", "opacityMode", "pivotY", "scaleMin", "scaleMax", "glowFromTheme", "glowRadius", "glowAlpha", "widthAdjust", "colorMute", "colorOverride" },
    "layers": [ ... ]
}
```

## Typy warstw (layers[])

### Canvas layers - rysowane przez `layerPainter.js` na wspólnym bgCanvas

| Typ | Opis | Kluczowe pola |
|-----|------|---------------|
| `shadow` | Cień za kontenerem | `x, y, blur, color, alpha, source?` |
| `border` | Ramka kontenera | `width, color/source, alpha, gradient?, alphaBottom?` |
| `innerHighlight` | Biały gradient od góry | `alpha, height(0-1)` |
| `specularHighlight` | Eliptyczny odblask | `x, y, width, height (0-1), alpha, color?` |
| `innerShadow` | Ciemny gradient od krawędzi | `color, alpha, blur` |

### Widget layers - St.Widget/Clutter w `extension.js`

| Typ | Opis | Kluczowe pola |
|-----|------|---------------|
| `scanlines` | Linie CRT | `lineHeight, lineSpacing, color, alpha, borderRadius` |
| `pixelGrid` | Siatka pikseli CRT | `cellSpacing, alpha, colorSource` |
| `frame` | Rama dekoracyjna | `inset, borderWidth, borderRadius, color, alpha` |

Widget layers mają opcjonalne `position: "background"` (za barami) lub domyślnie foreground.

**UWAGA:** pixelGrid jest rysowany na wspólnym bgCanvas (nie osobny Clutter.Canvas!) via `_drawPixelGridOnCanvas()`. Patrz sekcja "Clutter.Canvas - pułapki" w gnome-extension.md.

## Kolejność renderowania

1. shadow layers (Canvas, przed fill)
2. background fill (Canvas, bgColor + bgOpacity)
3. innerHighlight (Canvas, po fill, clipped)
4. specularHighlight (Canvas, po fill, clipped)
5. innerShadow (Canvas, po fill, clipped)
6. border (Canvas, na wierzchu)
7. pixelGrid (rysowany na bgCanvas, za barami)
8. widget layers position:"background" (scanlines za barami)
9. trail bars + main bars
10. widget layers position:"foreground" (scanlines, frame nad barami)

## Moduły

| Plik | Odpowiedzialność |
|------|-----------------|
| `layerPainter.js` | Czyste Cairo rendering - działa w extension i prefs |
| `designLoader.js` | Ładowanie JSON, deep merge z defaults, fallback |
| `extension.js` | Widget layers, bgCanvas draw, bar rendering |
| `prefs.js` | Preview w ustawieniach via `drawAllCanvasLayersAt()` |

## Kolory w layers

- `color: [r, g, b]` - stały kolor (0-255)
- `source: "theme"` - kolor z aktualnego color theme (center color)
- `colorSource: "theme"` - dla pixelGrid, gradient center→edge

## Dodawanie nowego designu

1. Stwórz `gnome-extension/designs/nazwa.json`
2. Ustaw `sortOrder` (kolejność w UI)
3. Wybierz `defaultTheme` z dostępnych (ocean, coral, phosphor, ember, twilight, graphite, ...)
4. Zdefiniuj `container`, `bars`, `layers`
5. Deploy + test: `/test-designs`

## Dodawanie nowego typu warstwy

### Canvas layer (layerPainter.js):
1. Dodaj `export function drawLayerNewType(cr, layer, theme?, w, h, radius)`
2. Dodaj case w `drawAllCanvasLayers()` we właściwej fazie renderowania
3. Dodaj obsługę w `prefs.js` preview (automatycznie via `drawAllCanvasLayers`)

### Widget layer (extension.js):
1. Dodaj typ do `widgetTypes` array w `_createDecorations()`
2. Dodaj `case 'newType':` w switch
3. Stwórz metodę `_createNewType(layer, w, h)`
4. Jeśli wymaga Cairo: rysuj na bgCanvas (nie twórz osobnego Clutter.Canvas!)
