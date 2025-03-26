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
	bufferSize = 1024
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
	recordingStartTime time.Time
	transcriber        *transcriber.Transcriber
	notifier           notification.Notifier
	fileOps            fileops.FileOps
	resultChan         chan recordingResult
}

type recordingResult struct {
	transcription string
	err           error
}

func NewRecorder() (*Recorder, error) {
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
		notifier:    notification.New(),
		fileOps:     fileOps,
		resultChan:  make(chan recordingResult, 1),
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
	r.recordingStartTime = time.Now()
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

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	wavFilename := fmt.Sprintf("recording_%s.wav", timestamp)
	oggFilename := strings.TrimSuffix(wavFilename, ".wav") + ".ogg"

	wavData, err := ConvertPCMToWAV(audioBuffer.Bytes(), channels, sampleRate)
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
	for {
		select {
		case <-ticker.C:
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
}

func ConvertPCMToWAV(pcmData []byte, channels int, sampleRate int) ([]byte, error) {
	var buffer bytes.Buffer

	// Write WAV header
	binary.Write(&buffer, binary.LittleEndian, []byte("RIFF"))
	binary.Write(&buffer, binary.LittleEndian, uint32(len(pcmData)+36))
	binary.Write(&buffer, binary.LittleEndian, []byte("WAVE"))
	binary.Write(&buffer, binary.LittleEndian, []byte("fmt "))
	binary.Write(&buffer, binary.LittleEndian, uint32(16))
	binary.Write(&buffer, binary.LittleEndian, uint16(1))
	binary.Write(&buffer, binary.LittleEndian, uint16(channels))
	binary.Write(&buffer, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buffer, binary.LittleEndian, uint32(sampleRate*channels*2))
	binary.Write(&buffer, binary.LittleEndian, uint16(channels*2))
	binary.Write(&buffer, binary.LittleEndian, uint16(16))
	binary.Write(&buffer, binary.LittleEndian, []byte("data"))
	binary.Write(&buffer, binary.LittleEndian, uint32(len(pcmData)))
	buffer.Write(pcmData)

	return buffer.Bytes(), nil
}
