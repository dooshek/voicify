package dbus

import (
	"context"
	"fmt"
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
	mu                          sync.Mutex
	// level forwarding
	levelForwardCancel context.CancelFunc
	// realtime transcription forwarding
	realtimeForwardCancel context.CancelFunc
	// accumulated realtime transcription across complete chunks
	realtimeAccum string
	// media playback state tracking
	wasMediaPlaying   bool
	autoPausePlayback bool
	// stats tracking
	statsManager       *stats.StatsManager
	recordingStartTime time.Time
	// model overrides from extension settings
	transcriptionModel string
	realtimeModel      string
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

	// Default transcription models - extension overrides via SetTranscriptionModels D-Bus call
	const defaultModel = "gpt-4o-mini-transcribe"
	// Set in state so standard transcriber picks it up immediately
	state.Get().Config.LLM.Transcription.Model = defaultModel

	return &Server{
		recorder:           recorder,
		realtimeRecorder:   realtimeRecorder,
		statsManager:       statsManager,
		transcriptionModel: defaultModel,
		realtimeModel:      defaultModel,
		isRealtimeMode:     false,
		ctx:                ctx,
		cancel:             cancel,
	}, nil
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
					Name: "SetTranscriptionModels",
					Args: []introspect.Arg{
						{Name: "standard", Type: "s", Direction: "in"},
						{Name: "realtime", Type: "s", Direction: "in"},
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

	return nil
}

// Stop stops the D-Bus server
func (s *Server) Stop() {
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

	logger.Debugf("D-Bus: TogglePostTranscriptionAutoPaste called")

	if s.recorder.IsRecording() || s.realtimeRecorder.IsRecording() {
		// Already recording - stop it
		logger.Debugf("D-Bus: Stopping post-transcription auto-paste recording")
		go s.stopPostTranscriptionAutoPasteAsync()
	} else {
		// Check if media is currently playing before starting recording
		s.wasMediaPlaying = s.pauseAndCheckMediaPlaying()

		// Start recording in auto-paste mode
		logger.Debugf("D-Bus: Starting post-transcription auto-paste recording")
		s.postTranscriptionAutoPaste = true
		s.postTranscriptionRouterMode = false
		s.isRealtimeMode = false

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

	logger.Debugf("D-Bus: TogglePostTranscriptionRouter called")

	if s.recorder.IsRecording() || s.realtimeRecorder.IsRecording() {
		// Already recording - stop it
		logger.Debugf("D-Bus: Stopping post-transcription router recording")
		go s.stopPostTranscriptionRouterAsync()
	} else {
		// Check if media is currently playing before starting recording
		s.wasMediaPlaying = s.pauseAndCheckMediaPlaying()

		// Start recording in router mode
		logger.Debugf("D-Bus: Starting post-transcription router recording")
		s.postTranscriptionRouterMode = true
		s.postTranscriptionAutoPaste = false
		s.isRealtimeMode = false

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

	logger.Debugf("D-Bus: StartRealtimeRecording called")

	if s.recorder.IsRecording() || s.realtimeRecorder.IsRecording() {
		return dbus.MakeFailedError(fmt.Errorf("recording already in progress"))
	}

	// Check if media is currently playing before starting recording
	s.wasMediaPlaying = s.pauseAndCheckMediaPlaying()

	s.isRealtimeMode = true
	// reset accumulator for this session
	s.realtimeAccum = ""

	s.recordingStartTime = time.Now()

	// Set realtime model if overridden
	if s.realtimeModel != "" {
		s.realtimeRecorder.SetRealtimeModel(s.realtimeModel)
	}

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

	logger.Debugf("D-Bus: CancelRecording called")

	if s.isRealtimeMode {
		if !s.realtimeRecorder.IsRecording() {
			logger.Debugf("D-Bus: No realtime recording in progress, cancel is no-op")
			return nil
		}

		logger.Debugf("D-Bus: Cancelling realtime recording")
		s.realtimeRecorder.Cancel()
		s.stopForwardingRealtimeTranscription()
		s.stopForwardingRealtimeLevels()

		// Resume media playback after recording stops
		go s.resumeMediaPlayback()

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

		// Resume media playback after recording stops
		go s.resumeMediaPlayback()
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

// SetTranscriptionModels sets the transcription models from extension settings (D-Bus method)
func (s *Server) SetTranscriptionModels(standard string, realtime string) *dbus.Error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if standard != "" {
		s.transcriptionModel = standard
		// Update state so openai/groq transcriber picks it up
		state.Get().Config.LLM.Transcription.Model = standard
		logger.Debugf("D-Bus: Transcription model set to: %s", standard)
	}
	if realtime != "" {
		s.realtimeModel = realtime
		logger.Debugf("D-Bus: Realtime model set to: %s", realtime)
	}
	return nil
}

// pauseAndCheckMediaPlaying pauses media if playing and returns whether it was playing.
// Must be called with s.mu held.
func (s *Server) pauseAndCheckMediaPlaying() bool {
	if !s.autoPausePlayback {
		return false
	}

	cmd := exec.Command("pactl", "list", "sink-inputs")
	output, err := cmd.Output()
	if err != nil {
		logger.Debugf("Failed to check media playing state: %v", err)
		return false
	}

	// Parse output to find if any stream is not corked (actively playing)
	playing := false
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Corked: no") {
			playing = true
			break
		}
	}

	if !playing {
		logger.Debugf("No active media playback detected")
		return false
	}

	logger.Debugf("Active media playback detected, pausing via playerctl")
	pauseCmd := exec.Command("playerctl", "pause")
	if err := pauseCmd.Run(); err != nil {
		logger.Debugf("playerctl pause failed, trying XF86AudioPause: %v", err)
		xdoCmd := exec.Command("xdotool", "key", "XF86AudioPause")
		if err2 := xdoCmd.Run(); err2 != nil {
			logger.Debugf("XF86AudioPause also failed: %v", err2)
		}
	}
	return true
}

// resumeMediaPlayback attempts to resume media playback after recording stops.
// Reads and resets wasMediaPlaying atomically under lock.
func (s *Server) resumeMediaPlayback() {
	s.mu.Lock()
	was := s.wasMediaPlaying
	s.wasMediaPlaying = false
	s.mu.Unlock()

	if !was {
		return
	}

	// Delay to allow audio device to settle
	time.Sleep(2 * time.Second)

	logger.Debugf("Resuming media playback via playerctl")
	cmd := exec.Command("playerctl", "play")
	if err := cmd.Run(); err != nil {
		logger.Debugf("playerctl play failed, trying XF86AudioPlay: %v", err)
		xdoCmd := exec.Command("xdotool", "key", "XF86AudioPlay")
		if err2 := xdoCmd.Run(); err2 != nil {
			logger.Debugf("XF86AudioPlay also failed: %v", err2)
		}
	}
}

// stopPostTranscriptionAutoPasteAsync stops recording in auto-paste mode
func (s *Server) stopPostTranscriptionAutoPasteAsync() {
	go func() {
		logger.Debugf("D-Bus: Stopping post-transcription auto-paste recording")

		transcription, err := s.recorder.Stop()
		if err != nil {
			logger.Errorf("D-Bus: Error stopping recording", err)
			s.emitSignal("RecordingError", err.Error())
			s.postTranscriptionAutoPaste = false
			s.resumeMediaPlayback()
			return
		}

		// Stop forwarding levels
		s.stopForwardingLevels()

		// Resume media playback after recording stops
		go s.resumeMediaPlayback()

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
		if err != nil {
			logger.Errorf("D-Bus: Error stopping recording", err)
			s.emitSignal("RecordingError", err.Error())
			s.postTranscriptionRouterMode = false
			s.resumeMediaPlayback()
			return
		}

		// Stop forwarding levels
		s.stopForwardingLevels()

		// Resume media playback after recording stops
		go s.resumeMediaPlayback()

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
