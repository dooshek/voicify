# Active Context

## Current Focus
Based on the codebase and IDEAS.md, the current focus appears to be extending the plugin architecture with new capabilities:

1. **Web Content Plugin**: Developing a plugin that can take a URL from clipboard, fetch the webpage content, and save it to Obsidian
2. **Memory Storage Plugin**: Creating a plugin that can store information in a vector database for later retrieval

## Recent Changes
- Added support for installing plugins from Git repositories
  - Users can now install plugins directly from Git repositories using `voicify plugin --install-repo <url>`
  - The system will clone the repository, build the plugin if needed, and install it
  - Simplified plugin repository requirements: repositories must have a main.go file in the root directory

## Active Decisions
1. **Plugin Architecture**: The system uses a plugin-based architecture for handling different types of transcriptions, which allows for easy extension
2. **Platform Support**: The application supports both X11 and Wayland with different implementation approaches
3. **Transcription Service**: Using OpenAI Whisper for transcription due to its accuracy and ease of integration
4. **Plugin Repository Structure**: Plugins installed from Git repositories must have their main.go file in the repository root

## Implementation Considerations
1. **Plugin Interface**: New plugins should implement the `PluginAction` interface with `Execute()` and `GetMetadata()` methods
2. **Configuration**: Consider how new plugins will be configured through the existing YAML configuration
3. **Dependencies**: Evaluate necessary dependencies for new features (e.g., Obsidian API, vector database)
4. **User Experience**: Maintain the seamless experience when adding new functionality

## Current Questions
1. How should the Obsidian integration work? Direct API, file system manipulation, or other approach?
2. What vector database would be most appropriate for the memory storage feature?
3. How should plugins be prioritized when multiple could handle the same transcription?
4. Are there additional AI capabilities that could enhance the transcription processing?

## Next Steps
1. Implement plugin for web content capture and Obsidian integration
2. Develop memory storage plugin with vector database integration
3. Update configuration system to handle new plugin options
4. Enhance documentation for new features
5. Consider additional AI integration beyond basic transcription
