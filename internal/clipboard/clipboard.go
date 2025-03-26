// internal/clipboard/clipboard.go
package clipboard

import (
	"github.com/dooshek/voicify/internal/types"
	"github.com/dooshek/voicify/pkg/clipboard"
)

// InitClipboard initializes the clipboard package with configuration
func InitClipboard(ydotoolCfg types.YdotoolConfig) {
	clipboard.InitClipboard(ydotoolCfg.SocketPath)
}

// CopyToClipboard copies the given text to the clipboard.
func CopyToClipboard(text string) error {
	return clipboard.CopyToClipboard(text)
}

// PasteWithReturn simulates pasting the given text and pressing the return key.
func PasteWithReturn(text string) error {
	return clipboard.PasteWithReturn(text)
}
