package keyboard

import (
	"context"
	"fmt"
	"time"

	"github.com/MarinX/keylogger"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
)

type WaylandMonitor struct {
	BaseMonitor
	keyboard         *keylogger.KeyLogger
	lastKeyEventTime time.Time
}

func NewWaylandMonitor(keyConfig types.KeyBinding) (*WaylandMonitor, error) {
	targetKeyCode := WaylandKeyCodes[keyConfig.Key]
	base, err := NewBaseMonitor(keyConfig, targetKeyCode)
	if err != nil {
		return nil, err
	}
	return &WaylandMonitor{
		BaseMonitor:      *base,
		lastKeyEventTime: time.Time{}, // Zero time
	}, nil
}

func (w *WaylandMonitor) Start(ctx context.Context) error {
	keyboards := keylogger.FindAllKeyboardDevices()

	if len(keyboards) == 0 {
		return fmt.Errorf("no keyboard devices found")
	}

	kbd, err := keylogger.New(keyboards[0])
	if err != nil {
		if err.Error() == "permission denied" ||
			err.Error() == "permission denied. run with root permission or use a user with access to /dev/input/event3" {
			return fmt.Errorf("permission denied. Run: sudo usermod -aG input $USER and log out/in")
		}
		return fmt.Errorf("error initializing keylogger: %w", err)
	}

	w.keyboard = kbd
	events := kbd.Read()

	// Debounce threshold - ignoruje zdarzenia, które przychodzą zbyt szybko
	debounceThreshold := 500 * time.Millisecond

	for e := range events {
		if e.Type == keylogger.EvKey {
			code := uint16(e.Code)
			// Zapisz czas zdarzenia dla modyfikatorów
			if e.KeyPress() {
				switch code {
				case WaylandLeftControl, WaylandRightControl:
					w.modifierState.Ctrl = true
				case WaylandLeftShift, WaylandRightShift:
					w.modifierState.Shift = true
				case WaylandLeftAlt, WaylandRightAlt:
					w.modifierState.Alt = true
				case WaylandSuper:
					w.modifierState.Super = true
				default:
					if code == w.targetKeyCode && w.checkModifiers() {
						// Sprawdź czas od ostatniego zdarzenia tego klawisza
						now := time.Now()
						if w.lastKeyEventTime.IsZero() || now.Sub(w.lastKeyEventTime) > debounceThreshold {
							w.lastKeyEventTime = now
							logger.Debugf("Wykryto kombinację klawiszy w Wayland, przełączanie nagrywania")
							w.handleRecordingToggle()
						} else {
							timeSinceLastEvent := now.Sub(w.lastKeyEventTime).Milliseconds()
							logger.Debugf("Wayland: Ignorowanie zdarzenia - zbyt szybko po poprzednim (%d ms < %d ms próg)",
								timeSinceLastEvent, debounceThreshold.Milliseconds())
						}
					}
				}
			} else if e.KeyRelease() {
				switch code {
				case WaylandLeftControl, WaylandRightControl:
					w.modifierState.Ctrl = false
				case WaylandLeftShift, WaylandRightShift:
					w.modifierState.Shift = false
				case WaylandLeftAlt, WaylandRightAlt:
					w.modifierState.Alt = false
				case WaylandSuper:
					w.modifierState.Super = false
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

func (w *WaylandMonitor) Stop() {
	if w.keyboard != nil {
		w.keyboard.Close()
	}
}
