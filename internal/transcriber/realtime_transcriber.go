package transcriber

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/state"
	"github.com/gorilla/websocket"
)

// RealtimeTranscriber handles real-time transcription using OpenAI WebSocket API
type RealtimeTranscriber struct {
	apiKey         string
	conn           *websocket.Conn
	mu             sync.Mutex
	isActive       bool
	transcriptChan chan string
	partialChan    chan string
	errorChan      chan error
	ctx            context.Context
	cancel         context.CancelFunc
	model          string
}

// RealtimeTranscriptionSession represents OpenAI session response
type RealtimeTranscriptionSession struct {
	Object                   string                `json:"object"`
	ID                       string                `json:"id"`
	InputAudioFormat         string                `json:"input_audio_format"`
	InputAudioTranscription  TranscriptionConfig   `json:"input_audio_transcription"`
	TurnDetection            *TurnDetectionConfig  `json:"turn_detection"`
	InputAudioNoiseReduction *NoiseReductionConfig `json:"input_audio_noise_reduction"`
	Include                  []string              `json:"include,omitempty"`
}

type TranscriptionConfig struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt,omitempty"`
	Language string `json:"language,omitempty"`
}

type TurnDetectionConfig struct {
	Type              string  `json:"type"`
	Threshold         float64 `json:"threshold"`
	PrefixPaddingMs   int     `json:"prefix_padding_ms"`
	SilenceDurationMs int     `json:"silence_duration_ms"`
}

type NoiseReductionConfig struct {
	Type string `json:"type"`
}

// WebSocket event types
type WSEvent struct {
	Type       string `json:"type"`
	EventID    string `json:"event_id,omitempty"`
	Audio      string `json:"audio,omitempty"`
	ItemID     string `json:"item_id,omitempty"`
	Delta      string `json:"delta,omitempty"`
	Transcript string `json:"transcript,omitempty"`
	Error      *struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
		Param   string `json:"param"`
	} `json:"error,omitempty"`
}

type SessionUpdateEvent struct {
	Type                     string                `json:"type"`
	InputAudioFormat         string                `json:"input_audio_format"`
	InputAudioTranscription  TranscriptionConfig   `json:"input_audio_transcription"`
	TurnDetection            *TurnDetectionConfig  `json:"turn_detection"`
	InputAudioNoiseReduction *NoiseReductionConfig `json:"input_audio_noise_reduction"`
	Include                  []string              `json:"include,omitempty"`
}

// SessionConfig is a minimal config used in update payload
type SessionConfig struct {
	InputAudioFormat         string                `json:"input_audio_format"`
	InputAudioTranscription  TranscriptionConfig   `json:"input_audio_transcription"`
	TurnDetection            *TurnDetectionConfig  `json:"turn_detection"`
	InputAudioNoiseReduction *NoiseReductionConfig `json:"input_audio_noise_reduction"`
	Include                  []string              `json:"include,omitempty"`
}

// SessionUpdateEnvelope is the shape expected by the API: { type, session: { ... } }
type SessionUpdateEnvelope struct {
	Type    string        `json:"type"`
	Session SessionConfig `json:"session"`
}

type AudioAppendEvent struct {
	Type  string `json:"type"`
	Audio string `json:"audio"`
}

// NewRealtimeTranscriber creates a new real-time transcriber
func NewRealtimeTranscriber() (*RealtimeTranscriber, error) {
	config := state.Get().Config
	if config.LLM.Keys.OpenAIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &RealtimeTranscriber{
		apiKey:         config.LLM.Keys.OpenAIKey,
		transcriptChan: make(chan string, 100),
		partialChan:    make(chan string, 100),
		errorChan:      make(chan error, 10),
		ctx:            ctx,
		cancel:         cancel,
	}, nil
}

// Start initializes the WebSocket connection and starts transcription
func (rt *RealtimeTranscriber) Start() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.isActive {
		return fmt.Errorf("realtime transcriber already active")
	}

	// Recreate context for this transcription session
	rt.ctx, rt.cancel = context.WithCancel(context.Background())

	// Connect directly to WebSocket (no session creation needed)
	if err := rt.connectWebSocket(); err != nil {
		return fmt.Errorf("failed to connect WebSocket: %w", err)
	}

	// Start message handling
	go rt.handleMessages()

	// Send session configuration directly via WebSocket
	if err := rt.configureSession(); err != nil {
		rt.Stop()
		return fmt.Errorf("failed to configure session: %w", err)
	}

	rt.isActive = true
	logger.Infof("🎙️ Real-time transcription started")
	return nil
}

// Stop closes the WebSocket connection and cleans up
func (rt *RealtimeTranscriber) Stop() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if !rt.isActive {
		return
	}

	rt.isActive = false
	rt.cancel()

	if rt.conn != nil {
		rt.conn.Close()
		rt.conn = nil
	}

	logger.Infof("🎙️ Real-time transcription stopped")
}

// SendAudio sends PCM audio data to the WebSocket
func (rt *RealtimeTranscriber) SendAudio(pcmData []byte) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if !rt.isActive || rt.conn == nil {
		return fmt.Errorf("transcriber not active")
	}

	// Convert PCM to base64
	audioData := fmt.Sprintf(`{"type":"input_audio_buffer.append","audio":"%s"}`,
		encodeAudioToBase64(pcmData))

	return rt.conn.WriteMessage(websocket.TextMessage, []byte(audioData))
}

// SetModel sets the transcription model for the next session
func (rt *RealtimeTranscriber) SetModel(model string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.model = model
}

// TranscriptChan returns channel for complete transcripts
func (rt *RealtimeTranscriber) TranscriptChan() <-chan string {
	return rt.transcriptChan
}

// PartialChan returns channel for partial transcripts
func (rt *RealtimeTranscriber) PartialChan() <-chan string {
	return rt.partialChan
}

// ErrorChan returns channel for errors
func (rt *RealtimeTranscriber) ErrorChan() <-chan error {
	return rt.errorChan
}

// connectWebSocket establishes WebSocket connection to OpenAI
func (rt *RealtimeTranscriber) connectWebSocket() error {
	u := url.URL{
		Scheme:   "wss",
		Host:     "api.openai.com",
		Path:     "/v1/realtime",
		RawQuery: "intent=transcription",
	}

	header := http.Header{}
	// Use API key directly for WebSocket auth (no session needed)
	header.Add("Authorization", "Bearer "+rt.apiKey)
	header.Add("OpenAI-Beta", "realtime=v1")

	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}

	conn, _, err := dialer.Dial(u.String(), header)
	if err != nil {
		return fmt.Errorf("failed to dial WebSocket: %w", err)
	}

	rt.conn = conn
	logger.Debugf("WebSocket connected to OpenAI Realtime API")
	return nil
}

// configureSession sends session configuration to the WebSocket
func (rt *RealtimeTranscriber) configureSession() error {
	// Use nested session object per API: { type: "transcription_session.update", session: { ... } }
	env := SessionUpdateEnvelope{
		Type: "transcription_session.update",
		Session: SessionConfig{
			InputAudioFormat: "pcm16",
			InputAudioTranscription: TranscriptionConfig{
				Model:    rt.getModel(),
				Language: "pl",
			},
			TurnDetection: &TurnDetectionConfig{
				Type:              "server_vad",
				Threshold:         0.5,
				PrefixPaddingMs:   300,
				SilenceDurationMs: 500,
			},
			InputAudioNoiseReduction: &NoiseReductionConfig{
				Type: "near_field",
			},
			// Do not set Include here; unsupported values cause API errors. If needed,
			// valid option is: "item.input_audio_transcription.logprobs".
		},
	}

	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("failed to marshal session update: %w", err)
	}

	return rt.conn.WriteMessage(websocket.TextMessage, data)
}

func (rt *RealtimeTranscriber) getModel() string {
	if rt.model != "" {
		return rt.model
	}
	return "gpt-4o-mini-transcribe"
}

// handleMessages processes incoming WebSocket messages
func (rt *RealtimeTranscriber) handleMessages() {
	defer rt.cancel()

	for {
		select {
		case <-rt.ctx.Done():
			return
		default:
		}

		_, message, err := rt.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Errorf("WebSocket error", err)
				rt.errorChan <- err
			}
			return
		}

		var event WSEvent
		if err := json.Unmarshal(message, &event); err != nil {
			logger.Errorf("Failed to unmarshal WebSocket message", err)
			continue
		}

		rt.processEvent(event)
	}
}

// processEvent handles different types of WebSocket events
func (rt *RealtimeTranscriber) processEvent(event WSEvent) {
	switch event.Type {
	case "input_audio_transcription.delta":
		if event.Delta != "" {
			logger.Debugf("📝 Partial transcript: %s", event.Delta)
			select {
			case rt.partialChan <- event.Delta:
			default:
				// Drop if channel is full
			}
		}
	case "input_audio_transcription.completed":
		if event.Transcript != "" {
			logger.Debugf("📝 Complete transcript: %s", event.Transcript)
			select {
			case rt.transcriptChan <- event.Transcript:
			default:
				// Drop if channel is full
			}
		}
	case "conversation.item.input_audio_transcription.delta":
		if event.Delta != "" {
			logger.Debugf("📝 Partial transcript: %s", event.Delta)
			select {
			case rt.partialChan <- event.Delta:
			default:
				// Drop if channel is full
			}
		}

	case "conversation.item.input_audio_transcription.completed":
		if event.Transcript != "" {
			logger.Debugf("📝 Complete transcript: %s", event.Transcript)
			select {
			case rt.transcriptChan <- event.Transcript:
			default:
				// Drop if channel is full
			}
		}

	case "error":
		if event.Error != nil {
			logger.Errorf("OpenAI API error", fmt.Errorf("%s (%s): %s", event.Error.Type, event.Error.Code, event.Error.Message))
			rt.errorChan <- fmt.Errorf("OpenAI API error: %s (%s): %s", event.Error.Type, event.Error.Code, event.Error.Message)
		} else {
			logger.Errorf("OpenAI API error in event", fmt.Errorf("event: %+v", event))
			rt.errorChan <- fmt.Errorf("OpenAI API error: %+v", event)
		}

	default:
		logger.Debugf("Received WebSocket event: %s", event.Type)
	}
}

// encodeAudioToBase64 converts PCM audio data to base64 string
func encodeAudioToBase64(pcmData []byte) string {
	// PCM data is already in the correct format (16-bit signed integers)
	// Encode to base64 for WebSocket transmission
	return base64.StdEncoding.EncodeToString(pcmData)
}
