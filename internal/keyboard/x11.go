package keyboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MarinX/keylogger"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
)

type X11Monitor struct {
	BaseMonitor
	isRunning bool
	keyLogger *keylogger.KeyLogger
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewX11Monitor(keyConfig types.KeyBinding) (*X11Monitor, error) {
	targetKeyCode := X11KeyCodes[keyConfig.Key]
	base, err := NewBaseMonitor(keyConfig, targetKeyCode)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &X11Monitor{
		BaseMonitor: *base,
		isRunning:   false,
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

func (x *X11Monitor) Start(ctx context.Context) error {
	if x.isRunning {
		return nil
	}

	// Use provided context instead of creating new one
	x.ctx, x.cancel = context.WithCancel(ctx)

	// Find keyboard device
	keyboard := keylogger.FindKeyboardDevice()
	if keyboard == "" {
		return fmt.Errorf("no keyboard device found - check permissions (input group)")
	}

	logger.Debugf("Found keyboard: %s", keyboard)

	// Initialize keylogger
	k, err := keylogger.New(keyboard)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			fmt.Printf("Cannot access keyboard device.\n" +
				"Solution:\n" +
				"1. Add yourself to the input group: sudo usermod -aG input $USER\n" +
				"2. Log out and log back in (or restart your system)\n" +
				"3. Run the program again\n" +
				"Alternatively, you can run the program with sudo (not recommended).\n\n")
		}
		return fmt.Errorf("error initializing keylogger: %w", err)
	}

	x.keyLogger = k
	x.isRunning = true

	logger.Debugf("Listening to combination: Ctrl=%v, Shift=%v, Alt=%v, Super=%v, Key=%s",
		x.keyConfig.Ctrl, x.keyConfig.Shift, x.keyConfig.Alt, x.keyConfig.Super, x.keyConfig.Key)

	// Start monitoring in goroutine
	logger.Debugf("Starting goroutine monitorKeys()")
	go func() {
		logger.Debugf("Inside goroutine - soon calling monitorKeys()")
		x.monitorKeys()
		logger.Debugf("monitorKeys() completed")
	}()

	logger.Debugf("Goroutine monitorKeys() started, Start() completed successfully")

	// Krótkie oczekiwanie aby upewnić się że goroutine ma szansę na start
	time.Sleep(100 * time.Millisecond)
	logger.Debugf("After time.Sleep - checking if goroutine is running")

	// Blokuj i czekaj na zakończenie - nieskończenie
	select {
	case <-x.ctx.Done():
		logger.Debugf("Context cancelled, stopping Start()")
	}

	return nil
}

func (x *X11Monitor) monitorKeys() {
	logger.Debugf("monitorKeys() - entering function")

	defer func() {
		logger.Debugf("monitorKeys() - defer: closing keyLogger")
		x.keyLogger.Close()
	}()

	logger.Debugf("monitorKeys() - calling x.keyLogger.Read()")
	events := x.keyLogger.Read()
	logger.Debugf("monitorKeys() - received events channel: %v", events != nil)

	// State tracking for modifiers
	ctrlPressed := false
	shiftPressed := false
	altPressed := false
	superPressed := false

	lastToggleTime := time.Now()
	debounceInterval := 200 * time.Millisecond

	logger.Debugf("Starting loop for listening to keys")

	for {
		select {
		case <-x.ctx.Done():
			logger.Debugf("Context cancelled, stopping listening")
			return
		case e, ok := <-events:
			if !ok {
				logger.Debugf("Events channel closed")
				return
			}

			logger.Debugf("Received event: Type=%d, KeyString=%s, KeyCode=%d", e.Type, e.KeyString(), e.Code)

			if e.Type != keylogger.EvKey {
				continue
			}

			keyName := strings.ToLower(e.KeyString())
			logger.Debugf("Keyboard event: %s, Press=%v, Release=%v, KeyCode=%d", keyName, e.KeyPress(), e.KeyRelease(), e.Code)

			// Handle Super key by keycode since KeyString() is empty
			isSuperKey := (e.Code == 125) // Left Super/Windows key

			// Track modifier states
			if e.KeyPress() {
				switch {
				case keyName == "ctrl" || keyName == "leftctrl" || keyName == "rightctrl" || keyName == "l_ctrl":
					ctrlPressed = true
					logger.Debugf("Ctrl pressed")
				case keyName == "shift" || keyName == "leftshift" || keyName == "rightshift":
					shiftPressed = true
					logger.Debugf("Shift pressed")
				case keyName == "alt" || keyName == "leftalt" || keyName == "rightalt":
					altPressed = true
					logger.Debugf("Alt pressed")
				case keyName == "leftmeta" || keyName == "rightmeta" || keyName == "cmd" || isSuperKey:
					superPressed = true
					logger.Debugf("Super pressed (KeyCode=%d)", e.Code)
				default:
					logger.Debugf("Key pressed: %s", keyName)
					// Check if this is our target key with correct modifiers
					if x.isTargetKey(keyName) && x.modifiersMatch(ctrlPressed, shiftPressed, altPressed, superPressed) {
						// Debounce to prevent multiple triggers
						if time.Since(lastToggleTime) > debounceInterval {
							logger.Debugf("Detected key combination, toggling recording")
							x.handleRecordingToggle()
							lastToggleTime = time.Now()
						}
					}
				}
			} else if e.KeyRelease() {
				// Reset modifier states when released
				switch {
				case keyName == "ctrl" || keyName == "leftctrl" || keyName == "rightctrl" || keyName == "l_ctrl":
					ctrlPressed = false
					logger.Debugf("Ctrl released")
				case keyName == "shift" || keyName == "leftshift" || keyName == "rightshift":
					shiftPressed = false
					logger.Debugf("Shift released")
				case keyName == "alt" || keyName == "leftalt" || keyName == "rightalt":
				case keyName == "alt" || keyName == "leftalt" || keyName == "rightalt":
					altPressed = false
					logger.Debugf("Alt released")
				case keyName == "leftmeta" || keyName == "rightmeta" || keyName == "cmd" || isSuperKey:
					superPressed = false
					logger.Debugf("Super released (KeyCode=%d)", e.Code)
				}
			}
		}
	}
}

func (x *X11Monitor) isTargetKey(keyName string) bool {
	targetKey := strings.ToLower(x.keyConfig.Key)

	// Handle special key mappings
	switch targetKey {
	case "space":
		return keyName == "space"
	case "enter":
		return keyName == "enter"
	case "tab":
		return keyName == "tab"
	case "escape":
		return keyName == "esc"
	default:
		return keyName == targetKey
	}
}

func (x *X11Monitor) modifiersMatch(ctrlPressed, shiftPressed, altPressed, superPressed bool) bool {
	return x.keyConfig.Ctrl == ctrlPressed &&
		x.keyConfig.Shift == shiftPressed &&
		x.keyConfig.Alt == altPressed &&
		x.keyConfig.Super == superPressed
}

func (x *X11Monitor) Stop() {
	if x.isRunning {
		x.cancel()
		if x.keyLogger != nil {
			x.keyLogger.Close()
		}
		x.isRunning = false
		logger.Debugf("Stopped monitoring keyboard")
	}
}
