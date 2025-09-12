package plugin

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dooshek/voicify/pkg/pluginapi"
	"github.com/go-vgo/robotgo"
)

// Logger implementation for VSCode plugin
type VSCodeLogger struct{}

var (
	vscodeCurrentLevel LogLevel  = LevelInfo
	vscodeOutput       io.Writer = os.Stdout
	vscodeMu           sync.Mutex
)

// NewVSCodeLogger creates a new logger instance
func NewVSCodeLogger() *VSCodeLogger {
	return &VSCodeLogger{}
}

// Debug logs a debug message
func (l *VSCodeLogger) Debug(message string) {
	if vscodeCurrentLevel <= LevelDebug {
		vscodeWriteLog("DEBUG", message)
	}
}

// Debugf logs a formatted debug message
func (l *VSCodeLogger) Debugf(format string, args ...interface{}) {
	if vscodeCurrentLevel <= LevelDebug {
		vscodeWriteLog("DEBUG", fmt.Sprintf(format, args...))
	}
}

// Info logs an info message
func (l *VSCodeLogger) Info(message string) {
	if vscodeCurrentLevel <= LevelInfo {
		vscodeWriteLog("INFO", message)
	}
}

// Infof logs a formatted info message
func (l *VSCodeLogger) Infof(format string, args ...interface{}) {
	if vscodeCurrentLevel <= LevelInfo {
		vscodeWriteLog("INFO", fmt.Sprintf(format, args...))
	}
}

// Warn logs a warning message
func (l *VSCodeLogger) Warn(message string) {
	if vscodeCurrentLevel <= LevelWarn {
		vscodeWriteLog("WARN", message)
	}
}

// Warnf logs a formatted warning message
func (l *VSCodeLogger) Warnf(format string, args ...interface{}) {
	if vscodeCurrentLevel <= LevelWarn {
		vscodeWriteLog("WARN", fmt.Sprintf(format, args...))
	}
}

// Error logs an error message with an optional error
func (l *VSCodeLogger) Error(message string, err error) {
	if vscodeCurrentLevel <= LevelError {
		errMsg := message
		if err != nil {
			errMsg = fmt.Sprintf("%s: %v", message, err)
		}
		vscodeWriteLog("ERROR", errMsg)
	}
}

// Errorf logs a formatted error message
func (l *VSCodeLogger) Errorf(format string, args ...interface{}) {
	if vscodeCurrentLevel <= LevelError {
		vscodeWriteLog("ERROR", fmt.Sprintf(format, args...))
	}
}

// vscodeWriteLog writes a log message with the specified level
func vscodeWriteLog(level, message string) {
	vscodeMu.Lock()
	defer vscodeMu.Unlock()
	timestamp := time.Now().Format("15:04:05")
	fmt.Fprintf(vscodeOutput, "%s [%s] %s\n", timestamp, level, message)
}

// Window implementation for VSCode plugin
type VSCodeWindow struct {
	Title string
}

// NewVSCodeWindow creates a new window instance
func NewVSCodeWindow() *VSCodeWindow {
	return &VSCodeWindow{}
}

// GetFocusedWindow gets the currently focused window
func (w *VSCodeWindow) GetFocusedWindow() (*VSCodeWindow, error) {
	// Get window ID
	windowID, err := exec.Command("xdotool", "getactivewindow").Output()
	if err != nil {
		return nil, err
	}

	// Get window name
	windowName, err := exec.Command("xdotool", "getwindowname", strings.TrimSpace(string(windowID))).Output()
	if err != nil {
		return nil, err
	}

	return &VSCodeWindow{
		Title: strings.TrimSpace(string(windowName)),
	}, nil
}

// Clipboard implementation for VSCode plugin
type VSCodeClipboard struct{}

// NewVSCodeClipboard creates a new clipboard instance
func NewVSCodeClipboard() *VSCodeClipboard {
	return &VSCodeClipboard{}
}

// CopyToClipboard copies text to the clipboard
func (c *VSCodeClipboard) CopyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		// macOS
		cmd = exec.Command("pbcopy")
	case "linux":
		// Linux
		if _, err := exec.LookPath("xclip"); err != nil {
			return fmt.Errorf("xclip is not installed")
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

// PasteWithReturn pastes text and adds a newline
func (c *VSCodeClipboard) PasteWithReturn(text string) error {
	// Copy text to clipboard
	if err := c.CopyToClipboard(text); err != nil {
		return err
	}

	if vscodeIsX11() {
		// Use robotgo for X11
		robotgo.KeyTap("v", "ctrl")
		robotgo.KeyTap("enter")
	} else {
		// Wayland: Use XWayland compatibility layer (most reliable on Fedora)
		cmd := exec.Command("xdotool", "key", "ctrl+v")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to paste: %v", err)
		}

		// Press Enter
		time.Sleep(50 * time.Millisecond)
		cmd = exec.Command("xdotool", "key", "Return")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to press enter: %v", err)
		}
	}

	return nil
}

// vscodeIsX11 checks if the current session is running X11
func vscodeIsX11() bool {
	session := os.Getenv("XDG_SESSION_TYPE")
	return strings.ToLower(session) == "x11"
}

// VSCodePlugin is a plugin for VSCode
type VSCodePlugin struct{}

// VSCodeAction is the VSCode action
type VSCodeAction struct {
	transcription string
}

// Initialize initializes the VSCode plugin
func (p *VSCodePlugin) Initialize() error {
	vscodeLogger.Debug("VSCode plugin initialized")
	return nil
}

// GetMetadata returns metadata about the plugin
func (p *VSCodePlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "vscode",
		Version:     "1.0.0",
		Description: "Plugin for Visual Studio Code",
		Author:      "Voicify Team",
	}
}

// GetActions returns a list of actions provided by this plugin
func (p *VSCodePlugin) GetActions(transcription string) []pluginapi.PluginAction {
	return []pluginapi.PluginAction{
		&VSCodeAction{transcription: transcription},
	}
}

// Execute executes the VSCode action
func (a *VSCodeAction) Execute(transcription string) error {
	vscodeLogger.Debugf("Checking if VSCode should execute action for transcription: %s", transcription)
	window := NewVSCodeWindow()
	focusedWindow, err := window.GetFocusedWindow()
	if err != nil {
		vscodeLogger.Error("Error getting focused window", err)
		return err
	}

	vscodeLogger.Debugf("Checking window title: %s", focusedWindow.Title)
	if !strings.Contains(focusedWindow.Title, "VSC") {
		vscodeLogger.Debug("VSCode is not open, skipping action")
		return nil
	}

	clipboard := NewVSCodeClipboard()
	clipboard.PasteWithReturn(transcription)

	return nil
}

// GetMetadata returns metadata about the action
func (a *VSCodeAction) GetMetadata() pluginapi.ActionMetadata {
	return pluginapi.ActionMetadata{
		Name:        "vscode",
		Description: "wykonanie akcji w edytorze VSCode",
		Priority:    2,
	}
}

// NewVSCodePlugin creates a new instance of the VSCode plugin
func NewVSCodePlugin() pluginapi.VoicifyPlugin {
	return &VSCodePlugin{}
}

var vscodeLogger = NewVSCodeLogger()
