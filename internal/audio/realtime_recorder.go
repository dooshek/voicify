package audio

import (
	"context"
	"fmt"
	"sync"

	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/notification"
	"github.com/dooshek/voicify/internal/transcriber"
	"github.com/gen2brain/malgo"
)

const (
	// Per OpenAI Realtime transcription docs, PCM16 must be 24kHz mono
	realtimeSampleRate = 24000                                      // 24 kHz PCM16 mono required
	realtimeChannels   = 1                                          // Mono
	audioChunkMs       = 50                                         // Send audio chunks every 50ms
	audioChunkSamples  = (realtimeSampleRate * audioChunkMs) / 1000 // samples per chunk
	audioChunkBytes    = audioChunkSamples * 2                      // 16-bit = 2 bytes per sample
)

// RealtimeRecorder handles real-time recording and transcription
type RealtimeRecorder struct {
	isRecording bool
	cancelled   bool
	transcriber *transcriber.RealtimeTranscriber
	notifier    notification.Notifier
	mu          sync.Mutex

	// Channels for streaming results
	partialChan  chan string
	completeChan chan string
	errorChan    chan error

	// Audio level tracking
	level *LevelProcessor

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRealtimeRecorder creates a new real-time recorder
func NewRealtimeRecorder() (*RealtimeRecorder, error) {
	return NewRealtimeRecorderWithNotifier(notification.New())
}

// NewRealtimeRecorderWithNotifier creates a real-time recorder with custom notifier
func NewRealtimeRecorderWithNotifier(notifier notification.Notifier) (*RealtimeRecorder, error) {
	transcriber, err := transcriber.NewRealtimeTranscriber()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize realtime transcriber: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &RealtimeRecorder{
		transcriber:  transcriber,
		notifier:     notifier,
		partialChan:  make(chan string, 50),
		completeChan: make(chan string, 10),
		errorChan:    make(chan error, 10),
		level:        NewLevelProcessor(),
		ctx:          ctx,
		cancel:       cancel,
	}, nil
}

// IsRecording returns whether the recorder is currently recording
func (rr *RealtimeRecorder) IsRecording() bool {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.isRecording
}

// Start begins real-time recording and transcription
func (rr *RealtimeRecorder) Start() error {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if rr.isRecording {
		return fmt.Errorf("already recording")
	}

	// Recreate context for this recording session
	rr.ctx, rr.cancel = context.WithCancel(context.Background())

	// Start the transcriber WebSocket connection
	if err := rr.transcriber.Start(); err != nil {
		return fmt.Errorf("failed to start transcriber: %w", err)
	}

	rr.isRecording = true
	rr.cancelled = false

	// Start audio recording in background
	go rr.recordAndStream()

	// Start transcript forwarding
	go rr.forwardTranscripts()

	logger.Infof("🎙️ Real-time recording started...")
	rr.notifier.NotifyRecordingStarted()
	rr.notifier.PlayStartBeep()

	return nil
}

// Stop ends the recording and returns accumulated transcription
func (rr *RealtimeRecorder) Stop() (string, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if !rr.isRecording {
		return "", nil
	}

	rr.isRecording = false
	rr.cancel() // Cancel context to stop goroutines
	rr.transcriber.Stop()
	rr.notifier.PlayStopBeep()

	// TODO: Accumulate and return final transcription
	// For now, just return empty - the real-time functionality
	// sends transcripts through channels, not return values
	return "", nil
}

// Cancel cancels the current recording
func (rr *RealtimeRecorder) Cancel() {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if !rr.isRecording {
		return
	}

	logger.Debugf("Cancelling real-time recording")
	rr.isRecording = false
	rr.cancelled = true
	rr.cancel()
	rr.transcriber.Stop()
	rr.notifier.PlayStopBeep()
}

// PartialChan returns channel for partial transcription results
func (rr *RealtimeRecorder) PartialChan() <-chan string {
	return rr.partialChan
}

// CompleteChan returns channel for complete transcription results
func (rr *RealtimeRecorder) CompleteChan() <-chan string {
	return rr.completeChan
}

// ErrorChan returns channel for errors
func (rr *RealtimeRecorder) ErrorChan() <-chan error {
	return rr.errorChan
}

// LevelChan returns channel for audio levels
func (rr *RealtimeRecorder) LevelChan() <-chan float64 {
	return rr.level.LevelChan
}

// SetRealtimeModel sets the transcription model for the next realtime session
func (rr *RealtimeRecorder) SetRealtimeModel(model string) {
	rr.transcriber.SetModel(model)
}

// recordAndStream captures audio and streams it to OpenAI WebSocket
func (rr *RealtimeRecorder) recordAndStream() {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		logger.Error("Error initializing audio context", err)
		rr.errorChan <- fmt.Errorf("failed to initialize audio context: %w", err)
		return
	}
	defer ctx.Uninit()

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = uint32(realtimeChannels)
	deviceConfig.SampleRate = uint32(realtimeSampleRate)
	deviceConfig.Alsa.NoMMap = 1

	// Buffer to accumulate audio before sending
	audioBuffer := make([]byte, 0, audioChunkBytes*2) // Double size for safety

	device, err := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: func(outputBuffer, inputBuffer []byte, frameCount uint32) {
			if !rr.isRecording {
				return
			}

			// Add to buffer
			audioBuffer = append(audioBuffer, inputBuffer...)

			// Process audio levels
			rr.level.Process(inputBuffer)

			// Send chunk when we have enough data
			if len(audioBuffer) >= audioChunkBytes {
				// Take exactly audioChunkBytes
				chunk := audioBuffer[:audioChunkBytes]
				audioBuffer = audioBuffer[audioChunkBytes:]

				// Send to transcriber (non-blocking)
				go func() {
					if err := rr.transcriber.SendAudio(chunk); err != nil {
						logger.Errorf("Failed to send audio chunk", err)
						rr.errorChan <- fmt.Errorf("failed to send audio: %w", err)
					}
				}()
			}
		},
	})
	if err != nil {
		logger.Error("Error initializing audio device", err)
		rr.errorChan <- fmt.Errorf("failed to initialize audio device: %w", err)
		return
	}
	defer device.Uninit()

	device.Start()

	// Wait until recording stops or context is cancelled
	<-rr.ctx.Done()
}

// forwardTranscripts forwards transcription results from transcriber to recorder channels
func (rr *RealtimeRecorder) forwardTranscripts() {
	for {
		select {
		case <-rr.ctx.Done():
			return
		case partial := <-rr.transcriber.PartialChan():
			select {
			case rr.partialChan <- partial:
			default:
				// Drop if channel is full
			}
		case complete := <-rr.transcriber.TranscriptChan():
			select {
			case rr.completeChan <- complete:
			default:
				// Drop if channel is full
			}
		case err := <-rr.transcriber.ErrorChan():
			select {
			case rr.errorChan <- err:
			default:
				// Drop if channel is full
			}
		}
	}
}
