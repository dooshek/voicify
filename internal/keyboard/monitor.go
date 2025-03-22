package keyboard

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dooshek/voicify/internal/audio"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/transcriptionrouter"
	"github.com/dooshek/voicify/internal/types"
)

// ModifierState tracks the state of modifier keys
type ModifierState struct {
	Ctrl  bool
	Shift bool
	Alt   bool
	Super bool
}

// KeyboardMonitor interface defines the contract for keyboard monitoring implementations
type KeyboardMonitor interface {
	Start() error
	Stop()
}

// Prosta blokada obsługi skrótów
var (
	isBlocked      bool
	blockMutex     sync.Mutex
	blockUntilTime time.Time
)

// BlockKeyboardShortcuts blokuje obsługę skrótów na określony czas
func BlockKeyboardShortcuts(duration time.Duration) {
	blockMutex.Lock()
	isBlocked = true
	blockUntilTime = time.Now().Add(duration)
	blockMutex.Unlock()
	logger.Debugf("Skróty klawiszowe zablokowane na %d sekund", int(duration.Seconds()))
}

// isKeyboardBlocked sprawdza czy obsługa skrótów jest obecnie zablokowana
func isKeyboardBlocked() bool {
	blockMutex.Lock()
	defer blockMutex.Unlock()

	// Jeśli czas blokady minął, automatycznie odblokuj
	if isBlocked && time.Now().After(blockUntilTime) {
		isBlocked = false
		logger.Debugf("Blokada skrótów klawiszowych zakończona")
	}

	return isBlocked
}

// BaseMonitor provides common functionality for keyboard monitors
type BaseMonitor struct {
	recorder      *audio.Recorder
	keyConfig     types.KeyBinding
	modifierState ModifierState
	targetKeyCode uint16
}

// NewBaseMonitor creates a new base monitor instance
func NewBaseMonitor(keyConfig types.KeyBinding, targetKeyCode uint16) (*BaseMonitor, error) {
	recorder, err := audio.NewRecorder()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize recorder: %w", err)
	}

	return &BaseMonitor{
		recorder:      recorder,
		keyConfig:     keyConfig,
		targetKeyCode: targetKeyCode,
	}, nil
}

// checkModifiers verifies if current modifier state matches the configuration
func (b *BaseMonitor) checkModifiers() bool {
	return b.modifierState.Ctrl == b.keyConfig.Ctrl &&
		b.modifierState.Shift == b.keyConfig.Shift &&
		b.modifierState.Alt == b.keyConfig.Alt &&
		b.modifierState.Super == b.keyConfig.Super
}

// handleRecordingToggle toggles the recording state
func (b *BaseMonitor) handleRecordingToggle() {
	// Najpierw sprawdź czy skróty są zablokowane
	if isKeyboardBlocked() {
		logger.Debugf("Skrót zignorowany - obsługa skrótów jest zablokowana")
		return
	}

	if !b.recorder.IsRecording() {
		logger.Debugf("Rozpoczynam nagrywanie")
		b.recorder.Start()
	} else {
		logger.Debugf("Zatrzymuję nagrywanie")

		// Blokuj ALL incomming keyboard events na 5 sekund
		// To bardzo agresywne podejście ale powinno rozwiązać problem
		BlockKeyboardShortcuts(5 * time.Second)

		transcription, err := b.recorder.Stop()
		if err != nil {
			logger.Errorf("Błąd podczas zatrzymywania nagrywania: %v", err)
			return
		}

		router := transcriptionrouter.New(transcription)
		if err := router.Route(transcription); err != nil {
			logger.Errorf("Błąd podczas routingu transkrypcji: %v", err)
		}
	}
}

// CreateMonitor creates the appropriate keyboard monitor based on the session type
func CreateMonitor(recordKey types.KeyBinding) (KeyboardMonitor, error) {
	if isX11() {
		return NewX11Monitor(recordKey)
	}
	return NewWaylandMonitor(recordKey)
}

// isX11 checks if the current session is running X11
func isX11() bool {
	session := os.Getenv("XDG_SESSION_TYPE")
	return strings.ToLower(session) == "x11"
}
