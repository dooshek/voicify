package pluginapi

import (
	"github.com/dooshek/voicify/internal/windowdetect"
)

// Window represents a window
type Window struct {
	Title string
	ID    string
}

// GetFocusedWindow gets the currently focused window
func GetFocusedWindow() (*Window, error) {
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
		ID:    window.ID,
	}, nil
}
