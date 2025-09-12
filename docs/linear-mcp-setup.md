# Linear MCP Setup

Linear MCP używa Personal Access Token (PAT) do autoryzacji.

## Setup

1. **Uruchom Voicify:**
   ```bash
   ./bin/voicify
   ```

2. **Automatyczny Setup:**
   - Przy pierwszym użyciu Linear, setup uruchomi się automatycznie
   - Przeglądarka otworzy stronę Linear API settings

3. **Stwórz Personal Access Token:**
   - Przejdź do Linear Settings > API (otwarte automatycznie)
   - Kliknij "Create Personal API Key"
   - Nazwij token (np. "Voicify MCP")
   - Skopiuj wygenerowany token

4. **Wprowadź Token:**
   - Wróć do terminala z Voicify
   - Wklej token gdy zostaniesz poproszony
   - System przetestuje token i zapisze automatycznie

## Konfiguracja

Po setup, w `voicify.yml` pojawi się:

```yaml
linear:
  access_token: "your_personal_access_token"
  is_configured: true
```

## Użycie

Po setup możesz używać Linear commands:

- "stwórz ticket w Linear"
- "pokaż moje tickety"
- "zaktualizuj ticket"

## Troubleshooting

- **Token nieprawidłowy**: Setup przetestuje token przed zapisaniem
- **Setup nie działa**: Sprawdź czy przeglądarka się otwiera
- **Błąd 401**: Token jest nieprawidłowy lub wygasł

## Architektura

- **MCP Client**: Komunikuje się z `https://mcp.linear.app/sse`
- **Personal Access Token**: Bezpieczna autoryzacja
- **Agentic Loop**: LLM decyduje które MCP tools użyć
- **TTS Integration**: Głosowe pytania i odpowiedzi
- **Interaktywny Setup**: Pyta o token i zapisuje do config
