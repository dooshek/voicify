package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dooshek/voicify/internal/config"
	"github.com/dooshek/voicify/internal/dbus"
	"github.com/dooshek/voicify/internal/fileops"
	"github.com/dooshek/voicify/internal/keyboard"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/notification"
	"github.com/dooshek/voicify/internal/plugin/linear"
	"github.com/dooshek/voicify/internal/state"
	"github.com/dooshek/voicify/internal/tts"
	"github.com/dooshek/voicify/internal/types"
)

func init() {
	// Set custom usage message to show -- prefix and all commands
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Voicify - Voice-controlled text automation\n\n")
		fmt.Fprintf(out, "USAGE:\n")
		fmt.Fprintf(out, "  voicify [OPTIONS]\n")
		fmt.Fprintf(out, "\n")

		fmt.Fprintf(out, "COMMANDS:\n")
		fmt.Fprintf(out, "  (default)    Start voice recording with keyboard monitoring\n")
		fmt.Fprintf(out, "\n")

		fmt.Fprintf(out, "OPTIONS:\n")
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(out, "  --%s", f.Name)
			name, usage := flag.UnquoteUsage(f)
			if len(name) > 0 {
				fmt.Fprintf(out, " %s", name)
			}
			fmt.Fprintf(out, "\n        %s", usage)
			if f.DefValue != "" && f.DefValue != "false" {
				fmt.Fprintf(out, " (default %q)", f.DefValue)
			}
			fmt.Fprintf(out, "\n")
		})

		fmt.Fprintf(out, "\nEXAMPLES:\n")
		fmt.Fprintf(out, "  voicify                                 Start voicify with keyboard monitoring\n")
		fmt.Fprintf(out, "  voicify --daemon                        Start D-Bus daemon (for GNOME extension)\n")
		fmt.Fprintf(out, "  voicify --wizard                        Run configuration wizard\n")
		fmt.Fprintf(out, "  voicify --log-level debug               Start with debug logging\n")
		fmt.Fprintf(out, "\n")
	}
}

// formatKeyCombo formats a key combination into a human-readable string
func formatKeyCombo(cfg types.KeyBinding) string {
	var parts []string
	if cfg.Ctrl {
		parts = append(parts, "CTRL")
	}
	if cfg.Shift {
		parts = append(parts, "SHIFT")
	}
	if cfg.Alt {
		parts = append(parts, "ALT")
	}
	if cfg.Super {
		parts = append(parts, "SUPER")
	}
	if key := cfg.Key; key != "" {
		parts = append(parts, strings.ToUpper(key))
	}
	return strings.Join(parts, " + ")
}

// initializeGlobalLinearMCP initializes global Linear MCP client
func initializeGlobalLinearMCP() error {
	client, err := linear.NewLinearMCPClient()
	if err != nil {
		return fmt.Errorf("failed to create Linear MCP client: %w", err)
	}

	// Store in global state
	state.Get().SetLinearMCPClient(client)
	return nil
}

func main() {
	// Parse command line flags
	runWizard := flag.Bool("wizard", false, "Run the configuration wizard")
	daemonMode := flag.Bool("daemon", false, "Run as D-Bus daemon (for GNOME extension integration)")
	logLevel := flag.String("log-level", "info", "Set log level (debug|info|warn|error)")
	logFilename := flag.String("log-filename", "", "Log to file instead of stdout")

	// Parse the global flags for non-plugin commands
	flag.Parse()

	// Set up logging level and output
	logger.SetLevel(*logLevel)
	if *logFilename != "" {
		if err := logger.SetOutputFile(*logFilename); err != nil {
			fmt.Printf("Error setting log file: %v\n", err)
			os.Exit(1)
		}
		defer logger.CloseLogFile()
	}

	var cfg *types.Config
	var err error

	if *runWizard {
		err = config.RunWizard()
		if err != nil {
			logger.Error("Error running wizard", err)
			os.Exit(1)
		}
		os.Exit(0)
	} else {
		cfg, err = config.LoadConfig()
		if err != nil {
			logger.Error("Error loading config", err)
			os.Exit(1)
		}

		if cfg == nil {
			logger.Info("No configuration found. Running setup wizard...")
			err = config.RunWizard()
			if err != nil {
				logger.Error("Error running wizard", err)
				os.Exit(1)
			}
		}
	}

	// Initialize global state with the entire config
	state.Init(cfg)

	// Initialize TTS manager if configuration is available
	var ttsManager *tts.Manager
	if cfg.LLM.Keys.OpenAIKey != "" {
		ttsConfig := cfg.GetTTSConfig()
		var err error
		ttsManager, err = tts.NewManager(ttsConfig, cfg.LLM.Keys.OpenAIKey)
		if err != nil {
			logger.Warnf("Failed to initialize TTS manager: %v", err)
			logger.Infof("TTS functionality will be disabled. Check your OpenAI API key configuration.")
		} else {
			logger.Infof("✨ TTS initialized: %s with voice '%s'", ttsManager.GetProviderName(), ttsConfig.Voice)
			// Add TTS manager to global state
			state.Get().SetTTSManager(ttsManager)
		}
	} else {
		logger.Debugf("TTS disabled: No OpenAI API key configured")
	}

	// Pre-initialize Linear MCP client asynchronously to avoid blocking startup
	go func() {
		logger.Debug("Pre-initializing Linear MCP client...")
		if err := initializeGlobalLinearMCP(); err != nil {
			logger.Warnf("Failed to pre-initialize Linear MCP client: %v", err)
			logger.Info("Linear MCP will be initialized on first use")
		} else {
			logger.Debug("Linear MCP client pre-initialized successfully")
		}
	}()

	var startMessage string
	var monitor keyboard.KeyboardMonitor
	var dbusServer *dbus.Server

	if *daemonMode {
		// D-Bus daemon mode
		dbusServer, err = dbus.NewServer()
		if err != nil {
			logger.Error("Failed to create D-Bus server", err)
			os.Exit(1)
		}
		startMessage = "D-Bus daemon"
	} else {
		// Keyboard monitoring mode
		monitor, err = keyboard.CreateMonitor(state.Get().Config.RecordKey)
		if err != nil {
			logger.Error("Failed to create keyboard monitor", err)
			os.Exit(1)
		}
		startMessage = formatKeyCombo(state.Get().Config.RecordKey)
	}

	// Initialize fileops
	fileOps, err := fileops.NewDefaultFileOps()
	if err != nil {
		logger.Error("Failed to initialize file operations", err)
		os.Exit(1)
	}

	// Ensure directories exist
	if err := fileOps.EnsureDirectories(); err != nil {
		logger.Error("Failed to create necessary directories", err)
		os.Exit(1)
	}

	// Check if another instance is running
	if err := fileOps.CheckPID(); err != nil {
		if errors.Is(err, fileops.ErrProcessAlreadyRunning) {
			logger.Error("Another instance of Voicify is already running", err)
			os.Exit(1)
		}
	}

	// Save current PID
	if err := fileOps.SavePID(); err != nil {
		logger.Error("Failed to save PID file", err)
		os.Exit(1)
	}

	// Print to console
	if *daemonMode {
		logger.Infof("🔌 D-Bus daemon started: %s", startMessage)
		logger.Info("💡 GNOME extension can now communicate with voicify")
	} else {
		// Send system notification
		notifier := notification.New()
		if err := notifier.Notify("🎙️ Voicify started", startMessage); err != nil {
			logger.Warn("Could not send notification")
		}
		logger.Infof("Press %s to start/stop recording", startMessage)
		logger.Info("💡 Note: You can run `voicify --wizard` to change the key combination")
	}

	// Create context for clean shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Handle cleanup in a separate goroutine
	go func() {
		sig := <-sigChan
		logger.Infof("Received signal %v, shutting down...", sig)
		cancel() // Cancel context

		// Clean up based on mode
		if *daemonMode && dbusServer != nil {
			dbusServer.Stop()
		}

		// Cleanup Linear MCP client
		if mcpClient := state.Get().GetLinearMCPClient(); mcpClient != nil {
			if client, ok := mcpClient.(*linear.LinearMCPClient); ok {
				client.Close()
			}
		}

		if err := fileOps.CleanupPID(); err != nil {
			logger.Error("Failed to cleanup PID file", err)
		}
		os.Exit(0)
	}()

	// Start service based on mode
	if *daemonMode {
		// Start D-Bus server
		if err := dbusServer.Start(); err != nil {
			logger.Error("Failed to start D-Bus server", err)
			os.Exit(1)
		}
		// Wait for server to be stopped
		dbusServer.Wait()
	} else {
		// Start keyboard monitoring in a goroutine
		go func() {
			if err := monitor.Start(ctx); err != nil {
				logger.Error("Keyboard monitor failed", err)
			}
		}()
		// Block and wait for shutdown signal
		<-ctx.Done()
	}

	logger.Infof("Shutting down...")
}
