package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
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

	// Parse the main command
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

	// Check if we're running a plugin subcommand
	if len(os.Args) > 1 && os.Args[1] == "plugin" {
		// Define plugin command related flags
		pluginCmd := flag.NewFlagSet("plugin", flag.ExitOnError)
		pluginInstall := pluginCmd.String("install", "", "Install a plugin from a directory")
		pluginRemove := pluginCmd.String("remove", "", "Remove an installed plugin")
		pluginList := pluginCmd.Bool("list", false, "List installed plugins")
		installRepo := pluginCmd.String("install-repo", "", "Install a plugin from a git repository (repo must have main.go in root)")

		err := handlePluginCommand(pluginCmd, os.Args[2:], pluginInstall, pluginRemove, pluginList, installRepo)
		if err != nil {
			logger.Error("Plugin command failed", err)
			os.Exit(1)
		}
		os.Exit(0)
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
	pluginsDir := fileOps.GetPluginsDir()
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

// handlePluginCommand implements plugin management functionality
func handlePluginCommand(pluginCmd *flag.FlagSet, args []string, installPath *string, removeName *string, list *bool, installRepo *string) error {
	// Parse the plugin command flags
	if err := pluginCmd.Parse(args); err != nil {
		return err
	}

	fileOps, err := fileops.NewDefaultFileOps()
	if err != nil {
		return fmt.Errorf("failed to initialize file operations: %w", err)
	}

	// Ensure the plugins directory exists
	pluginsDir := fileOps.GetPluginsDir()
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Handle plugin list
	if *list {
		return listPlugins(pluginsDir)
	}

	// Handle plugin installation
	if *installPath != "" {
		return installPlugin(*installPath, pluginsDir)
	}

	// Handle plugin installation from git repo
	if *installRepo != "" {
		return installPluginFromRepo(*installRepo, pluginsDir)
	}

	// Handle plugin removal
	if *removeName != "" {
		return removePlugin(*removeName, pluginsDir)
	}

	// If no operation specified, show usage
	fmt.Println("Plugin management commands:")
	fmt.Println("  voicify plugin --list                  List installed plugins")
	fmt.Println("  voicify plugin --install <dir>         Install a plugin from directory")
	fmt.Println("  voicify plugin --install-repo <url>    Install a plugin from git repository (repo must have main.go in root)")
	fmt.Println("  voicify plugin --remove <plugin-name>  Remove a plugin")
	return nil
}

// listPlugins lists all installed plugins
func listPlugins(pluginsDir string) error {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No plugins directory found.")
			return nil
		}
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No plugins installed.")
		return nil
	}

	fmt.Println("Installed plugins:")
	for _, entry := range entries {
		if entry.IsDir() {
			mainSoPath := filepath.Join(pluginsDir, entry.Name(), "main.so")
			status := "✅ "
			if _, err := os.Stat(mainSoPath); os.IsNotExist(err) {
				status = "❌ (missing main.so) "
			}
			fmt.Printf("  %s%s\n", status, entry.Name())
		}
	}
	return nil
}

// installPlugin installs a plugin from the specified directory
func installPlugin(srcDir string, pluginsDir string) error {
	// Check if source directory exists
	srcInfo, err := os.Stat(srcDir)
	if err != nil {
		return fmt.Errorf("failed to access source directory: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source path is not a directory: %s", srcDir)
	}

	// Get plugin name from directory name
	pluginName := filepath.Base(srcDir)
	destDir := filepath.Join(pluginsDir, pluginName)

	// Check if plugin is already installed
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("plugin %s is already installed", pluginName)
	}

	// Create plugin directory
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Check if main.so exists in source directory
	mainSoPath := filepath.Join(srcDir, "main.so")
	if _, err := os.Stat(mainSoPath); os.IsNotExist(err) {
		// Try to build the plugin
		fmt.Printf("Building plugin %s...\n", pluginName)
		if err := buildPlugin(srcDir); err != nil {
			return fmt.Errorf("failed to build plugin: %w", err)
		}
	}

	// Copy main.so to plugin directory
	mainSoData, err := os.ReadFile(mainSoPath)
	if err != nil {
		return fmt.Errorf("failed to read plugin file: %w", err)
	}

	destPath := filepath.Join(destDir, "main.so")
	if err := os.WriteFile(destPath, mainSoData, 0o644); err != nil {
		return fmt.Errorf("failed to write plugin file: %w", err)
	}

	fmt.Printf("✅ Plugin %s installed successfully\n", pluginName)
	return nil
}

// removePlugin removes an installed plugin
func removePlugin(pluginName string, pluginsDir string) error {
	pluginDir := filepath.Join(pluginsDir, pluginName)

	// Check if plugin exists
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		return fmt.Errorf("plugin %s is not installed", pluginName)
	}

	// Remove plugin directory
	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin directory: %w", err)
	}

	fmt.Printf("✅ Plugin %s removed successfully\n", pluginName)
	return nil
}

// buildPlugin attempts to build a plugin in the specified directory
func buildPlugin(srcDir string) error {
	// Change to the source directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(currentDir)

	if err := os.Chdir(srcDir); err != nil {
		return fmt.Errorf("failed to change to source directory: %w", err)
	}

	// Run go build
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", "main.so", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// installPluginFromRepo clones a git repository and installs the plugin
func installPluginFromRepo(repoURL string, pluginsDir string) error {
	// Create a temporary directory for the repository
	tempDir, err := os.MkdirTemp("", "voicify-plugin-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone the repository
	fmt.Printf("Cloning repository %s...\n", repoURL)
	cmd := exec.Command("git", "clone", repoURL, tempDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Get the plugin name from the repository URL
	repoName := filepath.Base(repoURL)
	if strings.HasSuffix(repoName, ".git") {
		repoName = repoName[:len(repoName)-4]
	}

	// Check if main.go exists in the root directory
	mainGoPath := filepath.Join(tempDir, "main.go")
	if _, err := os.Stat(mainGoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository does not contain a main.go file in the root directory - not a valid plugin")
	}

	// Install the plugin from the cloned repository
	fmt.Printf("Installing plugin from %s...\n", repoName)
	return installPlugin(tempDir, pluginsDir)
}
