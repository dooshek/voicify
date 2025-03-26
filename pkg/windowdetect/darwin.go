package windowdetect

import (
	"os/exec"
	"strings"
)

type darwinDetector struct{}

func newDarwinDetector() platformDetector {
	return &darwinDetector{}
}

func (d *darwinDetector) getFocusedWindow() (*WindowInfo, error) {
	script := `
		tell application "System Events"
			set frontApp to first application process whose frontmost is true
			set appName to name of frontApp
			set windowTitle to ""
			try
				set windowTitle to name of first window of frontApp
			end try
			return appName & "|" & windowTitle
		end tell
	`

	output, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return nil, err
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	info := &WindowInfo{
		AppName: parts[0],
		Title:   "",
	}

	if len(parts) > 1 {
		info.Title = parts[1]
	}

	return info, nil
}
