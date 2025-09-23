package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dooshek/voicify/internal/fileops"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/notification"
	"github.com/dooshek/voicify/internal/transcriber"
	"github.com/gen2brain/malgo"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

const (
	sampleRate = 16000
	channels   = 1
	bufferSize = 1024
)

// --- Automatic Gain Control (AGC) configuration ---
// agcAttackMs: Envelope attack time (ms). Mniejsze = szybciej reaguje na wzrosty (bardziej "żywe").
// agcReleaseMs: Envelope release time (ms). Większe = wolniejszy opad (stabilniejsza kreska).
// agcTarget: Docelowy poziom envelope po wzmocnieniu (skala 0..~1 przed logarytmicznym mapowaniem).
// agcMaxGain: Maksymalne wzmocnienie (x). Podnosi szept, ale za duże może klipować wizualizację.
// agcMinGain: Minimalne wzmocnienie (x). Zapobiega zbyt niskiemu poziomowi przy głośnym sygnale.
// agcGainAttack: Szybkość narastania GAIN (0..1). Większe = szybciej podbija ciche fragmenty.
// agcGainRelease: Szybkość opadania GAIN (0..1). Większe = szybciej zmniejsza gain po głośnym sygnale.
// agcNoiseGate: Próg bramki szumów dla WYJŚCIA (nie zeruje envelope). Poniżej progu nie rysujemy kreski.
// agcVisualBoost: Dodatkowy mnożnik na wyjściu (tylko do wizualizacji, nie wpływa na gain).
// uiMaxLevel: Maksymalny poziom dla UI (docinamy do tej wartości po skali logarytmicznej).
const (
	agcAttackMs    = 20.0  // ms
	agcReleaseMs   = 350.0 // ms
	agcTarget      = 1.2
	agcMaxGain     = 6.0
	agcMinGain     = 0.1
	agcGainAttack  = 0.03
	agcGainRelease = 0.03
	agcNoiseGate   = 0.002
	agcVisualBoost = 1.5
	uiMaxLevel     = 1.0
)

var ErrFFmpegNotInstalled = fmt.Errorf("FFmpeg is not installed. Please install FFmpeg to use voice recording functionality")

func checkFFmpegInstalled() error {
	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return ErrFFmpegNotInstalled
	}
	return nil
}

func init() {
	ffmpeg.LogCompiledCommand = false
}

type Recorder struct {
	isRecording        bool
	cancelled          bool
	recordingStartTime time.Time
	transcriber        *transcriber.Transcriber
	notifier           notification.Notifier
	fileOps            fileops.FileOps
	resultChan         chan recordingResult
	// Live input level streaming
	levelChan     chan float64
	lastLevelEmit time.Time
	// AGC state (peak-based)
	agcGain float64
	agcEnv  float64
}

type recordingResult struct {
	transcription string
	err           error
}

func NewRecorder() (*Recorder, error) {
	return NewRecorderWithNotifier(notification.New())
}

func NewRecorderWithNotifier(notifier notification.Notifier) (*Recorder, error) {
	if err := checkFFmpegInstalled(); err != nil {
		return nil, err
	}

	fileOps, err := fileops.NewDefaultFileOps()
	if err != nil {
		logger.Error("Failed to initialize file operations", err)
		return nil, fmt.Errorf("failed to initialize file operations: %w", err)
	}

	if err := fileOps.EnsureDirectories(); err != nil {
		logger.Error("Failed to create directories", err)
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	transcriber, err := transcriber.NewTranscriber()
	if err != nil {
		logger.Error("Failed to initialize transcriber", err)
		return nil, fmt.Errorf("failed to initialize transcriber: %w", err)
	}

	return &Recorder{
		transcriber: transcriber,
		notifier:    notifier,
		fileOps:     fileOps,
		resultChan:  make(chan recordingResult, 1),
		levelChan:   make(chan float64, 16),
		agcGain:     1.0,
		agcEnv:      0.0,
	}, nil
}

func (r *Recorder) IsRecording() bool {
	return r.isRecording
}

func (r *Recorder) Start() {
	if r.isRecording {
		return
	}

	r.isRecording = true
	r.cancelled = false
	r.recordingStartTime = time.Now()
	// Reset metering state
	r.lastLevelEmit = time.Time{}
	go r.record()
	go r.updateRecordingTime()

	logger.Info("🎙️  Recording started...")
	logger.Info("Press same key again to stop recording")

	r.notifier.NotifyRecordingStarted()
	r.notifier.PlayStartBeep()
}

func (r *Recorder) Stop() (string, error) {
	if !r.isRecording {
		return "", nil
	}
	r.isRecording = false

	r.notifier.PlayStopBeep()

	// Wait for the recording result
	result := <-r.resultChan

	return result.transcription, result.err
}

// Cancel cancels the current recording without processing
func (r *Recorder) Cancel() {
	if !r.isRecording {
		return
	}

	logger.Debugf("Cancelling recording")
	r.isRecording = false
	r.cancelled = true

	r.notifier.PlayStopBeep()
}

func (r *Recorder) record() {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		logger.Error("Error initializing context", err)
		return
	}
	defer ctx.Uninit()

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = uint32(channels)
	deviceConfig.SampleRate = uint32(sampleRate)
	deviceConfig.Alsa.NoMMap = 1

	var audioBuffer bytes.Buffer

	device, err := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: func(outputBuffer, inputBuffer []byte, frameCount uint32) {
			if !r.isRecording {
				return
			}
			audioBuffer.Write(inputBuffer)

			// Compute and emit input level (RMS with EMA + simple AGC)
			r.processInputLevel(inputBuffer)
		},
	})
	if err != nil {
		logger.Error("Error initializing device", err)
		return
	}
	defer device.Uninit()

	device.Start()

	for r.isRecording {
		time.Sleep(100 * time.Millisecond)
	}

	// Check if recording was cancelled
	if r.cancelled {
		logger.Debugf("Recording was cancelled, discarding audio data")
		return
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	wavFilename := fmt.Sprintf("recording_%s.wav", timestamp)
	oggFilename := strings.TrimSuffix(wavFilename, ".wav") + ".ogg"

	wavData, err := convertPCMToWAV(audioBuffer.Bytes(), channels, sampleRate)
	if err != nil {
		logger.Error("Error converting to WAV", err)
		return
	}

	wavPath := filepath.Join(r.fileOps.GetRecordingsDir(), wavFilename)
	os.WriteFile(wavPath, wavData, 0o644)

	logger.Info("🎙️ Processing audio...")
	oggStartTime := time.Now()

	oggPath := filepath.Join(r.fileOps.GetRecordingsDir(), oggFilename)
	err = ffmpeg.Input(wavPath).
		Output(oggPath, ffmpeg.KwArgs{
			"loglevel":          "quiet",
			"acodec":            "libvorbis",
			"b:a":               "24k",
			"ar":                "16000",
			"compression_level": "5",
			"threads":           "auto",
		}).
		OverWriteOutput().
		Run()
	if err != nil {
		logger.Error("Error converting to Ogg Vorbis", err)
		return
	}

	oggProcessingTime := time.Since(oggStartTime)

	if fileInfo, err := os.Stat(oggPath); err == nil {
		logger.Debugf("Conversion from WAV to Ogg Vorbis took: %d ms, file size is: %.2f kB", oggProcessingTime.Milliseconds(), float64(fileInfo.Size())/1024)
	}

	logger.Info("🎙️ Transcribing audio...")
	r.notifier.NotifyTranscribing()

	transcriptionStartTime := time.Now()
	text, err := r.transcriber.TranscribeFile(oggPath)
	if err != nil {
		logger.Error("Transcription failed", err)
		r.resultChan <- recordingResult{"", fmt.Errorf("transcription error: %w", err)}
		return
	}
	transcriptionTime := time.Since(transcriptionStartTime)
	logger.Debugf("Transcription took: %d ms", transcriptionTime.Milliseconds())

	logger.Infof("📝 Transcription: %s", text)

	r.notifier.NotifyTranscriptionComplete()
	r.notifier.PlayTranscriptionOverBeep()

	// Clean up temporary files
	os.Remove(wavPath)
	os.Remove(oggPath)

	// Send the result
	r.resultChan <- recordingResult{text, nil}
}

func (r *Recorder) updateRecordingTime() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	progressCounter := 0
	for range ticker.C {
		if r.isRecording {
			progressCounter++
			if progressCounter >= 4 { // Co 2 sekundy
				r.notifier.PlayProgressBeep()
				progressCounter = 0
			}
		} else {
			return
		}
	}
}

func convertPCMToWAV(pcmData []byte, channels int, sampleRate int) ([]byte, error) {
	var buffer bytes.Buffer

	// Write WAV header
	binary.Write(&buffer, binary.LittleEndian, []byte("RIFF"))
	binary.Write(&buffer, binary.LittleEndian, uint32(len(pcmData)+36))
	binary.Write(&buffer, binary.LittleEndian, []byte("WAVE"))

	// "fmt " chunk
	binary.Write(&buffer, binary.LittleEndian, []byte("fmt "))
	binary.Write(&buffer, binary.LittleEndian, uint32(16))
	binary.Write(&buffer, binary.LittleEndian, uint16(1))
	binary.Write(&buffer, binary.LittleEndian, uint16(channels))
	binary.Write(&buffer, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buffer, binary.LittleEndian, uint32(sampleRate*channels*2))
	binary.Write(&buffer, binary.LittleEndian, uint16(channels*2))
	binary.Write(&buffer, binary.LittleEndian, uint16(16))

	// "data" chunk
	binary.Write(&buffer, binary.LittleEndian, []byte("data"))
	binary.Write(&buffer, binary.LittleEndian, uint32(len(pcmData)))
	binary.Write(&buffer, binary.LittleEndian, pcmData)

	return buffer.Bytes(), nil
}

// LevelChan returns a channel with live input level values in range [0, 1.2]
// Values are emitted roughly every 30-40ms during active recording
func (r *Recorder) LevelChan() <-chan float64 {
	return r.levelChan
}

// processInputLevel converts raw PCM samples to simple logarithmic level for visualization
func (r *Recorder) processInputLevel(inputBuffer []byte) {
	if len(inputBuffer) == 0 {
		return
	}

	// Find max absolute sample in this buffer
	// Samples are int16 little-endian, mono
	sampleCount := len(inputBuffer) / 2
	if sampleCount == 0 {
		return
	}

	var maxSample float64
	for i := 0; i < sampleCount; i++ {
		// Little-endian int16 - correct conversion
		s := int16(inputBuffer[2*i]) | int16(inputBuffer[2*i+1])<<8
		absValue := float64(abs(int32(s)))
		if absValue > maxSample {
			maxSample = absValue
		}
	}

	// Peak envelope follower with attack/release
	// Typical values: attack ~5-10 ms, release ~80-200 ms at 16kHz
	attackCoeff := math.Exp(-1.0 / ((agcAttackMs / 1000.0) * float64(sampleRate)))
	releaseCoeff := math.Exp(-1.0 / ((agcReleaseMs / 1000.0) * float64(sampleRate)))

	// Convert max sample to normalized peak [0..1]
	peak := maxSample / 32768.0
	if peak < 0 {
		peak = 0
	} else if peak > 1 {
		peak = 1
	}

	// Update envelope
	if peak > r.agcEnv {
		r.agcEnv = attackCoeff*r.agcEnv + (1-attackCoeff)*peak
	} else {
		r.agcEnv = releaseCoeff*r.agcEnv + (1-releaseCoeff)*peak
	}

	// Noise gate threshold for display (apply later on output)
	gate := agcNoiseGate

	// Target level for envelope after AGC
	target := agcTarget

	// Compute desired gain with soft clamps
	desiredGain := 1.0
	if r.agcEnv > 0 {
		desiredGain = target / r.agcEnv
	}
	if desiredGain > agcMaxGain {
		desiredGain = agcMaxGain
	} else if desiredGain < agcMinGain {
		desiredGain = agcMinGain
	}

	// Smooth gain changes (separate attack/release for gain to avoid pumping)
	gainAttack := agcGainAttack
	gainRelease := agcGainRelease
	if desiredGain > r.agcGain {
		r.agcGain = r.agcGain + gainAttack*(desiredGain-r.agcGain)
	} else {
		r.agcGain = r.agcGain + gainRelease*(desiredGain-r.agcGain)
	}

	// Apply gain to current peak and map to log scale for UI
	adjusted := peak * r.agcGain
	if adjusted > 1 {
		adjusted = 1
	}
	level := 0.0
	if adjusted >= gate {
		level = math.Log10(adjusted*9.0+1.0) * uiMaxLevel * agcVisualBoost
		if level > uiMaxLevel {
			level = uiMaxLevel
		}
	}

	// Throttle emits to ~30ms
	now := time.Now()
	if now.Sub(r.lastLevelEmit) < 25*time.Millisecond {
		return
	}
	r.lastLevelEmit = now

	// Non-blocking send
	select {
	case r.levelChan <- level:
	default:
		// drop if channel is full
	}
}

// sqrt is a tiny helper to avoid importing math for one function
func sqrt(x float64) float64 {
	// Fast enough Newton-Raphson for small inputs
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 6; i++ {
		z = 0.5 * (z + x/z)
	}
	return z
}

// abs returns absolute value of int32
func abs(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// log10 computes log base 10
func log10(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return math.Log10(x)
}
