package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
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
	level *LevelProcessor
	// PipeWire source for BT handsfree (empty = use miniaudio default)
	captureSource string
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
		level:       NewLevelProcessor(),
	}, nil
}

func (r *Recorder) IsRecording() bool {
	return r.isRecording
}

// SetCaptureSource sets a specific PipeWire source for recording (e.g. BT handsfree mic).
// Pass empty string to use default device via miniaudio.
func (r *Recorder) SetCaptureSource(source string) {
	r.captureSource = source
}

func (r *Recorder) Start() {
	if r.isRecording {
		return
	}

	r.isRecording = true
	r.cancelled = false
	r.recordingStartTime = time.Now()
	if r.captureSource != "" {
		go r.recordWithParec()
	} else {
		go r.record()
	}
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

			// Compute and emit input level
			r.level.Process(inputBuffer)
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

	oggPath := filepath.Join(r.fileOps.GetRecordingsDir(), oggFilename)
	err = ffmpeg.Input(wavPath).
		Output(oggPath, ffmpeg.KwArgs{
			"loglevel":          "quiet",
			"acodec":            "libvorbis",
			"b:a":               "24k",
			"ar":                "16000",
			"compression_level": "5",
			"threads":           "auto",
			"af":                "silenceremove=start_periods=1:start_duration=0.1:start_threshold=-50dB:detection=peak:stop_periods=-1:stop_duration=0.2:stop_threshold=-50dB",
		}).
		OverWriteOutput().
		Run()
	if err != nil {
		logger.Error("Error converting to Ogg Vorbis", err)
		return
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

// recordWithParec records via parecord (PipeWire native) from a specific source.
// Used for BT handsfree - miniaudio doesn't follow PipeWire routing.
func (r *Recorder) recordWithParec() {
	source := r.captureSource
	r.captureSource = "" // clear for next recording
	logger.Infof("Recording via parecord from source: %s", source)

	cmd := exec.Command("parecord",
		"--device="+source,
		"--format=s16le",
		"--rate=16000",
		"--channels=1",
		"--raw",
		"--latency-msec=10",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Error("Error creating parecord pipe", err)
		r.resultChan <- recordingResult{"", fmt.Errorf("parecord pipe error: %w", err)}
		return
	}

	if err := cmd.Start(); err != nil {
		logger.Error("Error starting parecord", err)
		r.resultChan <- recordingResult{"", fmt.Errorf("parecord start error: %w", err)}
		return
	}

	var audioBuffer bytes.Buffer
	// Small buffer for responsive level updates (320 bytes = 10ms at 16kHz mono s16le)
	buf := make([]byte, 320)

	// Diagnostic state for the recording loop. These exist so we can tell
	// the difference, after the fact, between:
	//   * parecord never streamed (BT SCO dead from the start)
	//   * parecord streamed silence (BT SCO half-dead)
	//   * parecord streamed normally then went silent (something stole the
	//     source mid-recording: profile flip, default-source change)
	//   * parecord stopped streaming entirely mid-recording (EOF)
	startTime := time.Now()
	lastLogTime := startTime
	var bytesSinceLog int64
	var samplesSinceLog int64
	var sumAbsSinceLog int64
	totalBytes := int64(0)
	silenceWindowStart := time.Time{} // when current run of silence began
	silenceLogged := false

	for r.isRecording {
		n, err := stdout.Read(buf)
		if err != nil {
			logger.Errorf("parecord read error after %d bytes (%v elapsed)", err,
				totalBytes, time.Since(startTime))
			break
		}
		if n > 0 {
			audioBuffer.Write(buf[:n])
			r.level.Process(buf[:n])

			totalBytes += int64(n)
			bytesSinceLog += int64(n)

			// Sum abs s16le samples for amplitude logging
			for i := 0; i+1 < n; i += 2 {
				v := int16(buf[i]) | int16(buf[i+1])<<8
				if v < 0 {
					v = -v
				}
				sumAbsSinceLog += int64(v)
				samplesSinceLog++

				// Track contiguous silence windows. avg < ~30 ≈ near-zero.
				// This is checked per-buffer-end below to avoid extra work.
			}

			// Per-second progress log. Reports byte rate (should be ~32000 B/s
			// at 16kHz s16le mono) and average amplitude (silence ≈ 1, speech
			// well into the hundreds/thousands).
			now := time.Now()
			if now.Sub(lastLogTime) >= time.Second {
				avg := int64(0)
				if samplesSinceLog > 0 {
					avg = sumAbsSinceLog / samplesSinceLog
				}
				logger.Infof("parecord: t=%.1fs total=%dB last1s=%dB avgAmp=%d",
					now.Sub(startTime).Seconds(), totalBytes, bytesSinceLog, avg)

				// If the last second was effectively silent, mark it so we
				// know whether the cap was already silent at end-of-recording.
				if avg <= 4 {
					if silenceWindowStart.IsZero() {
						silenceWindowStart = lastLogTime
					}
					if !silenceLogged && now.Sub(silenceWindowStart) >= 2*time.Second {
						logger.Errorf("parecord: silence >=2s detected (avgAmp=%d) - SCO link likely dead",
							fmt.Errorf("source=%s", source), avg)
						silenceLogged = true
					}
				} else {
					if silenceLogged {
						logger.Infof("parecord: signal recovered after silence")
					}
					silenceWindowStart = time.Time{}
					silenceLogged = false
				}

				lastLogTime = now
				bytesSinceLog = 0
				samplesSinceLog = 0
				sumAbsSinceLog = 0
			}
		}
	}

	cmd.Process.Kill()
	cmd.Wait()

	logger.Infof("Parecord captured %d bytes (%.1f seconds)", audioBuffer.Len(), float64(audioBuffer.Len())/(2.0*float64(sampleRate)))

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
		r.resultChan <- recordingResult{"", fmt.Errorf("wav conversion error: %w", err)}
		return
	}

	wavPath := filepath.Join(r.fileOps.GetRecordingsDir(), wavFilename)
	os.WriteFile(wavPath, wavData, 0o644)

	logger.Info("🎙️ Processing audio...")

	// BT handsfree mic is inherently quiet (~-70dB) - boost 30dB before silence removal
	oggPath := filepath.Join(r.fileOps.GetRecordingsDir(), oggFilename)
	err = ffmpeg.Input(wavPath).
		Output(oggPath, ffmpeg.KwArgs{
			"loglevel":          "quiet",
			"acodec":            "libvorbis",
			"b:a":               "24k",
			"ar":                "16000",
			"compression_level": "5",
			"threads":           "auto",
			"af":                "volume=10dB,silenceremove=start_periods=1:start_duration=0.1:start_threshold=-60dB:detection=peak:stop_periods=-1:stop_duration=0.2:stop_threshold=-60dB",
		}).
		OverWriteOutput().
		Run()
	if err != nil {
		logger.Error("Error converting to Ogg Vorbis", err)
		r.resultChan <- recordingResult{"", fmt.Errorf("ogg conversion error: %w", err)}
		return
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
	logger.Debugf("Transcription took: %d ms", time.Since(transcriptionStartTime).Milliseconds())
	logger.Infof("📝 Transcription: %s", text)

	r.notifier.NotifyTranscriptionComplete()
	r.notifier.PlayTranscriptionOverBeep()

	os.Remove(wavPath)
	os.Remove(oggPath)

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

// LevelChan returns a channel with live input level values in range [0, 1]
// Values are emitted roughly every 40ms during active recording
func (r *Recorder) LevelChan() <-chan float64 {
	return r.level.LevelChan
}
