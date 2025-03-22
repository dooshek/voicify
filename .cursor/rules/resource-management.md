# Resource Management

- All application resources should be stored in ~/.config/voicify/resources/
- Audio resources should be in resources/audio/
- Prompt files should be in resources/prompts/
- Use fileops.GetResourcesDir(), GetAudioDir(), and GetPromptsDir() for paths
- Always embed resource files using //go:embed
- Extract embedded resources during initialization
- Use proper error handling when dealing with resources
- Ensure directories exist before extracting resources
