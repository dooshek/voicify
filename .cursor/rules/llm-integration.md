# LLM Integration

- Always use internal LLM package for AI completions
- Never implement direct OpenAI client calls
- Location: internal/llm
- Usage: llm.Complete(prompt) for text completions
- Proper models are configured in the LLM package
- Do not override model selection in individual actions
- Possible OpenAI models: `gpt-4o-mini`, `gpt-4o`, `gpt-3.5-turbo`
- Do not use any other models than the ones listed above
