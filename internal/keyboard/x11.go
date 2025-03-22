package keyboard

import (
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
	hook "github.com/robotn/gohook"
)

type X11Monitor struct {
	BaseMonitor
}

func NewX11Monitor(keyConfig types.KeyBinding) (*X11Monitor, error) {
	targetKeyCode := X11KeyCodes[keyConfig.Key]
	base, err := NewBaseMonitor(keyConfig, targetKeyCode)
	if err != nil {
		return nil, err
	}
	return &X11Monitor{
		BaseMonitor: *base,
	}, nil
}

func (x *X11Monitor) Start() error {
	evChan := hook.Start()
	defer hook.End()

	for ev := range evChan {
		code := uint16(ev.Rawcode)

		if ev.Kind == hook.KeyDown {
			switch code {
			case X11LeftControl, X11RightControl:
				x.modifierState.Ctrl = true
			case X11LeftShift, X11RightShift:
				x.modifierState.Shift = true
			case X11LeftAlt, X11RightAlt:
				x.modifierState.Alt = true
			case X11Super:
				x.modifierState.Super = true
			default:
				shouldToggle := false

				// Try direct code comparison
				if code == x.targetKeyCode && x.checkModifiers() {
					shouldToggle = true
				} else if key, ok := X11KeyMap[code]; ok && key == x.keyConfig.Key && x.checkModifiers() {
					// Try checking using X11KeyMap lookup
					shouldToggle = true
				} else if ev.Keychar >= 32 && ev.Keychar <= 126 && string(ev.Keychar) == x.keyConfig.Key && x.checkModifiers() {
					// Try using ASCII char value as fallback
					shouldToggle = true
				}

				if shouldToggle {
					logger.Debugf("Wykryto kombinację klawiszy, przełączanie nagrywania")
					x.handleRecordingToggle()
				}
			}
		} else if ev.Kind == hook.KeyUp {
			switch code {
			case X11LeftControl, X11RightControl:
				x.modifierState.Ctrl = false
			case X11LeftShift, X11RightShift:
				x.modifierState.Shift = false
			case X11LeftAlt, X11RightAlt:
				x.modifierState.Alt = false
			case X11Super:
				x.modifierState.Super = false
			}
		}
	}
	return nil
}

func (x *X11Monitor) Stop() {
	hook.End()
}
