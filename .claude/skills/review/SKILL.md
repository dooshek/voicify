---
name: review
description: Code review ostatnich zmian
allowed-tools: Bash, Read, Grep, Glob
---

# /review - Code Review

Przeprowadź code review ostatnich zmian.

## 1. Zbierz zmiany

```bash
cd /home/dooshek/projects/voicify/main && git diff HEAD
```

Jeśli brak zmian w working tree, sprawdź ostatni commit:
```bash
cd /home/dooshek/projects/voicify/main && git diff HEAD~1
```

## 2. Przeczytaj zmienione pliki

Przeczytaj pełną zawartość każdego zmienionego pliku, żeby zrozumieć kontekst.

## 3. Sprawdź (wg typu pliku)

### Pliki Go (*.go):
- Sygnatury loggera (Error/Errorf wymagają `err error` jako 2. arg)
- Error handling (propagacja z `%w`, logowanie na granicy)
- Resource leaks (goroutines, channels, file handles)
- Concurrency safety
- Graceful degradation dla external services

### Pliki JS (gnome-extension/):
- Cleanup w disable() (destroy + null)
- Timer cleanup (GLib.Source.remove)
- Zakazane: setTimeout, obiekty w constructor, Clutter w prefs.js
- Memory leaks (signal disconnects, chrome removal)

### Ogólne:
- Security (injection, hardcoded secrets)
- Performance (unnecessary allocations, busy loops)
- Edge cases (nil/null checks, empty strings)
- Spójność z konwencjami projektu

## 4. Format wyniku

Dla każdego problemu:
- **Plik:linia** - lokalizacja
- **Severity**: critical / warning / suggestion
- **Opis** - co jest nie tak
- **Fix** - sugerowana poprawka

Na końcu: ogólna ocena i podsumowanie.
