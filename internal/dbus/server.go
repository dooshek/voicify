package dbus

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/dooshek/voicify/internal/audio"
	"github.com/dooshek/voicify/internal/clipboard"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/notification"
	"github.com/dooshek/voicify/internal/state"
	"github.com/dooshek/voicify/internal/stats"
	"github.com/dooshek/voicify/internal/transcriptionrouter"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	dbusServiceName = "com.dooshek.voicify"
	dbusObjectPath  = "/com/dooshek/voicify/Recorder"
	dbusInterface   = "com.dooshek.voicify.Recorder"
)

// Server implements D-Bus service for voicify recording
type Server struct {
	conn                        *dbus.Conn
	recorder                    *audio.Recorder
	realtimeRecorder            *audio.RealtimeRecorder
	isRealtimeMode              bool
	postTranscriptionRouterMode bool // Post-transcription mode with router
	postTranscriptionAutoPaste  bool // Post-transcription mode with auto-paste
	ctx                         context.Context
	cancel                      context.CancelFunc
	// disconnected is closed when the bus connection drops or the service name is lost.
	// Allows main to distinguish unexpected disconnects (exit non-zero, systemd restarts)
	// from user-requested shutdown via Stop().
	disconnected chan struct{}
	mu           sync.Mutex
	// level forwarding
	levelForwardCancel context.CancelFunc
	// realtime transcription forwarding
	realtimeForwardCancel context.CancelFunc
	// accumulated realtime transcription across complete chunks
	realtimeAccum string
	// media playback state tracking
	//
	// pendingResume is true iff we paused MPRIS playback for a recording and
	// have not yet issued the matching resume. It persists across rapid
	// stop/start cycles: when the user starts recording 2 before recording 1's
	// resume fires, we cancel the timer but keep pendingResume true, so the
	// resume eventually happens after the LAST recording in the chain ends.
	// resumeTimer is cancellable via Stop() to support that.
	pendingResume     bool
	resumeTimer       *time.Timer
	autoPausePlayback bool
	// stats tracking
	statsManager       *stats.StatsManager
	recordingStartTime time.Time
	// model overrides from extension settings
	transcriptionModel    string
	realtimeModel         string
	transcriptionLanguage string
	// handsfree profile switching
	handsfreeManager *audio.HandsfreeManager
}

// NewServer creates a new D-Bus server instance with silent notifications
func NewServer() (*Server, error) {
	// Use silent notifier for daemon mode - extension handles all UI
	silentNotifier := notification.NewSilent()

	// Initialize regular recorder
	recorder, err := audio.NewRecorderWithNotifier(silentNotifier)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize recorder: %w", err)
	}

	// Initialize realtime recorder
	realtimeRecorder, err := audio.NewRealtimeRecorderWithNotifier(silentNotifier)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize realtime recorder: %w", err)
	}

	// Initialize stats manager (graceful degradation if fails)
	statsManager, err := stats.NewStatsManager()
	if err != nil {
		logger.Error("Failed to initialize stats manager, stats will be unavailable", err)
		// continue without stats - graceful degradation
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Default transcription settings - extension overrides via D-Bus
	const defaultModel = "gpt-4o-mini-transcribe"
	state.Get().Config.LLM.Transcription.Model = defaultModel
	state.Get().Config.LLM.Transcription.Language = "" // auto-detect
	state.Get().Config.LLM.Transcription.Provider = "openai"

	hf := audio.NewHandsfreeManager()

	// The extension pushes config (transcription model, auto-pause, handsfree
	// device, etc.) only at its own enable(); after a standalone daemon
	// restart we have no values until the user toggles a setting. Read them
	// directly from the extension's GSettings schema so the daemon survives
	// a restart with full fidelity.
	cfg := loadExtensionSettings(hf)
	model := defaultModel
	if cfg.model != "" {
		model = cfg.model
	}
	realtime := defaultModel
	if cfg.realtime != "" {
		realtime = cfg.realtime
	}
	state.Get().Config.LLM.Transcription.Model = model
	state.Get().Config.LLM.Transcription.Language = cfg.lang

	return &Server{
		recorder:              recorder,
		realtimeRecorder:      realtimeRecorder,
		statsManager:          statsManager,
		transcriptionModel:    model,
		realtimeModel:         realtime,
		transcriptionLanguage: cfg.lang,
		isRealtimeMode:        false,
		handsfreeManager:      hf,
		ctx:                   ctx,
		cancel:                cancel,
		disconnected:          make(chan struct{}),
		autoPausePlayback:     cfg.autoPause,
	}, nil
}

// extensionSettings holds the daemon-relevant subset of the GNOME extension's
// GSettings schema, read at startup so the daemon survives a restart with
// full fidelity (the extension only pushes config from its own enable()).
type extensionSettings struct {
	model     string
	realtime  string
	lang      string
	autoPause bool
}

// loadExtensionSettings reads daemon-relevant settings from the GNOME
// extension's GSettings schema. Applies handsfree config directly on hf
// (it has its own setter); other settings are returned for the caller to
// wire into the Server.
func loadExtensionSettings(hf *audio.HandsfreeManager) extensionSettings {
	var s extensionSettings
	schemaDir := os.Getenv("HOME") + "/.local/share/gnome-shell/extensions/voicify@dooshek.com/schemas/"
	if _, err := os.Stat(schemaDir); err != nil {
		logger.Debugf("Extension schema dir not found, skipping gsettings preload")
		return s
	}

	get := func(key string) string {
		cmd := exec.Command("gsettings", "get", "org.gnome.shell.extensions.voicify", key)
		cmd.Env = append(os.Environ(), "GSETTINGS_SCHEMA_DIR="+schemaDir)
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		// gsettings string values are quoted: 'value'. Booleans are bare: true/false.
		return strings.Trim(strings.TrimSpace(string(out)), "'")
	}

	s.model = get("transcription-model")
	s.realtime = get("realtime-model")
	s.lang = get("transcription-language")
	s.autoPause = get("auto-pause-playback") == "true"

	if hfEnabled := get("handsfree-enabled") == "true"; hfEnabled {
		if device := get("handsfree-device"); device != "" {
			hf.SetConfig(true, device)
			logger.Infof("Loaded handsfree config from gsettings (device=%s)", device)
		}
	}
	if s.model != "" {
		logger.Infof("Loaded transcription model from gsettings: %s", s.model)
	}
	logger.Infof("Loaded auto-pause-playback from gsettings: %v", s.autoPause)
	return s
}

// Start starts the D-Bus server
func (s *Server) Start() error {
	var err error
	s.conn, err = dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}

	// Request name
	reply, err := s.conn.RequestName(dbusServiceName, dbus.NameFlagDoNotQueue)
	if err != nil {
		s.conn.Close()
		return fmt.Errorf("failed to request name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		s.conn.Close()
		return fmt.Errorf("name already taken")
	}

	// Export object
	err = s.conn.Export(s, dbusObjectPath, dbusInterface)
	if err != nil {
		s.conn.Close()
		return fmt.Errorf("failed to export object: %w", err)
	}

	// Export introspection
	node := &introspect.Node{
		Name: dbusObjectPath,
		Interfaces: []introspect.Interface{{
			Name: dbusInterface,
			Methods: []introspect.Method{
				{
					Name: "StartRealtimeRecording",
				},
				{
					Name: "GetStatus",
					Args: []introspect.Arg{
						{Name: "is_recording", Type: "b", Direction: "out"},
					},
				},
				{
					Name: "CancelRecording",
				},
				{
					Name: "TogglePostTranscriptionAutoPaste",
				},
				{
					Name: "TogglePostTranscriptionRouter",
				},
				{
					Name: "UpdateFocusedWindow",
					Args: []introspect.Arg{
						{Name: "title", Type: "s", Direction: "in"},
						{Name: "app", Type: "s", Direction: "in"},
					},
				},
				{
					Name: "SetAutoPausePlayback",
					Args: []introspect.Arg{
						{Name: "enabled", Type: "b", Direction: "in"},
					},
				},
				{
					Name: "GetRecordingStats",
					Args: []introspect.Arg{
						{Name: "stats_json", Type: "s", Direction: "out"},
					},
				},
				{
					Name: "ResetRecordingStats",
				},
				{
					Name: "SetTranscriptionConfig",
					Args: []introspect.Arg{
						{Name: "standard", Type: "s", Direction: "in"},
						{Name: "realtime", Type: "s", Direction: "in"},
						{Name: "language", Type: "s", Direction: "in"},
					},
				},
				{
					Name: "SetHandsfreeConfig",
					Args: []introspect.Arg{
						{Name: "enabled", Type: "b", Direction: "in"},
						{Name: "device", Type: "s", Direction: "in"},
					},
				},
			},
			Signals: []introspect.Signal{
				{Name: "RecordingStarted"},
				{
					Name: "TranscriptionReady",
					Args: []introspect.Arg{
						{Name: "text", Type: "s"},
					},
				},
				{
					Name: "PartialTranscription",
					Args: []introspect.Arg{
						{Name: "text", Type: "s"},
					},
				},
				{
					Name: "CompleteTranscription",
					Args: []introspect.Arg{
						{Name: "text", Type: "s"},
					},
				},
				{
					Name: "RecordingError",
					Args: []introspect.Arg{
						{Name: "error", Type: "s"},
					},
				},
				{Name: "RecordingCancelled"},
				{
					Name: "InputLevel",
					Args: []introspect.Arg{
						{Name: "level", Type: "d"},
					},
				},
				{
					Name: "RequestPaste",
					Args: []introspect.Arg{
						{Name: "text", Type: "s"},
					},
				},
			},
		}},
	}

	err = s.conn.Export(introspect.NewIntrospectable(node), dbusObjectPath, "org.freedesktop.DBus.Introspectable")
	if err != nil {
		s.conn.Close()
		return fmt.Errorf("failed to export introspectable: %w", err)
	}

	logger.Infof("🔌 D-Bus service started: %s", dbusServiceName)
	logger.Infof("💡 Extension can now communicate with voicify daemon")

	// Watch for connection drop or service-name loss. Either means we're no longer
	// reachable; signal main so the process exits and systemd restarts us.
	go s.watchConnection()

	return nil
}

// watchConnection signals s.disconnected if the bus connection closes or our
// service name is taken away. godbus does not auto-reconnect, so a connection
// drop (e.g. session bus restart on suspend/resume) leaves the process alive
// but unreachable - we must exit so the supervisor can restart a fresh instance.
func (s *Server) watchConnection() {
	// NameLost subscription
	nameLostCh := make(chan *dbus.Signal, 4)
	s.conn.Signal(nameLostCh)
	if err := s.conn.AddMatchSignal(
		dbus.WithMatchObjectPath("/org/freedesktop/DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameLost"),
	); err != nil {
		logger.Errorf("Failed to subscribe to NameLost", err)
	}

	connDone := s.conn.Context().Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-connDone:
			logger.Error("D-Bus connection closed unexpectedly", fmt.Errorf("session bus disconnected"))
			s.signalDisconnected()
			return
		case sig := <-nameLostCh:
			if sig == nil {
				continue
			}
			if len(sig.Body) > 0 {
				if name, ok := sig.Body[0].(string); ok && name == dbusServiceName {
					logger.Error("Lost ownership of D-Bus name", fmt.Errorf("name=%s", dbusServiceName))
					s.signalDisconnected()
					return
				}
			}
		}
	}
}

func (s *Server) signalDisconnected() {
	select {
	case <-s.disconnected:
		// already closed
	default:
		close(s.disconnected)
	}
}

// Disconnected returns a channel that is closed when the daemon loses its
// D-Bus presence (connection drop or NameLost). Main should treat this as
// a fatal condition and exit non-zero so systemd restarts the daemon.
func (s *Server) Disconnected() <-chan struct{} {
	return s.disconnected
}

// Stop stops the D-Bus server
func (s *Server) Stop() {
	// Force-restore the BT profile synchronously - we're exiting, can't leave
	// a pending cooldown timer behind.
	if s.handsfreeManager != nil {
		s.handsfreeManager.ShutdownNow()
	}
	s.cancel()
	if s.conn != nil {
		s.conn.Close()
	}
	logger.Infof("🔌 D-Bus service stopped")
}

// Wait waits for the server context to be cancelled
func (s *Server) Wait() {
	<-s.ctx.Done()
}

// TogglePostTranscriptionAutoPaste toggles post-transcription recording with auto-paste (D-Bus method)
func (s *Server) TogglePostTranscriptionAutoPaste() *dbus.Error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Infof("EVENT: shortcut TogglePostTranscriptionAutoPaste (Ctrl+Super+C) - currently recording=%v",
		s.recorder.IsRecording() || s.realtimeRecorder.IsRecording())

	if s.recorder.IsRecording() || s.realtimeRecorder.IsRecording() {
		// Already recording - stop it
		logger.Debugf("D-Bus: Stopping post-transcription auto-paste recording")
		go s.stopPostTranscriptionAutoPasteAsync()
	} else {
		// Check if media is currently playing before starting recording
		s.pauseMediaIfPlaying()

		// Start recording in auto-paste mode
		logger.Debugf("D-Bus: Starting post-transcription auto-paste recording")
		s.postTranscriptionAutoPaste = true
		s.postTranscriptionRouterMode = false
		s.isRealtimeMode = false

		s.handsfreeManager.SwitchToHandsfree()
		s.recorder.SetCaptureSource(s.handsfreeManager.GetBTSource())

		s.recordingStartTime = time.Now()
		s.recorder.Start()
		s.emitSignal("RecordingStarted")
		s.startForwardingLevels()
	}

	return nil
}

// TogglePostTranscriptionRouter toggles post-transcription recording with router (D-Bus method)
func (s *Server) TogglePostTranscriptionRouter() *dbus.Error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Infof("EVENT: shortcut TogglePostTranscriptionRouter (Ctrl+Super+D) - currently recording=%v",
		s.recorder.IsRecording() || s.realtimeRecorder.IsRecording())

	if s.recorder.IsRecording() || s.realtimeRecorder.IsRecording() {
		// Already recording - stop it
		logger.Debugf("D-Bus: Stopping post-transcription router recording")
		go s.stopPostTranscriptionRouterAsync()
	} else {
		// Check if media is currently playing before starting recording
		s.pauseMediaIfPlaying()

		// Start recording in router mode
		logger.Debugf("D-Bus: Starting post-transcription router recording")
		s.postTranscriptionRouterMode = true
		s.postTranscriptionAutoPaste = false
		s.isRealtimeMode = false

		s.handsfreeManager.SwitchToHandsfree()
		s.recorder.SetCaptureSource(s.handsfreeManager.GetBTSource())

		s.recordingStartTime = time.Now()
		s.recorder.Start()
		s.emitSignal("RecordingStarted")
		s.startForwardingLevels()
	}

	return nil
}

// StartRealtimeRecording starts real-time recording with streaming transcription (D-Bus method)
func (s *Server) StartRealtimeRecording() *dbus.Error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Infof("EVENT: shortcut StartRealtimeRecording (Ctrl+Super+V) - currently realtime=%v",
		s.realtimeRecorder.IsRecording())

	if s.recorder.IsRecording() || s.realtimeRecorder.IsRecording() {
		return dbus.MakeFailedError(fmt.Errorf("recording already in progress"))
	}

	// Check if media is currently playing before starting recording
	s.pauseMediaIfPlaying()

	s.isRealtimeMode = true
	// reset accumulator for this session
	s.realtimeAccum = ""

	s.recordingStartTime = time.Now()

	// Set realtime model and language
	s.realtimeRecorder.SetRealtimeModel(s.realtimeModel)
	s.realtimeRecorder.SetRealtimeLanguage(s.transcriptionLanguage)

	s.handsfreeManager.SwitchToHandsfree()

	// Start real-time recording
	logger.Debugf("D-Bus: Starting real-time recording")
	if err := s.realtimeRecorder.Start(); err != nil {
		logger.Errorf("D-Bus: Failed to start realtime recording", err)
		return dbus.MakeFailedError(fmt.Errorf("failed to start realtime recording: %w", err))
	}

	// Emit signal
	s.emitSignal("RecordingStarted")

	// Start forwarding real-time transcription results
	s.startForwardingRealtimeTranscription()

	// Start forwarding input levels from realtime recorder
	s.startForwardingRealtimeLevels()

	return nil
}

// GetStatus returns current recording status (D-Bus method)
func (s *Server) GetStatus() (bool, *dbus.Error) {
	return s.recorder.IsRecording() || s.realtimeRecorder.IsRecording(), nil
}

// CancelRecording cancels the current recording (D-Bus method)
func (s *Server) CancelRecording() *dbus.Error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Infof("EVENT: shortcut CancelRecording")

	if s.isRealtimeMode {
		if !s.realtimeRecorder.IsRecording() {
			logger.Debugf("D-Bus: No realtime recording in progress, cancel is no-op")
			return nil
		}

		logger.Debugf("D-Bus: Cancelling realtime recording")
		s.realtimeRecorder.Cancel()
		s.stopForwardingRealtimeTranscription()
		s.stopForwardingRealtimeLevels()

		go s.handsfreeManager.RestoreProfile()
		s.scheduleMediaResumeLocked()

		// After cancelling realtime, route the accumulated transcription if present
		finalText := s.realtimeAccum
		s.realtimeAccum = "" // reset for next run

		// Track stats for realtime recording
		if s.statsManager != nil && finalText != "" {
			duration := time.Since(s.recordingStartTime).Seconds()
			s.statsManager.AddRecording(s.realtimeModel, duration)
		}

		if finalText != "" {
			// Route via router
			router := transcriptionrouter.New(finalText)
			if err := router.Route(finalText); err != nil {
				logger.Errorf("D-Bus: Error routing realtime transcription", err)
				s.emitSignal("RecordingError", fmt.Sprintf("routing error: %v", err))
			} else {
				// Emit final transcription ready for any listeners
				s.emitSignal("TranscriptionReady", finalText)
			}
		}
	} else {
		if !s.recorder.IsRecording() {
			logger.Debugf("D-Bus: No recording in progress, cancel is no-op")
			return nil
		}

		logger.Debugf("D-Bus: Cancelling regular recording")
		s.recorder.Cancel()
		s.stopForwardingLevels()

		go s.handsfreeManager.RestoreProfile()
		s.scheduleMediaResumeLocked()
	}

	// Emit signal
	s.emitSignal("RecordingCancelled")

	return nil
}

// UpdateFocusedWindow updates the cached focused window info (D-Bus method)
func (s *Server) UpdateFocusedWindow(title string, app string) *dbus.Error {
	logger.Debugf("D-Bus: UpdateFocusedWindow called - title: %s, app: %s", title, app)

	state.Get().SetFocusedWindow(title, app)

	return nil
}

// SetAutoPausePlayback enables or disables automatic media pause/resume during recording
func (s *Server) SetAutoPausePlayback(enabled bool) *dbus.Error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoPausePlayback = enabled
	logger.Debugf("D-Bus: SetAutoPausePlayback = %v", enabled)
	return nil
}

// GetRecordingStats returns recording statistics as JSON (D-Bus method)
func (s *Server) GetRecordingStats() (string, *dbus.Error) {
	if s.statsManager == nil {
		return "{}", nil
	}
	json, err := s.statsManager.GetStatsJSON()
	if err != nil {
		return "{}", dbus.MakeFailedError(err)
	}
	return json, nil
}

// ResetRecordingStats resets all recording statistics (D-Bus method)
func (s *Server) ResetRecordingStats() *dbus.Error {
	if s.statsManager == nil {
		return nil
	}
	if err := s.statsManager.Reset(); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

// SetTranscriptionConfig sets transcription models and language from extension settings (D-Bus method)
func (s *Server) SetTranscriptionConfig(standard string, realtime string, language string) *dbus.Error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if standard != "" {
		s.transcriptionModel = standard
		state.Get().Config.LLM.Transcription.Model = standard
		logger.Debugf("D-Bus: Transcription model set to: %s", standard)
	}
	if realtime != "" {
		s.realtimeModel = realtime
		logger.Debugf("D-Bus: Realtime model set to: %s", realtime)
	}
	s.transcriptionLanguage = language
	state.Get().Config.LLM.Transcription.Language = language
	logger.Debugf("D-Bus: Transcription language set to: %q", language)
	return nil
}

// SetHandsfreeConfig enables/disables BT handsfree profile switching (D-Bus method)
func (s *Server) SetHandsfreeConfig(enabled bool, device string) *dbus.Error {
	s.handsfreeManager.SetConfig(enabled, device)
	logger.Debugf("D-Bus: Handsfree config set: enabled=%v, device=%s", enabled, device)
	return nil
}

// pauseMediaIfPlaying pauses MPRIS playback when starting a recording.
//
// State machine: pendingResume tracks whether we owe a "resume" to the user.
// If a previous recording in a rapid stop-start chain already paused media
// and is still waiting in cooldown, we cancel that timer and inherit
// pendingResume - the resume will happen after the LAST recording ends, not
// after each one. This avoids the bug where MPRIS reports Paused on the
// second start (because we paused it ourselves), pauseAndCheck returns false,
// and the eventual resume never fires.
//
// Must be called with s.mu held.
func (s *Server) pauseMediaIfPlaying() {
	if !s.autoPausePlayback {
		logger.Infof("media: autoPausePlayback disabled, skipping pause/resume cycle")
		return
	}

	// Cancel any pending resume from a previous recording. We're starting
	// another one, so the resume that was scheduled for the previous stop
	// should not fire mid-this-recording.
	if s.resumeTimer != nil {
		s.resumeTimer.Stop()
		s.resumeTimer = nil
		logger.Infof("media: pending resume cancelled (new recording starting, pendingResume=%v carries over)",
			s.pendingResume)
		// If pendingResume was already set, keep it - we still owe a resume,
		// and media is already paused. No need to re-issue the pause.
		if s.pendingResume {
			return
		}
	}

	statusCmd := exec.Command("playerctl", "status")
	statusOut, err := statusCmd.Output()
	if err != nil {
		logger.Infof("media: no MPRIS players, nothing to pause")
		return
	}

	status := strings.TrimSpace(string(statusOut))
	if status != "Playing" {
		logger.Infof("media: MPRIS status=%s, nothing to pause", status)
		return
	}

	logger.Infof("media: pausing (MPRIS Playing detected)")
	pauseCmd := exec.Command("playerctl", "pause")
	if err := pauseCmd.Run(); err != nil {
		logger.Warnf("media: playerctl pause failed (%v), falling back to XF86AudioPause", err)
		xdoCmd := exec.Command("xdotool", "key", "XF86AudioPause")
		if err2 := xdoCmd.Run(); err2 != nil {
			logger.Errorf("media: XF86AudioPause also failed", err2)
		}
	}
	s.pendingResume = true
}

// scheduleMediaResume arms a cancellable timer to resume MPRIS playback if
// pendingResume is set. Replaces the old goroutine-with-sleep model so a
// fresh recording can cancel the in-flight resume cleanly.
//
// Takes s.mu - DO NOT call from a context that already holds the lock
// (e.g. inside a D-Bus method). Use scheduleMediaResumeLocked() there.
func (s *Server) scheduleMediaResume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduleMediaResumeLocked()
}

// scheduleMediaResumeLocked is the lock-free variant for callers that
// already hold s.mu (e.g. CancelRecording). go's sync.Mutex is not
// reentrant, so calling scheduleMediaResume() under the lock would
// deadlock the daemon - and silently break the entire D-Bus surface,
// since every other method also takes s.mu.
func (s *Server) scheduleMediaResumeLocked() {
	if !s.pendingResume {
		return
	}
	if s.resumeTimer != nil {
		s.resumeTimer.Stop()
	}
	logger.Infof("media: resume scheduled in 2.5s")
	s.resumeTimer = time.AfterFunc(2500*time.Millisecond, s.fireMediaResume)
}

// fireMediaResume is the timer callback for scheduleMediaResume.
func (s *Server) fireMediaResume() {
	s.mu.Lock()
	s.resumeTimer = nil
	recording := s.recorder.IsRecording() || s.realtimeRecorder.IsRecording()
	s.mu.Unlock()

	if recording {
		// Don't resume mid-recording - the eventual stop will reschedule us.
		logger.Infof("media: resume timer fired but recording in progress, holding (will reschedule on stop)")
		return
	}

	logger.Infof("media: resuming via playerctl play")
	cmd := exec.Command("playerctl", "play")
	if err := cmd.Run(); err != nil {
		logger.Warnf("media: playerctl play failed (%v), falling back to XF86AudioPlay", err)
		xdoCmd := exec.Command("xdotool", "key", "XF86AudioPlay")
		if err2 := xdoCmd.Run(); err2 != nil {
			logger.Errorf("media: XF86AudioPlay also failed", err2)
		}
	}

	s.mu.Lock()
	s.pendingResume = false
	s.mu.Unlock()
}

// stopPostTranscriptionAutoPasteAsync stops recording in auto-paste mode
func (s *Server) stopPostTranscriptionAutoPasteAsync() {
	go func() {
		logger.Debugf("D-Bus: Stopping post-transcription auto-paste recording")

		transcription, err := s.recorder.Stop()
		go s.handsfreeManager.RestoreProfile()
		if err != nil {
			logger.Errorf("D-Bus: Error stopping recording", err)
			s.emitSignal("RecordingError", err.Error())
			s.postTranscriptionAutoPaste = false
			s.scheduleMediaResume()
			return
		}

		// Stop forwarding levels
		s.stopForwardingLevels()

		// Resume media playback after recording stops
		s.scheduleMediaResume()

		logger.Debugf("D-Bus: Post-transcription auto-paste received: %s", transcription)

		// Track recording stats
		if s.statsManager != nil {
			duration := time.Since(s.recordingStartTime).Seconds()
			s.statsManager.AddRecording(s.transcriptionModel, duration)
		}

		// Copy to clipboard and trigger paste via extension
		if err := clipboard.CopyToClipboard(transcription); err != nil {
			logger.Error("D-Bus: Failed to copy to clipboard", err)
			s.emitSignal("RecordingError", "clipboard error")
			s.postTranscriptionAutoPaste = false
			return
		}

		// Reset mode
		s.postTranscriptionAutoPaste = false

		// Emit signal that transcription is ready for auto-paste
		s.emitSignal("TranscriptionReady", transcription)
	}()
}

// stopPostTranscriptionRouterAsync stops recording in router mode
func (s *Server) stopPostTranscriptionRouterAsync() {
	go func() {
		logger.Debugf("D-Bus: Stopping post-transcription router recording")

		transcription, err := s.recorder.Stop()
		go s.handsfreeManager.RestoreProfile()
		if err != nil {
			logger.Errorf("D-Bus: Error stopping recording", err)
			s.emitSignal("RecordingError", err.Error())
			s.postTranscriptionRouterMode = false
			s.scheduleMediaResume()
			return
		}

		// Stop forwarding levels
		s.stopForwardingLevels()

		// Resume media playback after recording stops
		s.scheduleMediaResume()

		logger.Debugf("D-Bus: Post-transcription router received: %s", transcription)

		// Track recording stats
		if s.statsManager != nil {
			duration := time.Since(s.recordingStartTime).Seconds()
			s.statsManager.AddRecording(s.transcriptionModel, duration)
		}

		// Route through router - plugins may call RequestPaste
		router := transcriptionrouter.New(transcription)
		if err := router.Route(transcription); err != nil {
			logger.Errorf("D-Bus: Error routing post-transcription", err)
			s.emitSignal("RecordingError", fmt.Sprintf("routing error: %v", err))
		}

		// Reset mode
		s.postTranscriptionRouterMode = false

		// Emit signal that transcription is ready (but not auto-pasted)
		s.emitSignal("TranscriptionReady", transcription)
	}()
}

// emitSignal emits a D-Bus signal
func (s *Server) emitSignal(name string, args ...interface{}) {
	if s.conn == nil {
		logger.Warnf("D-Bus: Cannot emit signal %s - no connection", name)
		return
	}

	signalPath := dbus.ObjectPath(dbusObjectPath)
	signalName := dbusInterface + "." + name

	err := s.conn.Emit(signalPath, signalName, args...)
	if err != nil {
		logger.Errorf("D-Bus: Failed to emit signal %s", err, name)
	} else {
		if name == "InputLevel" && len(args) > 0 {
			// logger.Debugf("D-Bus: Emitted signal: %s with value: %v", name, args[0])
		} else {
			logger.Debugf("D-Bus: Emitted signal: %s", name)
		}
	}
}

// EmitRequestPaste emits a RequestPaste signal for plugins to trigger text insertion
func (s *Server) EmitRequestPaste(text string) error {
	if s.conn == nil {
		return fmt.Errorf("no D-Bus connection")
	}

	logger.Debugf("D-Bus: Plugin requesting paste of text: %s", text)
	s.emitSignal("RequestPaste", text)
	return nil
}

// startForwardingLevels begins reading from recorder.LevelChan() and emits InputLevel signals
func (s *Server) startForwardingLevels() {
	if s.levelForwardCancel != nil {
		// already forwarding
		return
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.levelForwardCancel = cancel

	go func() {
		levelCh := s.recorder.LevelChan()
		for {
			select {
			case <-ctx.Done():
				return
			case level := <-levelCh:
				// Emit on D-Bus; ignore errors here
				s.emitSignal("InputLevel", level)
			}
		}
	}()
}

// stopForwardingLevels stops the level forwarding goroutine
func (s *Server) stopForwardingLevels() {
	if s.levelForwardCancel != nil {
		s.levelForwardCancel()
		s.levelForwardCancel = nil
	}
}

// startForwardingRealtimeTranscription starts forwarding realtime transcription results
func (s *Server) startForwardingRealtimeTranscription() {
	if s.realtimeForwardCancel != nil {
		// already forwarding
		return
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.realtimeForwardCancel = cancel

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case complete := <-s.realtimeRecorder.CompleteChan():
				logger.Debugf("D-Bus: Emitting complete transcription: %s", complete)
				s.emitSignal("CompleteTranscription", complete)
				// Accumulate complete chunks for final routing on cancel
				if s.realtimeAccum == "" {
					s.realtimeAccum = complete
				} else {
					// Add a space/newline between segments
					s.realtimeAccum = s.realtimeAccum + " " + complete
				}
			case err := <-s.realtimeRecorder.ErrorChan():
				logger.Errorf("D-Bus: Realtime transcription error", err)
				s.emitSignal("RecordingError", err.Error())
			}
		}
	}()
}

// stopForwardingRealtimeTranscription stops the realtime transcription forwarding
func (s *Server) stopForwardingRealtimeTranscription() {
	if s.realtimeForwardCancel != nil {
		s.realtimeForwardCancel()
		s.realtimeForwardCancel = nil
	}
}

// startForwardingRealtimeLevels starts forwarding audio levels from realtime recorder
func (s *Server) startForwardingRealtimeLevels() {
	if s.levelForwardCancel != nil {
		// already forwarding
		return
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.levelForwardCancel = cancel

	go func() {
		levelCh := s.realtimeRecorder.LevelChan()
		for {
			select {
			case <-ctx.Done():
				return
			case level := <-levelCh:
				s.emitSignal("InputLevel", level)
			}
		}
	}()
}

// stopForwardingRealtimeLevels stops the realtime level forwarding
func (s *Server) stopForwardingRealtimeLevels() {
	if s.levelForwardCancel != nil {
		s.levelForwardCancel()
		s.levelForwardCancel = nil
	}
}
