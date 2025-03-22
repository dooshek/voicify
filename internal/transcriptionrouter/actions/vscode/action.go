package vscode

import (
	"strings"

	"github.com/dooshek/voicify/internal/clipboard"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
	"github.com/dooshek/voicify/internal/windowdetect"
)

type PluginAction struct {
	transcription string
}

func New(transcription string) *PluginAction {
	return &PluginAction{transcription: transcription}
}

func (a *PluginAction) Name() string {
	return "vscode"
}

func (a *PluginAction) Execute(transcription string) error {
	logger.Debugf("Checking if VSCode should execute action for transcription: %s", transcription)
	windowDetector, err := windowdetect.New()
	if err != nil {
		logger.Error("Error getting window detector", err)
		return err
	}
	window, err := windowDetector.GetFocusedWindow()
	if err != nil {
		logger.Error("Error getting focused window", err)
		return err
	}

	logger.Debugf("Checking window title: %s", window.Title)
	if !strings.Contains(window.Title, "VSC") {
		logger.Debug("VSCode is not open, skipping action")
		return nil
	}

	clipboard.PasteWithReturn(transcription)

	return nil
}

func (a *PluginAction) GetMetadata() types.ActionMetadata {
	return types.ActionMetadata{
		Name:        "vscode",
		Description: "wykonanie akcji w edytorze VSCode",
		Priority:    2,
	}
}
