---
description: Git commit workflow i konwencje dla Voicify
---

# Git Workflow - Voicify

## Commit workflow

1. **NIE commituj automatycznie** - przygotuj komendy do review
2. Sprawdź zmiany: `git status`, `git diff`
3. Pogrupuj powiązane zmiany logicznie
4. Przygotuj `git add <konkretne pliki>` (nigdy `git add .`)
5. Napisz commit message w formacie conventional commits
6. Pokaż przygotowane komendy użytkownikowi
7. Wykonaj dopiero po potwierdzeniu

## Format commit message

```
type(scope): zwięzły opis

- bullet point 1
- bullet point 2
```

**Typy:** feat, fix, docs, style, refactor, test, chore

**Scope'y:** extension, dbus, audio, tts, plugin, linear, config, keyboard

**Przykłady:**
- `feat(extension): add waveform visualization bars`
- `fix(dbus): resolve recording state signal race condition`
- `refactor(plugin): extract MCP client to separate module`
- `chore: update go.mod dependencies`

## Branch strategy

- Praca na feature branches, nie na main
- Atomowe, skupione commity
- Squash powiązanych commitów przed merge
