package keyboard

import (
	"context"
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

// ModifierState tracks the state of modifier keys (Ctrl, Shift, Alt, Super)
type ModifierState struct {
	Ctrl  bool
	Shift bool
	Alt   bool
	Super bool
}

// KeyboardMonitor interface defines the contract for keyboard monitoring implementations
type KeyboardMonitor interface {
	Start(ctx context.Context) error
	Stop()
}

// Simple block of keyboard shortcuts
var (
	isBlocked      bool
	blockMutex     sync.Mutex
	blockUntilTime time.Time
)

// BlockKeyboardShortcuts blocks shortcuts for a specified duration
func BlockKeyboardShortcuts(duration time.Duration) {
	blockMutex.Lock()
	isBlocked = true
	blockUntilTime = time.Now().Add(duration)
	blockMutex.Unlock()
	logger.Debugf("Keyboard shortcuts blocked for %d seconds", int(duration.Seconds()))
}

// isKeyboardBlocked checks if keyboard shortcuts are currently blocked
func isKeyboardBlocked() bool {
	blockMutex.Lock()
	defer blockMutex.Unlock()

	// If the block time has passed, automatically unlock
	if isBlocked && time.Now().After(blockUntilTime) {
		isBlocked = false
		logger.Debugf("Keyboard shortcuts blocked ended")
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
	// First check if shortcuts are blocked
	if isKeyboardBlocked() {
		logger.Debugf("Shortcut ignored - keyboard shortcuts are blocked")
		return
	}

	if !b.recorder.IsRecording() {
		logger.Debugf("Starting recording")
		b.recorder.Start()
	} else {
		logger.Debugf("Stopping recording")

		// Block ALL incomming keyboard events for 5 seconds
		// To bardzo agresywne podejście ale powinno rozwiązać problem
		BlockKeyboardShortcuts(5 * time.Second)

		transcription, err := b.recorder.Stop()
		if err != nil {
			logger.Errorf("Error stopping recording: %v", err)
			return
		}

		router := transcriptionrouter.New(transcription)
		if err := router.Route(transcription); err != nil {
			logger.Errorf("Error routing transcription: %v", err)
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
