package audio

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/dooshek/voicify/internal/logger"
)

// BluetoothDevice represents a Bluetooth audio card with handsfree capability
type BluetoothDevice struct {
	CardName    string `json:"card_name"`
	Description string `json:"description"`
	Address     string `json:"address"`
}

// HandsfreeManager handles switching Bluetooth profiles for recording
type HandsfreeManager struct {
	mu              sync.Mutex
	enabled         bool
	deviceCardName  string
	originalProfile string
	originalSource  string
	btSource        string
	switched        bool
}

// NewHandsfreeManager creates a new HandsfreeManager
func NewHandsfreeManager() *HandsfreeManager {
	return &HandsfreeManager{}
}

// SetConfig updates handsfree configuration
func (h *HandsfreeManager) SetConfig(enabled bool, device string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = enabled
	h.deviceCardName = device
	logger.Debugf("Handsfree config: enabled=%v, device=%s", enabled, device)
}

// SwitchToHandsfree switches the configured BT card to headset-head-unit profile.
// Returns nil if handsfree is disabled or device not configured.
// Blocks until PipeWire settles (~1.5s).
func (h *HandsfreeManager) SwitchToHandsfree() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.enabled || h.deviceCardName == "" {
		return nil
	}

	// Get current profile to restore later
	currentProfile, err := getCurrentProfile(h.deviceCardName)
	if err != nil {
		logger.Debugf("Handsfree: card not found (disconnected?), skipping: %v", err)
		return nil
	}

	// Already in handsfree mode
	if strings.HasPrefix(currentProfile, "headset-head-unit") {
		logger.Debugf("Handsfree: already in headset profile (%s)", currentProfile)
		return nil
	}

	h.originalProfile = currentProfile
	h.originalSource = getDefaultSource()

	logger.Infof("Handsfree: switching %s from %s to headset-head-unit", h.deviceCardName, currentProfile)

	// Switch to handsfree (mSBC preferred over CVSD)
	if err := setCardProfile(h.deviceCardName, "headset-head-unit"); err != nil {
		logger.Debugf("Handsfree: mSBC failed, trying CVSD: %v", err)
		if err := setCardProfile(h.deviceCardName, "headset-head-unit-cvsd"); err != nil {
			logger.Debugf("Handsfree: CVSD also failed: %v", err)
			return nil
		}
	}

	h.switched = true

	// Wait for PipeWire to create the BT source
	time.Sleep(300 * time.Millisecond)

	// Find BT input source and set it as default + max volume
	btSource := findBluetoothSource(h.deviceCardName)
	if btSource != "" {
		h.btSource = btSource
		logger.Infof("Handsfree: setting default source to %s", btSource)
		setDefaultSource(btSource)
		setSourceVolume(btSource, "100%")
	} else {
		logger.Debugf("Handsfree: could not find BT input source")
	}

	logger.Infof("Handsfree: ready, will restore %s after recording", h.originalProfile)
	return nil
}

// GetBTSource returns the BT input source name if handsfree is active
func (h *HandsfreeManager) GetBTSource() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.switched {
		return h.btSource
	}
	return ""
}

// RestoreProfile switches back to the original profile after recording
func (h *HandsfreeManager) RestoreProfile() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.switched {
		return
	}

	// Restore default source first
	if h.originalSource != "" {
		logger.Infof("Handsfree: restoring default source to %s", h.originalSource)
		setDefaultSource(h.originalSource)
	}

	// Restore card profile
	if h.originalProfile != "" {
		logger.Infof("Handsfree: restoring profile to %s", h.originalProfile)
		if err := setCardProfile(h.deviceCardName, h.originalProfile); err != nil {
			logger.Debugf("Handsfree: failed to restore profile: %v", err)
		}
	}

	h.switched = false
	h.originalProfile = ""
	h.originalSource = ""
	h.btSource = ""
}

// --- pactl helpers ---

func getCurrentProfile(cardName string) (string, error) {
	cmd := exec.Command("pactl", "list", "cards")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pactl list cards failed: %w", err)
	}
	blocks := strings.Split(string(output), "Card #")
	for _, block := range blocks {
		if !strings.Contains(block, "Name: "+cardName) {
			continue
		}
		for _, line := range strings.Split(block, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "Active Profile: ") {
				return strings.TrimPrefix(trimmed, "Active Profile: "), nil
			}
		}
	}
	return "", fmt.Errorf("card %s not found", cardName)
}

func setCardProfile(cardName, profile string) error {
	cmd := exec.Command("pactl", "set-card-profile", cardName, profile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set-card-profile failed: %s: %w", string(out), err)
	}
	return nil
}

func getDefaultSource() string {
	out, err := exec.Command("pactl", "get-default-source").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func setDefaultSource(source string) {
	if out, err := exec.Command("pactl", "set-default-source", source).CombinedOutput(); err != nil {
		logger.Debugf("Handsfree: set-default-source failed: %s: %v", string(out), err)
	}
}

func setSourceVolume(source, volume string) {
	if out, err := exec.Command("pactl", "set-source-volume", source, volume).CombinedOutput(); err != nil {
		logger.Debugf("Handsfree: set-source-volume failed: %s: %v", string(out), err)
	}
}

// findBluetoothSource finds the BT input source name.
// Card: bluez_card.80_99_E7_62_95_B0 → Source: bluez_input.80_99_E7_62_95_B0.0 (or with colons)
func findBluetoothSource(cardName string) string {
	// Extract address part: "bluez_card.80_99_E7_62_95_B0" -> "80_99_E7_62_95_B0"
	addr := strings.TrimPrefix(cardName, "bluez_card.")
	// Also try with colons for older PipeWire versions
	addrColons := strings.ReplaceAll(addr, "_", ":")

	out, err := exec.Command("pactl", "list", "sources", "short").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.Contains(fields[1], "bluez_input") {
			if strings.Contains(fields[1], addr) || strings.Contains(fields[1], addrColons) {
				return fields[1]
			}
		}
	}
	return ""
}
