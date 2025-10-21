package pluginapi

import (
	"fmt"

	"github.com/dooshek/voicify/internal/state"
)

// DBusServer interface to avoid import cycle
type DBusServer interface {
	EmitRequestPaste(text string) error
}

// RequestPaste requests the extension to paste the given text
// This should be called by plugins when they want to insert text into the focused window
func RequestPaste(text string) error {
	server := state.Get().GetDBusServer()
	if server == nil {
		return fmt.Errorf("D-Bus server not available")
	}

	dbusServer, ok := server.(DBusServer)
	if !ok {
		return fmt.Errorf("invalid D-Bus server type")
	}

	return dbusServer.EmitRequestPaste(text)
}
