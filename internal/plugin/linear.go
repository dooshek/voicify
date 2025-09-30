package plugin

import (
	"fmt"
	"sync"

	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/plugin/linear"
	"github.com/dooshek/voicify/pkg/pluginapi"
)

// LinearPlugin is a plugin for Linear
type LinearPlugin struct {
	agenticLoop *linear.AgenticLoop
	mu          sync.RWMutex
}

// LinearAction is the Linear action
type LinearAction struct {
	transcription string
	plugin       *LinearPlugin
}

// Initialize initializes the Linear plugin
func (p *LinearPlugin) Initialize() error {
	logger.Debug("Linear plugin initialized")
	// AgenticLoop will be created lazily on first use to avoid blocking during MCP initialization
	return nil
}

// SetupLinearMCP initiates OAuth setup for Linear MCP
func (p *LinearPlugin) SetupLinearMCP() error {
	logger.Info("Starting Linear MCP OAuth setup...")

	if err := linear.SetupLinearMCP(); err != nil {
		logger.Errorf("Linear MCP setup failed: %v", err)
		return err
	}

	logger.Info("Linear MCP setup completed successfully!")
	return nil
}

// GetMetadata returns metadata about the plugin
func (p *LinearPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "linear",
		Version:     "1.0.0",
		Description: "Plugin for Linear",
		Author:      "Voicify Team",
	}
}

// GetActions returns a list of actions provided by this plugin
func (p *LinearPlugin) GetActions(transcription string) []pluginapi.PluginAction {
	return []pluginapi.PluginAction{
		&LinearAction{transcription: transcription, plugin: p},
	}
}

// Execute executes the Linear action
func (a *LinearAction) Execute(transcription string) error {
	logger.Debugf("Linear plugin: Executing action for transcription: %s", transcription)
	logger.Info("🔧 Linear plugin: Akcja została uruchomiona - plugin Linear jest aktywny")

	// Get or create agentic loop (lazy initialization)
	a.plugin.mu.Lock()
	agenticLoop := a.plugin.agenticLoop
	if agenticLoop == nil {
		logger.Debug("Creating agentic loop on first use...")
		var err error
		agenticLoop, err = linear.NewAgenticLoop()
		if err != nil {
			a.plugin.mu.Unlock()
			logger.Errorf("Failed to initialize agentic loop: %v", err)
			logger.Info("⚠️  Linear MCP client may not be initialized yet. Try again in a moment.")
			return fmt.Errorf("failed to initialize agentic loop: %w", err)
		}
		a.plugin.agenticLoop = agenticLoop
		logger.Debug("Agentic loop created successfully")
	}
	a.plugin.mu.Unlock()

	// Check current state
	currentState := agenticLoop.GetState()
	logger.Debugf("Current agentic loop state: %s", currentState)

	switch currentState {
	case linear.StateIdle:
		// Start new agentic loop
		logger.Debug("Starting new agentic loop")
		if err := agenticLoop.Start(transcription); err != nil {
			return err
		}
		// Don't return here - let the loop continue running
		logger.Debug("Agentic loop started, continuing to listen for responses")
		return nil
	case linear.StateWaitingResponse:
		// Process user response
		logger.Debug("Processing user response")
		if err := agenticLoop.ProcessResponse(transcription); err != nil {
			return err
		}
		// Don't return here - let the loop continue running
		logger.Debug("User response processed, continuing agentic loop")
		return nil
	case linear.StateAnswering:
		// Agent is providing answer, ignore new transcriptions
		logger.Debug("Agent is providing answer, ignoring transcription")
		return nil
	default:
		logger.Warnf("Agentic loop is in state %s, ignoring transcription", currentState)
		return nil
	}
}

// GetMetadata returns metadata about the action
func (a *LinearAction) GetMetadata() pluginapi.ActionMetadata {
	// Check if agentic loop is active
	a.plugin.mu.RLock()
	agenticLoop := a.plugin.agenticLoop
	a.plugin.mu.RUnlock()

	// If agentic loop is active (waiting for response or answering), make this a high-priority non-LLM action
	if agenticLoop != nil && (agenticLoop.GetState() == linear.StateWaitingResponse ||
	                          agenticLoop.GetState() == linear.StateAnswering) {
		return pluginapi.ActionMetadata{
			Name:        "linear",
			Description: "active Linear agentic loop - processing user response",
			Priority:    100, // Highest priority
			// No LLMRouterPrompt - will be executed directly without LLM routing
		}
	}

	// Normal LLM-routed action when agentic loop is idle
	prompt := `linear: Zarządzanie ticketami w Linear. Rozpoznaje TYLKO operacyjne komendy związane z akcjami na ticketach/issue'ach w aplikacji Linear.

WYKONUJ gdy słyszysz polecenia operacyjne:
- Tworzenie: "stwórz tiket", "dodaj nowy issue", "utwórz zadanie w Linear", "nowy ticket", "linear zadanie"
- Edycja: "edytuj ticket", "zmień status tiketu", "zaktualizuj issue", "usuń ticket", "zamknij issue"
- Wyszukiwanie/Zarządzanie: "pokaż moje tickety", "znajdź issue", "lista zadań w Linear", "status ticketów", "poszukaj tickety", "ile ticketów", "sprawdź tickety", "wyświetl issue", "chciałbym żebyś poszukał", "ile razy występuje", "ile jest takich", "znajdź duplikaty"
- Raportowanie/Status: "jakie tickety zostały zamknięte", "co się działo dzisiaj w Linear", "jakie zadania skończone", "opowiedz o ticketach", "raport z Linear", "co było robione", "które tickety są done", "jakie issues zakończone"
- Usuwanie: "usuń ticket", "skasuj issue", "wyrzuć zadanie", "delete ticket"

NIE WYKONUJ gdy to tylko rozmowa o Linear:
- "Linear to dobre narzędzie"
- "co myślisz o Linear"
- "Linear ma fajny interface"
- "używam Linear w pracy"

Kluczowe: musi być wyraźne POLECENIE AKCJI, nie tylko wzmianka o Linear.`

	return pluginapi.ActionMetadata{
		Name:            "linear",
		Description:     "wykonanie akcji w Linear - tworzenie i edycja ticketów",
		Priority:        2,
		LLMRouterPrompt: &prompt,
	}
}

// NewLinearPlugin creates a new instance of the Linear plugin
func NewLinearPlugin() pluginapi.VoicifyPlugin {
	return &LinearPlugin{}
}
