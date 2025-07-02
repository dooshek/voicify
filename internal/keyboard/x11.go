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
		return fmt.Errorf("nie znaleziono urządzenia klawiatury - sprawdź uprawnienia (grupa input)")
	}

	logger.Debugf("Znaleziono klawiaturę: %s", keyboard)

	// Initialize keylogger
	k, err := keylogger.New(keyboard)
	if err != nil {
		return fmt.Errorf("błąd inicjalizacji keylogger: %v", err)
	}

	x.keyLogger = k
	x.isRunning = true

	logger.Debugf("Nasłuchiwanie kombinacji: Ctrl=%v, Shift=%v, Alt=%v, Super=%v, Key=%s",
		x.keyConfig.Ctrl, x.keyConfig.Shift, x.keyConfig.Alt, x.keyConfig.Super, x.keyConfig.Key)

	// Start monitoring in goroutine
	logger.Debugf("Uruchamiam goroutine monitorKeys()")
	go func() {
		logger.Debugf("Wewnątrz goroutine - zaraz wywołam monitorKeys()")
		x.monitorKeys()
		logger.Debugf("monitorKeys() zakończone")
	}()

	logger.Debugf("Goroutine monitorKeys() uruchomiona, metoda Start() zakończona pomyślnie")

	// Krótkie oczekiwanie aby upewnić się że goroutine ma szansę na start
	time.Sleep(100 * time.Millisecond)
	logger.Debugf("Po time.Sleep - sprawdzam czy goroutine działa")

	// Blokuj i czekaj na zakończenie - nieskończenie
	select {
	case <-x.ctx.Done():
		logger.Debugf("Kontekst anulowany, kończę Start()")
	}

	return nil
}

func (x *X11Monitor) monitorKeys() {
	logger.Debugf("monitorKeys() - wejście do funkcji")

	defer func() {
		logger.Debugf("monitorKeys() - defer: zamykam keyLogger")
		x.keyLogger.Close()
	}()

	logger.Debugf("monitorKeys() - wywołuję x.keyLogger.Read()")
	events := x.keyLogger.Read()
	logger.Debugf("monitorKeys() - otrzymałem kanał events: %v", events != nil)

	// State tracking for modifiers
	ctrlPressed := false
	shiftPressed := false
	altPressed := false
	superPressed := false

	lastToggleTime := time.Now()
	debounceInterval := 200 * time.Millisecond

	logger.Debugf("Rozpoczynam pętlę nasłuchiwania klawiszy")

	for {
		select {
		case <-x.ctx.Done():
			logger.Debugf("Kontekst został anulowany, kończę nasłuchiwanie")
			return
		case e, ok := <-events:
			if !ok {
				logger.Debugf("Kanał events został zamknięty")
				return
			}

			logger.Debugf("Otrzymano event: Type=%d, KeyString=%s, KeyCode=%d", e.Type, e.KeyString(), e.Code)

			if e.Type != keylogger.EvKey {
				continue
			}

			keyName := strings.ToLower(e.KeyString())
			logger.Debugf("Event klawiatury: %s, Press=%v, Release=%v, KeyCode=%d", keyName, e.KeyPress(), e.KeyRelease(), e.Code)

			// Handle Super key by keycode since KeyString() is empty
			isSuperKey := (e.Code == 125) // Left Super/Windows key

			// Track modifier states
			if e.KeyPress() {
				switch {
				case keyName == "ctrl" || keyName == "leftctrl" || keyName == "rightctrl" || keyName == "l_ctrl":
					ctrlPressed = true
					logger.Debugf("Ctrl naciśnięty")
				case keyName == "shift" || keyName == "leftshift" || keyName == "rightshift":
					shiftPressed = true
					logger.Debugf("Shift naciśnięty")
				case keyName == "alt" || keyName == "leftalt" || keyName == "rightalt":
					altPressed = true
					logger.Debugf("Alt naciśnięty")
				case keyName == "leftmeta" || keyName == "rightmeta" || keyName == "cmd" || isSuperKey:
					superPressed = true
					logger.Debugf("Super naciśnięty (KeyCode=%d)", e.Code)
				default:
					logger.Debugf("Klawisz naciśnięty: %s", keyName)
					// Check if this is our target key with correct modifiers
					if x.isTargetKey(keyName) && x.modifiersMatch(ctrlPressed, shiftPressed, altPressed, superPressed) {
						// Debounce to prevent multiple triggers
						if time.Since(lastToggleTime) > debounceInterval {
							logger.Debugf("Wykryto kombinację klawiszy, przełączanie nagrywania")
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
					logger.Debugf("Ctrl zwolniony")
				case keyName == "shift" || keyName == "leftshift" || keyName == "rightshift":
					shiftPressed = false
					logger.Debugf("Shift zwolniony")
				case keyName == "alt" || keyName == "leftalt" || keyName == "rightalt":
					altPressed = false
					logger.Debugf("Alt zwolniony")
				case keyName == "leftmeta" || keyName == "rightmeta" || keyName == "cmd" || isSuperKey:
					superPressed = false
					logger.Debugf("Super zwolniony (KeyCode=%d)", e.Code)
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
		logger.Debugf("Zatrzymano monitorowanie klawiatury")
	}
}
