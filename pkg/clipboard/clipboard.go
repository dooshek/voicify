// pkg/clipboard/clipboard.go
package clipboard

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/go-vgo/robotgo"
)

var ydotoolSocket string

// InitClipboard initializes the clipboard package with configuration
func InitClipboard(socketPath string) {
	ydotoolSocket = socketPath
}

// CopyToClipboard copies the given text to the clipboard.
func CopyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		// macOS
		cmd = exec.Command("pbcopy")
	case "linux":
		// Linux
		if _, err := exec.LookPath("xclip"); err != nil {
			return errors.New("xclip is not installed")
		}
		cmd = exec.Command("xclip", "-selection", "clipboard")
		pipeReader, pipeWriter := io.Pipe()
		cmd.Stdin = pipeReader

		go func() {
			defer pipeWriter.Close()
			pipeWriter.Write([]byte(text))
		}()
	}

	return cmd.Run()
}

// PasteWithReturn simulates pasting the given text and pressing the return key.
func PasteWithReturn(text string) error {
	// Copy text to clipboard
	if err := CopyToClipboard(text); err != nil {
		return err
	}

	if isX11() {
		// Use robotgo for X11 - paste with Ctrl+V instead of typing each character
		robotgo.KeyTap("v", "ctrl")
		robotgo.KeyTap("enter")
	} else {
		// Wayland implementation using ydotool
		// First press Ctrl+V to paste
		// cmd := exec.Command("ydotool", "key", "29:1", "47:1", "47:0", "29:0") // Ctrl+V
		// cmd.Env = append(os.Environ(), "YDOTOOL_SOCKET="+ydotoolSocket)
		// if err := cmd.Run(); err != nil {
		//     return err
		// }

		// // Then press Enter
		// cmd = exec.Command("ydotool", "key", "28:1", "28:0") // Enter key
		// cmd.Env = append(os.Environ(), "YDOTOOL_SOCKET="+ydotoolSocket)
		// if err := cmd.Run(); err != nil {
		//     return err
		// }
	}

	return nil
}

// isX11 checks if the current session is running X11
func isX11() bool {
	session := os.Getenv("XDG_SESSION_TYPE")
	return strings.ToLower(session) == "x11"
}
