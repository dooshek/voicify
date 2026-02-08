---
name: test-designs
description: Wizualne testowanie designow - ustawia kazdy design, odpala nagrywanie, robi screenshot i ocenia wyglad
allowed-tools: Bash, Read
---

# /test-designs - Wizualne testowanie designow

Testuj wizualnie kazdy design Voicify: ustaw design via gsettings, odpali nagrywanie przez D-Bus, poczekaj na stabilizacje widgetu, zrob screenshot, przeczytaj go i oceń.

## Konfiguracja

```bash
SCHEMA_DIR="$HOME/.local/share/gnome-shell/extensions/voicify@dooshek.com/schemas"
GSETTINGS="gsettings --schemadir $SCHEMA_DIR"
SCHEMA="org.gnome.shell.extensions.voicify"
SCREENSHOT_DIR="/tmp/voicify-design-tests"
mkdir -p "$SCREENSHOT_DIR"
```

## Procedura dla kazdego designu

Listę designów pobierz z plików JSON:
```bash
ls gnome-extension/designs/*.json | sed 's|.*/||;s|\.json||' | sort
```

Dla kazdego design ID:

### 1. Ustaw design
```bash
$GSETTINGS set $SCHEMA wave-design "$DESIGN_ID"
sleep 0.5
```

### 2. Rozpocznij nagrywanie (symuluj audio level)
```bash
# Start recording via D-Bus
gdbus call --session --dest=com.dooshek.voicify --object-path=/com/dooshek/voicify/Recorder --method=com.dooshek.voicify.Recorder.TogglePostTranscriptionAutoPaste

# Poczekaj na pojawienie sie widgetu i jego stabilizacje
sleep 2
```

### 3. Zrob screenshot
```bash
gdbus call --session --dest=org.gnome.Shell --object-path=/org/gnome/Shell/Screenshot --method=org.gnome.Shell.Screenshot.Screenshot false false "$SCREENSHOT_DIR/${DESIGN_ID}.png"
```

### 4. Anuluj nagrywanie
```bash
gdbus call --session --dest=com.dooshek.voicify --object-path=/com/dooshek/voicify/Recorder --method=com.dooshek.voicify.Recorder.CancelRecording
sleep 1
```

### 5. Przeczytaj screenshot (Read tool) i oceń

Uzyj Read tool na `$SCREENSHOT_DIR/${DESIGN_ID}.png` - Claude jest multimodalny i widzi obrazy.

Ocen:
- Czy kontener jest widoczny?
- Czy bary wizualizacji sa widoczne?
- Czy efekty (shadow, border, glow, pixel grid, scanlines) sa widoczne i wygladaja dobrze?
- Czy design wyglada jak jego nazwa sugeruje (Glass = szklany, Retro = CRT, Brutalist = surowy, itp.)?

## Po przejsciu wszystkich

Podsumuj ktore designy wygladaja dobrze, a ktore wymagaja poprawek. Podaj konkretne sugestie co zmienic (alpha, kolory, rozmiary, nowe warstwy).

## Uwaga

- Na Wayland zmiany JS wymagaja logout/login! Ten skill testuje aktualnie zaladowany kod.
- Jesli daemon nie jest uruchomiony, nagrywanie nie startuje. Sprawdz: `pgrep -f "voicify --daemon"`
- Jesli extension jest w stanie ERROR, napierw /deploy-extension
