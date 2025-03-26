package pluginapi

import (
	"github.com/dooshek/voicify/pkg/windowdetect"
)

// Window represents a window
type Window struct {
	Title string
}

// NewWindow creates a new window instance
func NewWindow() *Window {
	return &Window{}
}

// GetFocusedWindow gets the currently focused window
func (w *Window) GetFocusedWindow() (*Window, error) {
	detector, err := windowdetect.New()
	if err != nil {
		return nil, err
	}

	window, err := detector.GetFocusedWindow()
	if err != nil {
		return nil, err
	}

	return &Window{
		Title: window.Title,
	}, nil
}
