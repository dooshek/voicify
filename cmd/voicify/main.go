package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dooshek/voicify/internal/config"
	"github.com/dooshek/voicify/internal/fileops"
	"github.com/dooshek/voicify/internal/keyboard"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/notification"
	"github.com/dooshek/voicify/internal/state"
	"github.com/dooshek/voicify/internal/types"
)

func init() {
	// Set custom usage message to show -- prefix
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage of %s:\n", os.Args[0])
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(out, "  --%s", f.Name)
			name, usage := flag.UnquoteUsage(f)
			if len(name) > 0 {
				fmt.Fprintf(out, " %s", name)
			}
			fmt.Fprintf(out, "\n    \t%s", usage)
			if f.DefValue != "" && f.DefValue != "false" {
				fmt.Fprintf(out, " (default %q)", f.DefValue)
			}
			fmt.Fprintf(out, "\n")
		})
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

func main() {
	// Parse command line flags
	runWizard := flag.Bool("wizard", false, "Run the configuration wizard")
	logLevel := flag.String("log-level", "info", "Set log level (debug|info|warn|error)")
	logFilename := flag.String("log-filename", "", "Log to file instead of stdout")
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

	// Update monitor creation to not pass API key
	monitor, err := keyboard.CreateMonitor(state.Get().Config.RecordKey)
	if err != nil {
		logger.Error("Failed to create keyboard monitor", err)
		os.Exit(1)
	}

	startMessage := formatKeyCombo(state.Get().Config.RecordKey)

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

	// Ensure plugins directory exists
	pluginsDir := filepath.Join(fileOps.GetBaseDir(), "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		logger.Error("Failed to create plugins directory", err)
		os.Exit(1)
	}

	// Check if another instance is running
	if err := fileOps.CheckPID(); err != nil {
		if errors.Is(err, fileops.ErrProcessAlreadyRunning) {
			logger.Error("Another instance of Voicify is already running", err)
			os.Exit(1)
		}
		logger.Error("Error checking PID file", err)
		os.Exit(1)
	}

	// Save current PID
	if err := fileOps.SavePID(); err != nil {
		logger.Error("Failed to save PID file", err)
		os.Exit(1)
	}

	// Send system notification
	notifier := notification.New()
	if err := notifier.Notify("🎙️ Voicify started", startMessage); err != nil {
		logger.Warn("Could not send notification")
	}

	// Print to console
	logger.Infof("Press %s to start/stop recording", startMessage)
	logger.Info("💡 Note: You can run `voicify --wizard` to change the key combination")

	// Set up signal handling for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Handle cleanup in a separate goroutine
	go func() {
		sig := <-sigChan
		logger.Infof("Received signal %v, shutting down...", sig)
		if err := fileOps.CleanupPID(); err != nil {
			logger.Error("Failed to cleanup PID file", err)
		}
		os.Exit(0)
	}()

	monitor.Start()
}
