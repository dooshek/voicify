package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"plugin"
	"reflect"
	"strings"
	"syscall"

	"github.com/dooshek/voicify/internal/config"
	"github.com/dooshek/voicify/internal/fileops"
	"github.com/dooshek/voicify/internal/keyboard"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/notification"
	"github.com/dooshek/voicify/internal/state"
	"github.com/dooshek/voicify/internal/types"
	"github.com/dooshek/voicify/pkg/pluginapi"
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

	// Check if we're running a plugin subcommand before parsing global flags
	if len(os.Args) > 1 && os.Args[1] == "plugin" {
		// Define plugin command related flags
		pluginCmd := flag.NewFlagSet("plugin", flag.ExitOnError)

		// Add the same global flags to the plugin command
		pluginLogLevel := pluginCmd.String("log-level", "info", "Set log level (debug|info|warn|error)")
		pluginLogFilename := pluginCmd.String("log-filename", "", "Log to file instead of stdout")

		// Add plugin-specific flags
		pluginInstall := pluginCmd.String("install", "", "Install a plugin from a directory")
		pluginRemove := pluginCmd.String("remove", "", "Remove an installed plugin")
		pluginList := pluginCmd.Bool("list", false, "List installed plugins")
		installRepo := pluginCmd.String("install-repo", "", "Install a plugin from a git repository (repo must have main.go in root)")
		cleanup := pluginCmd.Bool("cleanup", false, "Clean up invalid plugin installations")

		// Parse the plugin command flags
		if err := pluginCmd.Parse(os.Args[2:]); err != nil {
			fmt.Printf("Error parsing plugin flags: %v\n", err)
			os.Exit(1)
		}

		// Set up logging level and output
		logger.SetLevel(*pluginLogLevel)
		if *pluginLogFilename != "" {
			if err := logger.SetOutputFile(*pluginLogFilename); err != nil {
				fmt.Printf("Error setting log file: %v\n", err)
				os.Exit(1)
			}
			defer logger.CloseLogFile()
		}

		// Handle the plugin command
		err := handlePluginCommand(pluginCmd, pluginInstall, pluginRemove, pluginList, installRepo, cleanup)
		if err != nil {
			logger.Error("Plugin command failed", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

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
func handlePluginCommand(pluginCmd *flag.FlagSet, installPath *string, removeName *string, list *bool, installRepo *string, cleanup *bool) error {
	fileOps, err := fileops.NewDefaultFileOps()
	if err != nil {
		return fmt.Errorf("failed to initialize file operations: %w", err)
	}

	// Ensure the plugins directory exists
	pluginsDir := fileOps.GetPluginsDir()
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Handle plugin cleanup option
	if *cleanup {
		return cleanupPlugins(pluginsDir)
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
	fmt.Println("  voicify plugin --cleanup               Clean up invalid plugin installations")
	return nil
}

// listPlugins lists all installed plugins
func listPlugins(pluginsDir string) error {
	fmt.Println("⏳ Listing installed plugins...")

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("ℹ️ No plugins directory found.")
			return nil
		}
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("ℹ️ No plugins installed.")
		return nil
	}

	// Count valid and invalid plugins
	validCount := 0
	invalidCount := 0

	fmt.Println("📋 Installed plugins:")
	for _, entry := range entries {
		if entry.IsDir() {
			mainSoPath := filepath.Join(pluginsDir, entry.Name(), "main.so")
			if _, err := os.Stat(mainSoPath); os.IsNotExist(err) {
				fmt.Printf("  ❌ %s (missing main.so)\n", entry.Name())
				invalidCount++
			} else if isTemporaryDirectory(entry.Name()) {
				fmt.Printf("  ⚠️ %s (temporary plugin directory)\n", entry.Name())
				invalidCount++
			} else {
				fmt.Printf("  ✅ %s\n", entry.Name())
				validCount++
			}
		}
	}

	// Print summary
	fmt.Printf("\nSummary: %d valid plugin(s), %d invalid plugin(s)\n", validCount, invalidCount)
	if invalidCount > 0 {
		fmt.Println("You can run 'voicify plugin --cleanup' to remove invalid plugin installations.")
	}

	return nil
}

// isTemporaryDirectory checks if a directory name appears to be a temporary directory
func isTemporaryDirectory(name string) bool {
	// Check for common patterns in temporary directory names
	return strings.HasPrefix(name, "voicify-plugin-") && strings.Contains(name, "-")
}

// cleanupPlugins removes invalid plugin installations
func cleanupPlugins(pluginsDir string) error {
	fmt.Println("⏳ Cleaning up invalid plugin installations...")

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("ℹ️ No plugins directory found.")
			return nil
		}
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("ℹ️ No plugins to clean up.")
		return nil
	}

	// Count removed directories
	removedCount := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(pluginsDir, entry.Name())
		mainSoPath := filepath.Join(pluginDir, "main.so")

		// Check if it's a valid plugin installation
		isValid := true

		// Check for missing main.so
		if _, err := os.Stat(mainSoPath); os.IsNotExist(err) {
			isValid = false
			fmt.Printf("  🗑️ Removing %s (missing main.so)...\n", entry.Name())
		}

		// Check for temporary directory
		if isTemporaryDirectory(entry.Name()) {
			isValid = false
			fmt.Printf("  🗑️ Removing %s (temporary plugin directory)...\n", entry.Name())
		}

		// Remove invalid installation
		if !isValid {
			if err := os.RemoveAll(pluginDir); err != nil {
				fmt.Printf("  ❌ Failed to remove %s: %v\n", entry.Name(), err)
			} else {
				fmt.Printf("  ✅ Removed %s\n", entry.Name())
				removedCount++
			}
		}
	}

	fmt.Printf("\nCleanup complete: Removed %d invalid plugin installation(s).\n", removedCount)
	return nil
}

// checkPluginCompatibility verifies that a plugin is compatible with the current version of Voicify
func checkPluginCompatibility(mainSoPath string) error {
	// This is a placeholder for a more sophisticated version check
	// In a real implementation, we would load the plugin and check its API version

	// For now, just check if the file exists and is a valid shared object file
	info, err := os.Stat(mainSoPath)
	if err != nil {
		return fmt.Errorf("plugin file not found: %w", err)
	}

	if info.Size() == 0 {
		return fmt.Errorf("plugin file is empty")
	}

	// In a future version, we could implement a more sophisticated check:
	// 1. Try to load the plugin with plugin.Open()
	// 2. Look for a Version() function
	// 3. Check the returned version against the application's expected version

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

	fmt.Printf("⏳ Installing plugin %s...\n", pluginName)

	// Create plugin directory
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Check if main.so exists in source directory
	mainSoPath := filepath.Join(srcDir, "main.so")
	if _, err := os.Stat(mainSoPath); os.IsNotExist(err) {
		// Try to build the plugin
		if err := buildPlugin(srcDir); err != nil {
			// Clean up the destination directory on build failure
			os.RemoveAll(destDir)
			return fmt.Errorf("failed to build plugin: %w", err)
		}
	}

	// Check plugin compatibility
	if err := checkPluginCompatibility(mainSoPath); err != nil {
		// Clean up the destination directory if incompatible
		os.RemoveAll(destDir)
		return fmt.Errorf("plugin compatibility check failed: %w", err)
	}

	// Copy main.so to plugin directory
	mainSoData, err := os.ReadFile(mainSoPath)
	if err != nil {
		// Clean up on failure
		os.RemoveAll(destDir)
		return fmt.Errorf("failed to read plugin file: %w", err)
	}

	destPath := filepath.Join(destDir, "main.so")
	if err := os.WriteFile(destPath, mainSoData, 0o644); err != nil {
		// Clean up on failure
		os.RemoveAll(destDir)
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

	fmt.Printf("⏳ Removing plugin %s...\n", pluginName)

	// Remove plugin directory
	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin directory: %w", err)
	}

	fmt.Printf("✅ Plugin %s removed successfully\n", pluginName)
	return nil
}

// buildPlugin attempts to build a plugin in the specified directory
func buildPlugin(srcDir string) error {
	fmt.Println("⏳ Building plugin...")

	// Change to the source directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer func() {
		if err := os.Chdir(currentDir); err != nil {
			fmt.Printf("Warning: Failed to change back to original directory: %v\n", err)
		}
	}()

	if err := os.Chdir(srcDir); err != nil {
		return fmt.Errorf("failed to change to source directory: %w", err)
	}

	// Create a buffer to capture stdout and stderr
	var stderrBuf strings.Builder

	// Run go build
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", "main.so", ".")
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		errMsg := stderrBuf.String()
		if errMsg != "" {
			fmt.Println("❌ Build error:")
			fmt.Println(errMsg)
		}
		return fmt.Errorf("failed to build plugin: %w", err)
	}

	fmt.Println("✅ Build completed")
	return nil
}

// installPluginFromRepo clones a git repository and installs the plugin
func installPluginFromRepo(repoURL string, pluginsDir string) error {
	// Normalize the repository URL
	// Add https:// prefix if no protocol is specified
	if !strings.HasPrefix(repoURL, "https://") && !strings.HasPrefix(repoURL, "http://") && !strings.HasPrefix(repoURL, "git@") && !strings.HasPrefix(repoURL, "ssh://") {
		repoURL = "https://" + repoURL
	}

	// Add .git suffix if not present
	if !strings.HasSuffix(repoURL, ".git") {
		repoURL = repoURL + ".git"
	}

	// Extract the repository name from the URL
	repoName := filepath.Base(repoURL)
	if strings.HasSuffix(repoName, ".git") {
		repoName = repoName[:len(repoName)-4]
	}

	// Define local plugins directory in the current working directory
	localPluginsDir := "./plugins"

	// Check if the local plugins directory exists, and if so, delete it
	if _, err := os.Stat(localPluginsDir); err == nil {
		fmt.Printf("⏳ Removing existing %s directory...\n", localPluginsDir)
		if err := os.RemoveAll(localPluginsDir); err != nil {
			return fmt.Errorf("failed to remove existing plugins directory: %w", err)
		}
		fmt.Printf("✅ Removed existing %s directory\n", localPluginsDir)
	}

	// Create the local plugins directory
	if err := os.MkdirAll(localPluginsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Clone the repository to the local plugins directory
	fmt.Printf("⏳ Cloning repository %s to %s...\n", repoURL, localPluginsDir)
	cmd := exec.Command("git", "clone", repoURL, localPluginsDir)
	// Capture error output for better error reporting
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository: %w\n%s", err, stderr.String())
	}
	fmt.Println("✅ Clone completed")

	// Check if main.go exists in the root directory
	mainGoPath := filepath.Join(localPluginsDir, "main.go")
	if _, err := os.Stat(mainGoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository does not contain a main.go file in the root directory - not a valid plugin")
	}

	// Build the plugin
	if err := buildPlugin(localPluginsDir); err != nil {
		return fmt.Errorf("failed to build plugin: %w", err)
	}

	// Get the correct plugin name from the metadata
	pluginName, err := getPluginNameFromBuildResult(localPluginsDir)
	if err != nil {
		// If we can't get the name from metadata, use the repo name as fallback
		pluginName = repoName
		logger.Warnf("Could not extract plugin name from metadata, using repo name: %v", err)
	}

	// Create destination directory with the correct plugin name
	destDir := filepath.Join(pluginsDir, pluginName)

	// Check if plugin is already installed
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("plugin %s is already installed at %s", pluginName, destDir)
	}

	// Create the destination directory
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create plugin directory %s: %w", destDir, err)
	}

	// Check plugin compatibility
	mainSoPath := filepath.Join(localPluginsDir, "main.so")
	if err := checkPluginCompatibility(mainSoPath); err != nil {
		// Clean up the destination directory if incompatible
		os.RemoveAll(destDir)
		return fmt.Errorf("plugin compatibility check failed: %w", err)
	}

	// Copy the built plugin to the destination
	mainSoData, err := os.ReadFile(mainSoPath)
	if err != nil {
		// Clean up destination on failure
		os.RemoveAll(destDir)
		return fmt.Errorf("failed to read built plugin file: %w", err)
	}

	destPath := filepath.Join(destDir, "main.so")
	if err := os.WriteFile(destPath, mainSoData, 0o644); err != nil {
		// Clean up destination on failure
		os.RemoveAll(destDir)
		return fmt.Errorf("failed to write plugin file to destination: %w", err)
	}

	fmt.Printf("✅ Plugin %s installed successfully to %s\n", pluginName, destDir)

	// Clean up the local plugins directory
	fmt.Printf("⏳ Cleaning up temporary %s directory...\n", localPluginsDir)
	if err := os.RemoveAll(localPluginsDir); err != nil {
		// Just log this error as a warning since the installation was successful
		logger.Warnf("Warning: Failed to clean up local plugins directory: %v", err)
	} else {
		fmt.Printf("✅ Cleaned up temporary %s directory\n", localPluginsDir)
	}

	return nil
}

// getPluginNameFromBuildResult loads the plugin to get its actual name from metadata
func getPluginNameFromBuildResult(pluginDir string) (string, error) {
	// Default to directory name if we can't extract the plugin name
	dirName := filepath.Base(pluginDir)

	// Path to the built plugin
	pluginPath := filepath.Join(pluginDir, "main.so")

	// Try to open the plugin
	p, err := plugin.Open(pluginPath)
	if err != nil {
		return dirName, fmt.Errorf("failed to open plugin for metadata extraction: %w", err)
	}

	// Look up the CreatePlugin symbol
	sym, err := p.Lookup("CreatePlugin")
	if err != nil {
		return dirName, fmt.Errorf("plugin does not export 'CreatePlugin' symbol: %w", err)
	}

	// Create the plugin instance based on multiple possible signatures
	var pluginInstance interface{}

	switch createFunc := sym.(type) {
	case func() types.VoicifyPlugin:
		pluginInstance = createFunc()
	case func() interface{}:
		pluginInstance = createFunc()
	case func() pluginapi.VoicifyPlugin:
		pluginInstance = createFunc()
	default:
		return dirName, fmt.Errorf("plugin 'CreatePlugin' has unsupported signature: %T", sym)
	}

	// Try to call GetMetadata using reflection
	metadataMethod := reflect.ValueOf(pluginInstance).MethodByName("GetMetadata")
	if !metadataMethod.IsValid() {
		return dirName, fmt.Errorf("plugin does not have a GetMetadata method")
	}

	// Call the method
	results := metadataMethod.Call(nil)
	if len(results) == 0 {
		return dirName, fmt.Errorf("GetMetadata method returned no values")
	}

	// Try to extract the Name field using reflection
	metadataValue := reflect.ValueOf(results[0].Interface())
	nameField := metadataValue.FieldByName("Name")
	if !nameField.IsValid() {
		return dirName, fmt.Errorf("metadata does not have a Name field")
	}

	pluginName := nameField.String()
	if pluginName == "" {
		return dirName, fmt.Errorf("plugin name is empty")
	}

	return pluginName, nil
}
