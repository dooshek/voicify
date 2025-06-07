package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
	"github.com/fatih/color"
)

type KeyPress struct {
	Key   string
	Ctrl  bool
	Shift bool
	Alt   bool
	Super bool
}

// Implement types.KeyCombo for KeyPress
func (kp KeyPress) HasCtrl() bool  { return kp.Ctrl }
func (kp KeyPress) HasShift() bool { return kp.Shift }
func (kp KeyPress) HasAlt() bool   { return kp.Alt }
func (kp KeyPress) HasSuper() bool { return kp.Super }
func (kp KeyPress) GetKey() string { return kp.Key }

func RunWizard() error {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)

	bold.Println("\n  Welcome to Voicify Configuration Wizard!")
	fmt.Println("\nThis wizard will help you set up your recording shortcut.")

	for {
		cyan.Println("\nPress your key combination (Ctrl, Alt, Shift, Super + key)...")
		fmt.Println("You can use combinations like Ctrl+Shift+G, Alt+R, Super+V etc.")
		fmt.Println("Only a-z, 0-9, and `[]\\;',./-= keys are allowed.")
		fmt.Println("(Press Ctrl+C to cancel)")

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter your key combination: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			logger.Error("Failed to read input", err)
			return err
		}

		line = strings.TrimSpace(line)
		parts := strings.Split(line, "+")
		var keyPress KeyPress
		for _, part := range parts {
			switch strings.ToLower(part) {
			case "ctrl":
				keyPress.Ctrl = true
			case "shift":
				keyPress.Shift = true
			case "alt":
				keyPress.Alt = true
			case "super":
				keyPress.Super = true
			default:
				keyPress.Key = part
			}
		}

		if keyPress.Key == "" {
			err := fmt.Errorf("no valid key was pressed")
			logger.Error("No valid key was pressed", err)
			return err
		}

		yellow.Print("\nSelected shortcut is: ")
		printKeyCombination(keyPress, false)
		fmt.Println()

		fmt.Print("\nDo you want to use this shortcut? [Y/n]: ")
		reader = bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			logger.Error("Failed to read input", err)
			return err
		}

		// Clean the response more thoroughly
		response = strings.TrimSpace(response)
		response = strings.ToLower(response)
		response = strings.TrimRight(response, "\r\n")
		// Remove any control characters
		response = strings.Map(func(r rune) rune {
			if r < 32 || r == 127 { // ASCII control characters
				return -1
			}
			return r
		}, response)

		confirm := response == "" || response == "y" || response == "yes"

		if !confirm {
			fmt.Println("\nOK, let's try again.")
			continue
		}

		// Default config generator
		config := &types.Config{
			RecordKey: types.KeyBinding{
				Key:   keyPress.Key,
				Ctrl:  keyPress.Ctrl,
				Shift: keyPress.Shift,
				Alt:   keyPress.Alt,
				Super: keyPress.Super,
			},
			LLM: types.LLMConfig{
				Keys: types.LLMKeys{
					OpenAIKey: "",
					GroqKey:   "",
				},
				Transcription: types.LLMTranscription{
					Provider: string(types.ProviderOpenAI),
					Model:    string(types.OpenAIModelWhisper1),
					Language: "en",
				},
				Router: types.LLMRouter{
					Provider:    string(types.ProviderOpenAI),
					Model:       string(types.OpenAIModelGPT4oMini),
					Temperature: 0.2,
				},
			},
		}

		if err := SaveConfig(config); err != nil {
			logger.Error("Failed to save config", err)
			return err
		}

		green.Println("\n Configuration saved successfully!")
		fmt.Print("Your shortcut is: ")
		printKeyCombination(config.RecordKey, false)
		fmt.Println("\nYou can now use this shortcut to start/stop recording.")

		return nil
	}
}

// printKeyCombination prints a key combination in a standardized format.
// If clearLine is true, it will clear the current line before printing.
func printKeyCombination(combo types.KeyCombo, clearLine bool) {
	if clearLine {
		fmt.Print("\033[2K\r") // Clear the line and return carriage
		fmt.Print("Shortcut: ")
	}

	var parts []string
	if combo.HasCtrl() {
		parts = append(parts, "CTRL")
	}
	if combo.HasShift() {
		parts = append(parts, "SHIFT")
	}
	if combo.HasAlt() {
		parts = append(parts, "ALT")
	}
	if combo.HasSuper() {
		parts = append(parts, "SUPER")
	}
	if key := combo.GetKey(); key != "" {
		parts = append(parts, strings.ToUpper(key))
	}
	fmt.Print(strings.Join(parts, " + "))
}
