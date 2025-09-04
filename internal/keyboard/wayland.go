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
		if strings.Contains(err.Error(), "permission denied") {
			fmt.Printf("Cannot access keyboard device.\n" +
				"Solution: \n" +
				"1. Add yourself to the input group: sudo usermod -aG input $USER \n" +
				"2. Log out and log back in (or restart your system) \n" +
				"3. Run the program again \n" +
				"Alternatively, you can run the program with sudo (not recommended).\n\n")
		}
		return fmt.Errorf("error initializing keylogger: %w", err)
	}

	w.keyboard = kbd
	events := kbd.Read()

	// Debounce threshold - ignore events that come too fast
	debounceThreshold := 500 * time.Millisecond

	for e := range events {
		if e.Type == keylogger.EvKey {
			code := uint16(e.Code)
			// Save event time for modifiers
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
						// Check time since last event of this key
						now := time.Now()
						if w.lastKeyEventTime.IsZero() || now.Sub(w.lastKeyEventTime) > debounceThreshold {
							w.lastKeyEventTime = now
							logger.Debugf("Detected key combination in Wayland, toggling recording")
							w.handleRecordingToggle()
						} else {
							timeSinceLastEvent := now.Sub(w.lastKeyEventTime).Milliseconds()
							logger.Debugf("Wayland: Ignoring event - too soon after previous (%d ms < %d ms threshold)",
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
