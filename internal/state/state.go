package state

import (
	"sync"

	"github.com/dooshek/voicify/internal/types"
)

var (
	once     sync.Once
	instance *AppState
)

type AppState struct {
	Config          *types.Config
	ttsManager      interface{} // Use interface{} to avoid import cycle, will be *tts.Manager
	router          interface{} // Use interface{} to avoid import cycle, will be *transcriptionrouter.Router
	linearMCPClient interface{} // Use interface{} to avoid import cycle, will be *linear.LinearMCPClient
	dbusServer      interface{} // Use interface{} to avoid import cycle, will be *dbus.Server
	// Focused window cache provided by GNOME extension via D-Bus
	focusedWindowTitle string
	focusedWindowApp   string
	mu                 sync.RWMutex
}

func Init(cfg *types.Config) {
	once.Do(func() {
		instance = &AppState{
			Config: cfg,
		}
	})
}

func Get() *AppState {
	if instance == nil {
		panic("AppState not initialized")
	}
	return instance
}

// SetTTSManager sets the TTS manager in the global state
func (s *AppState) SetTTSManager(manager interface{}) {
	s.ttsManager = manager
}

// GetTTSManager returns the TTS manager from global state
func (s *AppState) GetTTSManager() interface{} {
	return s.ttsManager
}

func (s *AppState) GetTranscriptionProvider() types.LLMProvider {
	return types.LLMProvider(s.Config.LLM.Transcription.Provider)
}

func (s *AppState) GetRouterProvider() types.LLMProvider {
	return types.LLMProvider(s.Config.LLM.Router.Provider)
}

func (s *AppState) GetRouterModel() string {
	return s.Config.LLM.Router.Model
}

// SetRouter sets the global router in the state
func (s *AppState) SetRouter(router interface{}) {
	s.router = router
}

// GetRouter returns the global router from state
func (s *AppState) GetRouter() interface{} {
	return s.router
}

// SetLinearMCPClient sets the global Linear MCP client in the state
func (s *AppState) SetLinearMCPClient(client interface{}) {
	s.linearMCPClient = client
}

// GetLinearMCPClient returns the global Linear MCP client from state
func (s *AppState) GetLinearMCPClient() interface{} {
	return s.linearMCPClient
}

// SetFocusedWindow updates cached info about the focused window
func (s *AppState) SetFocusedWindow(title string, app string) {
	s.mu.Lock()
	s.focusedWindowTitle = title
	s.focusedWindowApp = app
	s.mu.Unlock()
}

// GetFocusedWindow returns cached focused window info (title, app)
func (s *AppState) GetFocusedWindow() (string, string) {
	s.mu.RLock()
	title := s.focusedWindowTitle
	app := s.focusedWindowApp
	s.mu.RUnlock()
	return title, app
}

// SetDBusServer sets the DBus server in the global state
func (s *AppState) SetDBusServer(server interface{}) {
	s.dbusServer = server
}

// GetDBusServer returns the DBus server from global state
func (s *AppState) GetDBusServer() interface{} {
	return s.dbusServer
}
