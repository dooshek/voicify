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
	Config     *types.Config
	ttsManager interface{} // Use interface{} to avoid import cycle, will be *tts.Manager
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

func (s *AppState) GetTranscriptionModel() string {
	return s.Config.LLM.Transcription.Model
}

func (s *AppState) GetRouterModel() string {
	return s.Config.LLM.Router.Model
}
