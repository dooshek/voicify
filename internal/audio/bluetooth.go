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

// restoreCooldown is how long we keep the headset-head-unit profile active
// after a recording ends, before reverting to the user's original (a2dp-sink).
//
// Why: rapid a2dp->hsp->a2dp->hsp flip-flop within seconds confuses bluez/SCO
// state on some headsets (Sony WH-1000XM5 notably). The "ready" source name
// resolves but the SCO link is half-dead, parecord reads EOF after one buffer
// (320 bytes). Holding the profile for a few seconds lets back-to-back
// recordings reuse the same SCO link, eliminating the race entirely. Music
// quality stays degraded for the cooldown window, which is the trade we make.
const restoreCooldown = 5 * time.Second

// HandsfreeManager handles switching Bluetooth profiles for recording
type HandsfreeManager struct {
	mu              sync.Mutex
	enabled         bool
	deviceCardName  string
	originalProfile string
	originalSource  string
	btSource        string
	switched        bool
	// restoreTimer fires `restoreCooldown` after the last recording ends.
	// SwitchToHandsfree stops it if the user records again within the window.
	restoreTimer *time.Timer
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
//
// If a previous recording's restore is still pending in the cooldown window,
// this cancels it and reuses the existing SCO link - no profile change, no
// race with bluez. Otherwise the profile is switched and we block until the
// BT input source is registered AND the SCO link has had time to settle.
func (h *HandsfreeManager) SwitchToHandsfree() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.enabled || h.deviceCardName == "" {
		return nil
	}

	// Pending restore from a recent recording - cancel it, we're recording again.
	if h.restoreTimer != nil {
		h.restoreTimer.Stop()
		h.restoreTimer = nil
		if h.switched && h.btSource != "" {
			logger.Debugf("Handsfree: reusing active headset profile (cooldown cancelled)")
			// Make sure default source is still bluez_input - some app could have
			// changed it during the cooldown. Cheap to re-set.
			setDefaultSource(h.btSource)
			return nil
		}
	}

	// Get current profile to restore later
	currentProfile, err := getCurrentProfile(h.deviceCardName)
	if err != nil {
		logger.Debugf("Handsfree: card not found (disconnected?), skipping: %v", err)
		return nil
	}

	// Already in handsfree mode (e.g., user set it manually)
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

	// Wait for the BT input source to register, polling every 100ms.
	btSource := waitForBTSource(h.deviceCardName, 2*time.Second)
	if btSource == "" {
		logger.Debugf("Handsfree: could not find BT input source within timeout")
	} else {
		// Source name is registered, but the SCO audio link may still be
		// settling. Without this grace period, parecord launches against a
		// "ready" source name but reads EOF after a single buffer (320 bytes).
		time.Sleep(600 * time.Millisecond)

		h.btSource = btSource
		logger.Infof("Handsfree: setting default source to %s", btSource)
		setDefaultSource(btSource)
		setSourceVolume(btSource, "100%")
	}

	logger.Infof("Handsfree: ready, will restore %s after recording (with %v cooldown)",
		h.originalProfile, restoreCooldown)
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

// RestoreProfile schedules a profile restore after restoreCooldown. If the
// user records again within the window, SwitchToHandsfree cancels the timer
// and reuses the existing SCO link.
func (h *HandsfreeManager) RestoreProfile() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.switched {
		return
	}

	// Replace any existing timer (e.g. if RestoreProfile is called twice)
	if h.restoreTimer != nil {
		h.restoreTimer.Stop()
	}
	logger.Infof("Handsfree: scheduling profile restore in %v", restoreCooldown)
	h.restoreTimer = time.AfterFunc(restoreCooldown, h.doRestore)
}

// doRestore performs the actual profile/source restore. Called by the
// cooldown timer or synchronously by ShutdownNow.
func (h *HandsfreeManager) doRestore() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.restoreLocked()
}

// restoreLocked performs the actual restore. Caller must hold h.mu.
func (h *HandsfreeManager) restoreLocked() {
	if !h.switched {
		return
	}

	if h.originalSource != "" {
		logger.Infof("Handsfree: restoring default source to %s", h.originalSource)
		setDefaultSource(h.originalSource)
	}

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
	h.restoreTimer = nil
}

// ShutdownNow restores the profile immediately, bypassing the cooldown.
// Call this from the daemon's shutdown path so we don't leave the headset
// stuck in handsfree mode after the process exits.
func (h *HandsfreeManager) ShutdownNow() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.restoreTimer != nil {
		h.restoreTimer.Stop()
		h.restoreTimer = nil
	}
	h.restoreLocked()
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

// waitForBTSource polls until the BT input source for cardName appears,
// returning its name. Returns "" if the timeout expires first.
func waitForBTSource(cardName string, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	for {
		if src := findBluetoothSource(cardName); src != "" {
			return src
		}
		if !time.Now().Before(deadline) {
			return ""
		}
		time.Sleep(100 * time.Millisecond)
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
