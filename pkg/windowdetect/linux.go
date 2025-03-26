package windowdetect

import (
	"fmt"
	"os/exec"
	"strings"
)

type linuxDetector struct{}

func newLinuxDetector() platformDetector {
	if err := checkXdotool(); err != nil {
		// Return nil so higher layers can handle the error
		return nil
	}
	return &linuxDetector{}
}

// checkXdotool checks if xdotool is installed
func checkXdotool() error {
	_, err := exec.LookPath("xdotool")
	if err != nil {
		return fmt.Errorf("xdotool is not installed - window detection will not work. Install it using:\n" +
			"Fedora: sudo dnf install xdotool\n" +
			"Ubuntu/Debian: sudo apt-get install xdotool\n" +
			"Arch Linux: sudo pacman -S xdotool")
	}
	return nil
}

func (d *linuxDetector) getFocusedWindow() (*WindowInfo, error) {
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

	// Get window class (app name)
	windowClass, err := exec.Command("xdotool", "getwindowclassname", strings.TrimSpace(string(windowID))).Output()
	if err != nil {
		return nil, err
	}

	return &WindowInfo{
		Title:   strings.TrimSpace(string(windowName)),
		AppName: strings.TrimSpace(string(windowClass)),
	}, nil
}
