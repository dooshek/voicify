---
name: build
description: Kompilacja projektu Voicify (go build + go vet)
allowed-tools: Bash
---

# /build - Kompilacja projektu

Wykonaj następujące kroki:

1. Uruchom kompilację:
```bash
cd /home/dooshek/projects/voicify/main && go build -o bin/voicify ./cmd/voicify/main.go
```

2. Jeśli kompilacja się powiodła, uruchom vet:
```bash
cd /home/dooshek/projects/voicify/main && go vet ./...
```

3. Opcjonalnie uruchom staticcheck:
```bash
cd /home/dooshek/projects/voicify/main && ~/go/bin/staticcheck ./...
```

4. Raportuj wyniki:
   - Jeśli wszystko OK: potwierdź sukces i rozmiar binarki
   - Jeśli błędy: pokaż błędy i zasugeruj poprawki
