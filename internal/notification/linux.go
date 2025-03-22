package notification

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/dooshek/voicify/internal/fileops"
	"github.com/dooshek/voicify/internal/logger"
)

//go:embed resources/sounds/*.oga resources/sounds/*.ogg
var soundFiles embed.FS

var (
	instance platformNotifier
	once     sync.Once
)

type linuxNotifier struct {
	audioDir string
}

func newLinuxNotifier() platformNotifier {
	once.Do(func() {
		logger.Debug("Initializing Linux notifier")

		fileOps, err := fileops.NewDefaultFileOps()
		if err != nil {
			logger.Errorf("Failed to initialize file operations: %v", err)
			return
		}

		notifier := &linuxNotifier{audioDir: fileOps.GetAudioDir()}

		// Initialize sound files
		if err := initializeSoundFiles(notifier.audioDir); err != nil {
			logger.Errorf("Failed to initialize sound files: %v", err)
		}

		// Setup cleanup on exit
		setupCleanup(notifier)

		instance = notifier
	})

	return instance
}

// Wydzielamy inicjalizację plików dźwiękowych do osobnej funkcji
func initializeSoundFiles(tempDir string) error {
	entries, err := soundFiles.ReadDir("resources/sounds")
	if err != nil {
		return fmt.Errorf("error reading embedded sounds directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			soundFile := entry.Name()
			logger.Debugf("Extracting sound file: %s", soundFile)
			data, err := soundFiles.ReadFile(filepath.Join("resources/sounds", soundFile))
			if err != nil {
				logger.Errorf("Error reading sound file: %v", err)
				continue
			}
			err = os.WriteFile(filepath.Join(tempDir, soundFile), data, 0o644)
			if err != nil {
				logger.Errorf("Error writing sound file: %v", err)
			}
		}
	}
	return nil
}

// Wydzielamy setup czyszczenia do osobnej funkcji
func setupCleanup(notifier *linuxNotifier) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		notifier.cleanup()
		os.Exit(0)
	}()
}

func (n *linuxNotifier) send(title, message string) error {
	logger.Debugf("Sending notification: %s - %s", title, message)
	go func() {
		if err := exec.Command("notify-send", title, message).Run(); err != nil {
			logger.Errorf("Failed to send notification: %v", err)
		}
	}()
	return nil
}

func (n *linuxNotifier) playStartBeep() error {
	go func() {
		if err := exec.Command("paplay", filepath.Join(n.audioDir, "start.ogg")).Run(); err != nil {
			logger.Errorf("Failed to play start beep: %v", err)
		}
	}()
	return nil
}

func (n *linuxNotifier) playStopBeep() error {
	go func() {
		if err := exec.Command("paplay", filepath.Join(n.audioDir, "stop.oga")).Run(); err != nil {
			logger.Errorf("Failed to play stop beep: %v", err)
		}
	}()
	return nil
}

func (n *linuxNotifier) playProgressBeep() error {
	go func() {
		if err := exec.Command("paplay", filepath.Join(n.audioDir, "progress.ogg")).Run(); err != nil {
			logger.Errorf("Failed to play progress beep: %v", err)
		}
	}()
	return nil
}

func (n *linuxNotifier) playTranscriptionOverBeep() error {
	go func() {
		if err := exec.Command("paplay", filepath.Join(n.audioDir, "transcription-over.oga")).Run(); err != nil {
			logger.Errorf("Failed to play transcription over beep: %v", err)
		}
	}()
	return nil
}

func (n *linuxNotifier) cleanup() {
	if n.audioDir != "" {
		logger.Debugf("Cleaning up temporary directory: %s", n.audioDir)
		if err := os.RemoveAll(n.audioDir); err != nil {
			logger.Errorf("Failed to cleanup temporary directory: %v", err)
		}
	}
}
