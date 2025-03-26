package pluginapi

import (
	"github.com/dooshek/voicify/pkg/clipboard"
)

// Clipboard is a wrapper around the clipboard package
type Clipboard struct{}

// NewClipboard creates a new clipboard instance
func NewClipboard() *Clipboard {
	return &Clipboard{}
}

// CopyToClipboard copies text to the clipboard
func (c *Clipboard) CopyToClipboard(text string) error {
	return clipboard.CopyToClipboard(text)
}

// PasteWithReturn pastes text and adds a newline
func (c *Clipboard) PasteWithReturn(text string) error {
	return clipboard.PasteWithReturn(text)
}
