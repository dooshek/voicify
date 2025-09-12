package dbus

import (
	"context"
	"fmt"
	"sync"

	"github.com/dooshek/voicify/internal/audio"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/notification"
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
	conn     *dbus.Conn
	recorder *audio.Recorder
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
}

// NewServer creates a new D-Bus server instance with silent notifications
func NewServer() (*Server, error) {
	// Use silent notifier for daemon mode - extension handles all UI
	silentNotifier := notification.NewSilent()
	recorder, err := audio.NewRecorderWithNotifier(silentNotifier)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize recorder: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		recorder: recorder,
		ctx:      ctx,
		cancel:   cancel,
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
			Name:    dbusInterface,
			Methods: []introspect.Method{
				{
					Name: "ToggleRecording",
				},
				{
					Name: "GetStatus",
					Args: []introspect.Arg{
						{Name: "is_recording", Type: "b", Direction: "out"},
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
					Name: "RecordingError",
					Args: []introspect.Arg{
						{Name: "error", Type: "s"},
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

// ToggleRecording toggles the recording state (D-Bus method)
func (s *Server) ToggleRecording() *dbus.Error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Debugf("D-Bus: ToggleRecording called")

	if !s.recorder.IsRecording() {
		// Start recording
		logger.Debugf("D-Bus: Starting recording")
		s.recorder.Start()

		// Emit signal
		s.emitSignal("RecordingStarted")

	} else {
		logger.Debugf("D-Bus: Recording already in progress, stopping")
		// Process transcription in background to avoid blocking D-Bus call
		go s.stopRecordingAsync()
	}

	return nil
}

// GetStatus returns current recording status (D-Bus method)
func (s *Server) GetStatus() (bool, *dbus.Error) {
	return s.recorder.IsRecording(), nil
}

// stopRecordingAsync stops recording and handles transcription in background
func (s *Server) stopRecordingAsync() {
	go func() {
		logger.Debugf("D-Bus: Stopping recording and processing transcription")

		transcription, err := s.recorder.Stop()
		if err != nil {
			logger.Errorf("D-Bus: Error stopping recording", err)
			s.emitSignal("RecordingError", err.Error())
			return
		}

		logger.Debugf("D-Bus: Transcription received: %s", transcription)

		// Route transcription through the router (same as keyboard monitor)
		router := transcriptionrouter.New(transcription)
		if err := router.Route(transcription); err != nil {
			logger.Errorf("D-Bus: Error routing transcription", err)
			s.emitSignal("RecordingError", fmt.Sprintf("routing error: %v", err))
			return
		}

		// Emit transcription ready signal
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
		logger.Errorf("D-Bus: Failed to emit signal %s: %v", err, name)
	} else {
		logger.Debugf("D-Bus: Emitted signal: %s", name)
	}
}
