package keyboard

import (
	"time"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
	"github.com/go-vgo/robotgo"
)

type X11Monitor struct {
	BaseMonitor
	stopChan chan bool
	lastPress time.Time
	wasPressed bool
}

func NewX11Monitor(keyConfig types.KeyBinding) (*X11Monitor, error) {
	targetKeyCode := X11KeyCodes[keyConfig.Key]
	base, err := NewBaseMonitor(keyConfig, targetKeyCode)
	if err != nil {
		return nil, err
	}
	return &X11Monitor{
		BaseMonitor: *base,
		stopChan:    make(chan bool),
	}, nil
}

func (x *X11Monitor) Start() error {
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond) // Check every 50ms
		defer ticker.Stop()
		
		for {
			select {
			case <-x.stopChan:
				return
			case <-ticker.C:
				if x.checkKeyCombo() {
					if !x.wasPressed {
						x.wasPressed = true
						logger.Debugf("Wykryto kombinację klawiszy, przełączanie nagrywania")
						x.handleRecordingToggle()
					}
				} else {
					x.wasPressed = false
				}
			}
		}
	}()
	
	// Keep the main goroutine alive
	<-x.stopChan
	return nil
}

func (x *X11Monitor) checkKeyCombo() bool {
	// Check modifiers first
	if x.keyConfig.Ctrl {
		mleft := robotgo.KeyTap("ctrl")
		mright := robotgo.KeyTap("ctrl.r")
		if mleft != nil && mright != nil {
			return false
		}
	}
	
	if x.keyConfig.Shift {
		mleft := robotgo.KeyTap("shift")
		mright := robotgo.KeyTap("shift.r") 
		if mleft != nil && mright != nil {
			return false
		}
	}
	
	if x.keyConfig.Alt {
		mleft := robotgo.KeyTap("alt")
		mright := robotgo.KeyTap("alt.r")
		if mleft != nil && mright != nil {
			return false
		}
	}
	
	if x.keyConfig.Super {
		mleft := robotgo.KeyTap("super")
		mright := robotgo.KeyTap("super.r")
		if mleft != nil && mright != nil {
			return false
		}
	}
	
	// Check main key - for now just return true if we get here
	// This is a simplified implementation to get the app running
	return true
}

func (x *X11Monitor) Stop() {
	close(x.stopChan)
}
