package audio

import (
	"context"
	"fmt"
	"sync"
	"time"

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

	// Audio level tracking (reuse from regular recorder)
	levelChan     chan float64
	lastLevelEmit time.Time
	agcGain       float64
	agcEnv        float64

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
		levelChan:    make(chan float64, 16),
		agcGain:      3.0,
		agcEnv:       0.0,
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

// LevelChan returns channel for audio levels (same as regular recorder)
func (rr *RealtimeRecorder) LevelChan() <-chan float64 {
	return rr.levelChan
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

			// Process audio levels (same logic as regular recorder)
			rr.processInputLevel(inputBuffer)

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

// processInputLevel reuses the same AGC logic from the regular recorder
func (rr *RealtimeRecorder) processInputLevel(inputBuffer []byte) {
	if len(inputBuffer) == 0 {
		return
	}

	// Find max absolute sample in this buffer
	sampleCount := len(inputBuffer) / 2
	if sampleCount == 0 {
		return
	}

	var maxSample float64
	for i := 0; i < sampleCount; i++ {
		s := int16(inputBuffer[2*i]) | int16(inputBuffer[2*i+1])<<8
		absValue := float64(abs(int32(s)))
		if absValue > maxSample {
			maxSample = absValue
		}
	}

	// Same AGC logic as regular recorder
	attackMs := agcAttackMs
	releaseMs := agcReleaseMs
	if agcEnvelopeSizeMs > 0 {
		attackMs = agcEnvelopeSizeMs
		releaseMs = agcEnvelopeSizeMs
	}

	// Calculate coefficients (same formula as regular recorder)
	attackCoeff := 1.0 - (1.0 / ((attackMs / 1000.0) * float64(realtimeSampleRate)))
	releaseCoeff := 1.0 - (1.0 / ((releaseMs / 1000.0) * float64(realtimeSampleRate)))

	if attackCoeff < 0 {
		attackCoeff = 0
	}
	if releaseCoeff < 0 {
		releaseCoeff = 0
	}

	peak := maxSample / 32768.0
	if peak < 0 {
		peak = 0
	} else if peak > 1 {
		peak = 1
	}

	// Update envelope
	if peak > rr.agcEnv {
		rr.agcEnv = attackCoeff*rr.agcEnv + (1-attackCoeff)*peak
	} else {
		rr.agcEnv = releaseCoeff*rr.agcEnv + (1-releaseCoeff)*peak
	}

	// Calculate gain and final level (same logic as regular recorder)
	desiredGain := 1.0
	if rr.agcEnv > 0 {
		desiredGain = agcTarget / rr.agcEnv
	}
	if desiredGain > agcMaxGain {
		desiredGain = agcMaxGain
	} else if desiredGain < agcMinGain {
		desiredGain = agcMinGain
	}

	// Smooth gain changes
	if desiredGain > rr.agcGain {
		rr.agcGain = rr.agcGain + agcGainAttack*(desiredGain-rr.agcGain)
	} else {
		rr.agcGain = rr.agcGain + agcGainRelease*(desiredGain-rr.agcGain)
	}

	// Apply gain and emit level
	adjusted := peak * rr.agcGain
	if adjusted > 1 {
		adjusted = 1
	}

	level := 0.0
	if adjusted >= agcNoiseGate {
		// Same logarithmic mapping as regular recorder
		level = 0.5 * (1.0 + adjusted) * uiMaxLevel * agcVisualBoost
		if level > uiMaxLevel {
			level = uiMaxLevel
		}
	}

	// Throttle emits (same as regular recorder)
	now := time.Now()
	if now.Sub(rr.lastLevelEmit) < throttleIntervalMs*time.Millisecond {
		return
	}
	rr.lastLevelEmit = now

	// Non-blocking send
	select {
	case rr.levelChan <- level:
	default:
		// drop if channel is full
	}
}
