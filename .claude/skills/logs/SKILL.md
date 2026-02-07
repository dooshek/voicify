---
name: logs
description: Przegląd logów GNOME Shell i Voicify daemon
allowed-tools: Bash, Read, Grep, Glob
---

# /logs - Przegląd logów

Zbierz i przeanalizuj logi z różnych źródeł:

## 1. Logi GNOME Shell (extension)

```bash
journalctl --user -u gnome-shell --since "15 min ago" | grep -iE "(voicify|JS ERROR|JS WARNING)" | tail -50
```

## 2. Błędy systemowe

```bash
journalctl --user --since "15 min ago" -p err -p warning | grep -iv systemd | tail -30
```

## 3. Status rozszerzenia

```bash
gnome-extensions info voicify@dooshek.com
```

## 4. D-Bus status

```bash
gdbus call --session --dest org.freedesktop.DBus \
    --object-path /org/freedesktop/DBus \
    --method org.freedesktop.DBus.ListNames 2>/dev/null | tr ',' '\n' | grep voicify
```

## 5. Analiza

Po zebraniu logów:
- Zidentyfikuj błędy i ostrzeżenia
- Pogrupuj wg komponentu (extension / backend / D-Bus / audio)
- Zasugeruj rozwiązania dla znalezionych problemów
- Jeśli brak problemów - potwierdź że system działa poprawnie
