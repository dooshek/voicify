package windowdetect

import (
	"fmt"
	"runtime"
)

// WindowInfo contains information about the focused window
type WindowInfo struct {
	Title   string
	AppName string
}

// Detector defines the interface for window detection
type Detector interface {
	GetFocusedWindow() (*WindowInfo, error)
}

type baseDetector struct {
	platform platformDetector
}

type platformDetector interface {
	getFocusedWindow() (*WindowInfo, error)
}

// New creates a new platform-specific window detector
func New() (Detector, error) {
	var platform platformDetector

	switch runtime.GOOS {
	case "darwin":
		platform = newDarwinDetector()
	default:
		platform = newLinuxDetector()
		if platform == nil {
			return nil, fmt.Errorf("failed to initialize Linux window detector: xdotool is not installed")
		}
	}

	return &baseDetector{platform: platform}, nil
}

// Common implementation for all platforms
func (d *baseDetector) GetFocusedWindow() (*WindowInfo, error) {
	return d.platform.getFocusedWindow()
}
