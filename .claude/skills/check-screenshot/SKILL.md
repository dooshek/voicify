---
name: check-screenshot
description: Sprawdz ostatnie screenshoty z tmp/screenshots - uzytkownik robi screenshoty i chce zeby Claude je zobaczyl i ocenil
allowed-tools: Bash, Read, Glob
---

# /check-screenshot - Sprawdzanie screenshotow

Uzytkownik robi screenshoty (GNOME Screenshot) i zapisuje je w `~/Pictures/Screenshots/`. Symlink do nich jest w `tmp/screenshots/`.

## Procedura

### 1. Znajdz najnowsze screenshoty
```bash
ls -lt tmp/screenshots/ | head -6
```

### 2. Przeczytaj najnowsze (1-3 ostatnie)

Uzyj Read tool na kazdym pliku - Claude jest multimodalny i widzi obrazy.

Domyslnie czytaj **3 najnowsze** screenshoty. Jesli uzytkownik powiedzial ile, przeczytaj tyle ile chce.

### 3. Oceń

Opisz co widzisz na screenshotach:
- Jak wyglada widget Voicify (design, ksztalt, efekty)
- Czy sa problemy wizualne (artefakty, brakujace elementy, zle rogi)
- Porownaj z oczekiwanym wygladen designu (sprawdz `gnome-extension/designs/*.json`)
- Zaproponuj konkretne poprawki jesli widac problemy
