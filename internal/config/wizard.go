package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/MarinX/keylogger"
	"github.com/dooshek/voicify/internal/keyboard"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
	"github.com/fatih/color"
	hook "github.com/robotn/gohook"
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

	bold.Println("\n🎙️  Welcome to Voicify Configuration Wizard!")
	fmt.Println("\nThis wizard will help you set up your recording shortcut.")

	var keyPress KeyPress
	var err error

	for {
		cyan.Println("\nPress your key combination (Ctrl, Alt, Shift, Super + key)...")
		fmt.Println("You can use combinations like Ctrl+Shift+G, Alt+R, Super+V etc.")
		fmt.Println("Only a-z, 0-9, and `[]\\;',./-= keys are allowed.")
		fmt.Println("(Press Ctrl+C to cancel)")

		keyPress, err = captureKeys()
		if err != nil {
			logger.Error("Failed to capture key", err)
			return err
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
		reader := bufio.NewReader(os.Stdin)
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

		green.Println("\n✅ Configuration saved successfully!")
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

func captureKeys() (KeyPress, error) {
	if strings.ToLower(os.Getenv("XDG_SESSION_TYPE")) == "x11" {
		return captureX11Keys()
	}
	return captureWaylandKeys()
}

func captureX11Keys() (KeyPress, error) {
	evChan := hook.Start()
	defer hook.End()

	var keyPress KeyPress
	done := make(chan bool)
	modifierUpdates := make(chan bool)

	// Goroutine to handle modifier updates
	go func() {
		for range modifierUpdates {
			printKeyCombination(keyPress, true)
		}
	}()

	// Main event loop
	go func() {
		for ev := range evChan {
			code := uint16(ev.Rawcode)

			if ev.Kind == hook.KeyHold || ev.Kind == hook.KeyDown {
				switch code {
				case keyboard.X11LeftControl, keyboard.X11RightControl:
					keyPress.Ctrl = true
					modifierUpdates <- true
				case keyboard.X11LeftShift, keyboard.X11RightShift:
					keyPress.Shift = true
					modifierUpdates <- true
				case keyboard.X11LeftAlt, keyboard.X11RightAlt:
					keyPress.Alt = true
					modifierUpdates <- true
				case keyboard.X11Super:
					keyPress.Super = true
					modifierUpdates <- true
				default:
					// Try checking using X11KeyMap lookup
					if key, ok := keyboard.X11KeyMap[code]; ok {
						keyPress.Key = key
						done <- true
						return
					}

					// Try using ASCII char value as fallback
					if ev.Keychar >= 32 && ev.Keychar <= 126 {
						keyPress.Key = string(ev.Keychar)
						done <- true
						return
					}

					// Last resort fallback - for alphabetic keys
					if code >= 97 && code <= 122 {
						// ASCII codes for a-z
						keyPress.Key = string(rune(code))
						done <- true
						return
					}
				}
			} else if ev.Kind == hook.KeyUp {
				switch code {
				case keyboard.X11LeftControl, keyboard.X11RightControl:
					keyPress.Ctrl = false
					modifierUpdates <- true
				case keyboard.X11LeftShift, keyboard.X11RightShift:
					keyPress.Shift = false
					modifierUpdates <- true
				case keyboard.X11LeftAlt, keyboard.X11RightAlt:
					keyPress.Alt = false
					modifierUpdates <- true
				case keyboard.X11Super:
					keyPress.Super = false
					modifierUpdates <- true
				}
			}
		}
	}()

	<-done
	close(modifierUpdates)
	return keyPress, nil
}

func captureWaylandKeys() (KeyPress, error) {
	keyboards := keylogger.FindAllKeyboardDevices()
	if len(keyboards) == 0 {
		err := fmt.Errorf("no keyboard devices found")
		logger.Error("No keyboard devices found", err)
		return KeyPress{}, err
	}

	kbd, err := keylogger.New(keyboards[0])
	if err != nil {
		err = fmt.Errorf("failed to initialize keylogger: %w", err)
		logger.Error("Failed to initialize keylogger", err)
		return KeyPress{}, err
	}
	defer kbd.Close()

	events := kbd.Read()
	var keyPress KeyPress
	done := make(chan bool)
	modifierUpdates := make(chan bool)

	// Goroutine to handle modifier updates
	go func() {
		for range modifierUpdates {
			printKeyCombination(keyPress, true)
		}
	}()

	// Main event loop
	go func() {
		for e := range events {
			if e.Type == keylogger.EvKey {
				code := uint16(e.Code)

				if e.KeyPress() {
					switch code {
					case keyboard.WaylandLeftControl, keyboard.WaylandRightControl:
						keyPress.Ctrl = true
						modifierUpdates <- true
					case keyboard.WaylandLeftShift, keyboard.WaylandRightShift:
						keyPress.Shift = true
						modifierUpdates <- true
					case keyboard.WaylandLeftAlt, keyboard.WaylandRightAlt:
						keyPress.Alt = true
						modifierUpdates <- true
					case keyboard.WaylandSuper:
						keyPress.Super = true
						modifierUpdates <- true
					default:
						// Bezpośrednie sprawdzenie z mapy wayland
						if key, ok := keyboard.WaylandKeyMap[code]; ok {
							keyPress.Key = key
							done <- true
							return
						}

					}
				} else if e.KeyRelease() {
					switch code {
					case keyboard.WaylandLeftControl, keyboard.WaylandRightControl:
						keyPress.Ctrl = false
						modifierUpdates <- true
					case keyboard.WaylandLeftShift, keyboard.WaylandRightShift:
						keyPress.Shift = false
						modifierUpdates <- true
					case keyboard.WaylandLeftAlt, keyboard.WaylandRightAlt:
						keyPress.Alt = false
						modifierUpdates <- true
					case keyboard.WaylandSuper:
						keyPress.Super = false
						modifierUpdates <- true
					}
				}
			}
		}
	}()

	<-done
	close(modifierUpdates)
	return keyPress, nil
}
