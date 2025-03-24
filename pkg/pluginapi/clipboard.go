package pluginapi

import (
	"github.com/dooshek/voicify/internal/clipboard"
)

// PasteText pastes text from the clipboard
func PasteText(text string) error {
	return clipboard.Paste(text)
}

// PasteWithReturn pastes text and adds a newline
func PasteWithReturn(text string) error {
	return clipboard.PasteWithReturn(text)
}
