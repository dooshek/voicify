---
name: deploy-extension
description: Deploy rozszerzenia GNOME Shell (rsync + compile schemas + restart)
allowed-tools: Bash
---

# /deploy-extension - Deploy rozszerzenia GNOME

Wykonaj skrypt deploy:

```bash
bash /home/dooshek/projects/voicify/main/deploy.sh
```

Jeśli status extension = ERROR, sprawdź szczegóły błędu:
```bash
gdbus call --session --dest=org.gnome.Shell --object-path=/org/gnome/Shell --method=org.gnome.Shell.Extensions.GetExtensionInfo "voicify@dooshek.com" 2>/dev/null | tr ',' '\n' | grep -i "error"
```

Raportuj:
- Status rozszerzenia (ACTIVE/ERROR) i daemon (running/dead) z outputu skryptu
- Ewentualne błędy
- **Wayland**: poinformuj że zmiany JS wymagają **logout/login**
