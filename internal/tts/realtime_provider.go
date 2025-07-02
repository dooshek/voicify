package tts

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	openairt "github.com/WqyJh/go-openai-realtime"
	"github.com/dooshek/voicify/internal/logger"
)

// streamingPlayer holds the command and stdin for streaming audio
type streamingPlayer struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
}

// RealtimeTTSProvider implements TTSProvider using OpenAI Realtime API
type RealtimeTTSProvider struct {
	apiKey string
	config RealtimeConfig
}

// RealtimeConfig holds Realtime API TTS configuration
type RealtimeConfig struct {
	Model string  `yaml:"model"` // "gpt-4o-realtime-preview" or "gpt-4o-mini-realtime-preview"
	Speed float64 `yaml:"speed"` // Not directly supported, but can be post-processed
	Voice string  `yaml:"voice"` // "alloy", "echo", "fable", "onyx", "nova", "shimmer"
}

// NewRealtimeTTSProvider creates a new Realtime TTS provider
func NewRealtimeTTSProvider(apiKey string, config RealtimeConfig) *RealtimeTTSProvider {
	// Set defaults
	if config.Model == "" {
		config.Model = "gpt-4o-realtime-preview"
	}
	if config.Speed == 0 {
		config.Speed = 1.0
	}
	if config.Voice == "" {
		config.Voice = "nova"
	}

	return &RealtimeTTSProvider{
		apiKey: apiKey,
		config: config,
	}
}

// Speak converts text to speech using Realtime API and plays it immediately
func (p *RealtimeTTSProvider) Speak(ctx context.Context, text string, voice string) error {
	// Use provided voice or fallback to config
	if voice == "" {
		voice = p.config.Voice
	}

	logger.Infof("Generating Realtime TTS for text (length: %d chars) with voice: %s", len(text), voice)

	// Create Realtime API client
	client := openairt.NewClient(p.apiKey)

	// Connect with context
	conn, err := client.Connect(ctx, openairt.WithModel(p.config.Model))
	if err != nil {
		logger.Error("Failed to connect to Realtime API", err)
		return fmt.Errorf("realtime API connection failed: %w", err)
	}
	defer conn.Close()

	temperature := float32(0.7)

	// Update session to support text input and audio output
	err = conn.SendMessage(ctx, &openairt.SessionUpdateEvent{
		Session: openairt.ClientSession{
			Modalities:        []openairt.Modality{openairt.ModalityText},
			Temperature:       &temperature,
			Voice:             openairt.Voice(voice),
			OutputAudioFormat: openairt.AudioFormatPcm16,
			Instructions: `<rule>
		Czytaj poprawnym językiem polskim, z polskim akcentem.
		Jeśli w tekście pojawiają się angielskie terminy lub całe zdania to czytaj je po angielsku z akcentem angielskim.
		Zawsze musisz czytać dokładnie tekst w tagach text.
		W tagu text znajdują się specjalne tagi, określające sposób mowy, które musisz czytać zgodnie z ich znaczeniem, zdefiniowane w nawiasach {}
		Możliwe tagi określające sposób mowy:
			- {robot} - czytaj monotonnie, nie moduluj głosu, nie zmieniaj tonu głosu, trochę jak robot,
			- {giggles} - zaśmiej się
			- {sad} - czytaj ze smutkiem w głosie
			- {happy} - czytaj ze szczęściem w głosie
			- {laugh} - zaśmiej się krótko, naturalnie, nie zbyt długo
			- {sigh} - westchnij
			- {angry} - czytaj ze złością w głosie
			- {excited} - czytaj ze zachwytem w głosie
			- {fast} - czytaj szybko
			- {emotional} - czytaj emocjonalnie, zmieniaj głos zgodnie z emocją
			- {break} - zrób przerwę na wielokrotne yyy tak jakbyś nie wiedział co powiedzieć
			- {om} - powiedz om, wydaj dźwięk o, jakbyś myślał o czymś
			- {um} - powiedz um, wydaj dźwięk u, jakbyś myślał o czymś
			- {yyy} - powiedz dosłownie yyy, wydaj dźwięk y, długi
		Nigdy nie czytaj treści w tagach {}!
		Zmieniaj głos zawsze wtedy gdy pojawi się nowy tag.

		</rule>`,
		},
	})
	if err != nil {
		logger.Error("Failed to update session", err)
		return fmt.Errorf("session update failed: %w", err)
	}

	// Create conversation item with text
	err = conn.SendMessage(ctx, &openairt.ConversationItemCreateEvent{
		Item: openairt.MessageItem{
			Type: openairt.MessageItemTypeMessage,
			Role: openairt.MessageRoleUser,
			Content: []openairt.MessageContentPart{
				{
					Type: openairt.MessageContentTypeInputText,
					Text: text,
				},
			},
		},
	})
	if err != nil {
		logger.Error("Failed to create conversation item", err)
		return fmt.Errorf("conversation item creation failed: %w", err)
	}

	// Request response with audio and text (API requires both)
	err = conn.SendMessage(ctx, &openairt.ResponseCreateEvent{
		Response: openairt.ResponseCreateParams{
			Modalities:        []openairt.Modality{openairt.ModalityAudio, openairt.ModalityText},
			Voice:             openairt.Voice(voice),
			OutputAudioFormat: openairt.AudioFormatPcm16,
		},
	})
	if err != nil {
		logger.Error("Failed to create response", err)
		return fmt.Errorf("response creation failed: %w", err)
	}

	// Start streaming audio player
	return p.streamAudioRealtime(ctx, conn)
}

// streamAudioRealtime handles real-time audio streaming and playback
func (p *RealtimeTTSProvider) streamAudioRealtime(ctx context.Context, conn *openairt.Conn) error {
	logger.Infof("🔊 Started real-time audio streaming...")

	// Create channel for streaming audio chunks
	audioChannel := make(chan []byte, 100) // Buffer for smooth streaming
	done := make(chan struct{})
	playbackError := make(chan error, 1)

	// Start streaming audio player in background
	go func() {
		defer close(done)

		// Start audio player command that accepts raw PCM16 via stdin
		playerCmd := exec.Command("aplay", "-r", "24000", "-f", "S16_LE", "-c", "1", "-")
		stdin, err := playerCmd.StdinPipe()
		if err != nil {
			playbackError <- fmt.Errorf("failed to create stdin pipe: %w", err)
			return
		}

		if err := playerCmd.Start(); err != nil {
			playbackError <- fmt.Errorf("failed to start player: %w", err)
			return
		}

		logger.Infof("🎵 Started real-time audio player")

		// Stream chunks to player as they arrive
		for chunk := range audioChannel {
			if _, err := stdin.Write(chunk); err != nil {
				logger.Error("Failed to write chunk to player", err)
				break
			}
		}

		stdin.Close()
		playerCmd.Wait()
		logger.Infof("✅ Audio player finished")
	}()

	// Set timeout for entire operation
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	audioReceived := false

	for {
		select {
		case <-timeout.C:
			close(audioChannel)
			if !audioReceived {
				return fmt.Errorf("timeout - no audio data received from API")
			}
			return fmt.Errorf("timeout waiting for audio completion")
		case <-ctx.Done():
			close(audioChannel)
			return ctx.Err()
		case err := <-playbackError:
			close(audioChannel)
			return fmt.Errorf("playback error: %w", err)
		default:
			// Read message with shorter timeout
			msgCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			event, err := conn.ReadMessage(msgCtx)
			cancel()

			if err != nil {
				logger.Error("Failed to read message", err)
				close(audioChannel)
				return fmt.Errorf("message read failed: %w", err)
			}

			switch event.ServerEventType() {
			case openairt.ServerEventTypeResponseAudioDelta:
				deltaEvent := event.(openairt.ResponseAudioDeltaEvent)

				// Decode base64 audio data
				audioChunk, err := base64.StdEncoding.DecodeString(deltaEvent.Delta)
				if err != nil {
					logger.Error("Failed to decode audio delta", err)
					continue
				}

				if len(audioChunk) > 0 {
					audioReceived = true
					logger.Debugf("🎵 Streaming %d bytes to player", len(audioChunk))

					// Send chunk to player immediately
					select {
					case audioChannel <- audioChunk:
						// Chunk sent successfully
					default:
						logger.Warn("Audio channel full, dropping chunk")
					}
				}

			case openairt.ServerEventTypeResponseDone:
				logger.Infof("✅ Real-time audio streaming completed")
				close(audioChannel) // Signal end of stream
				<-done              // Wait for player to finish
				return nil

			case openairt.ServerEventTypeError:
				errorEvent := event.(openairt.ErrorEvent)
				logger.Error("Realtime API error", fmt.Errorf("%s: %s", errorEvent.Error.Type, errorEvent.Error.Message))
				close(audioChannel)
				return fmt.Errorf("realtime API error: %s", errorEvent.Error.Message)

			default:
				// Only log important events
				if event.ServerEventType() != "response.audio_transcript.delta" {
					logger.Debugf("📨 Received event: %s", event.ServerEventType())
				}
			}
		}
	}
}

// startStreamingPlayer starts an audio player that accepts streaming WAV data
func (p *RealtimeTTSProvider) startStreamingPlayer() (*streamingPlayer, error) {
	players := []string{"paplay", "aplay", "play", "ffplay"}

	for _, playerName := range players {
		if _, err := exec.LookPath(playerName); err == nil {
			var cmd *exec.Cmd

			switch playerName {
			case "paplay":
				// paplay can handle streaming WAV from stdin
				cmd = exec.Command("paplay", "--rate=24000", "--format=s16le", "--channels=1", "-")
			case "aplay":
				// aplay can handle streaming WAV from stdin
				cmd = exec.Command("aplay", "-r", "24000", "-f", "S16_LE", "-c", "1", "-")
			case "play":
				// SoX play can handle streaming from stdin
				cmd = exec.Command("play", "-t", "wav", "-")
			case "ffplay":
				// ffplay can handle streaming WAV from stdin
				cmd = exec.Command("ffplay", "-nodisp", "-autoexit", "-f", "wav", "-")
			}

			// Setup stdin pipe for streaming
			stdin, err := cmd.StdinPipe()
			if err != nil {
				logger.Warnf("Failed to create stdin pipe for %s: %v", playerName, err)
				continue
			}

			// Start the player
			if err := cmd.Start(); err != nil {
				logger.Warnf("Failed to start %s: %v", playerName, err)
				stdin.Close()
				continue
			}

			logger.Infof("🎶 Started streaming audio with %s", playerName)
			return &streamingPlayer{cmd: cmd, stdin: stdin}, nil
		}
	}

	return nil, fmt.Errorf("no streaming audio player found (tried: %v)", players)
}

// createWAVHeader creates a WAV header for streaming (size can be unknown)
func (p *RealtimeTTSProvider) createWAVHeader(dataSize uint32) []byte {
	sampleRate := 24000 // OpenAI Realtime API uses 24kHz
	numChannels := 1    // Mono
	bitsPerSample := 16

	// WAV header (44 bytes)
	header := make([]byte, 44)

	// RIFF header
	copy(header[0:4], "RIFF")
	if dataSize == 0 {
		putUint32(header[4:8], 0xFFFFFFFF) // Unknown size for streaming
	} else {
		putUint32(header[4:8], uint32(36+dataSize))
	}
	copy(header[8:12], "WAVE")

	// Format chunk
	copy(header[12:16], "fmt ")
	putUint32(header[16:20], 16) // PCM format chunk size
	putUint16(header[20:22], 1)  // PCM format
	putUint16(header[22:24], uint16(numChannels))
	putUint32(header[24:28], uint32(sampleRate))
	putUint32(header[28:32], uint32(sampleRate*numChannels*bitsPerSample/8)) // Byte rate
	putUint16(header[32:34], uint16(numChannels*bitsPerSample/8))            // Block align
	putUint16(header[34:36], uint16(bitsPerSample))

	// Data chunk
	copy(header[36:40], "data")
	if dataSize == 0 {
		putUint32(header[40:44], 0xFFFFFFFF) // Unknown size for streaming
	} else {
		putUint32(header[40:44], dataSize)
	}

	return header
}

// GetAudio converts text to speech and returns raw audio data
func (p *RealtimeTTSProvider) GetAudio(ctx context.Context, text string, voice string) ([]byte, error) {
	// Use provided voice or fallback to config
	if voice == "" {
		voice = p.config.Voice
	}

	logger.Infof("Generating Realtime TTS for text (length: %d chars) with voice: %s", len(text), voice)

	// Create Realtime API client
	client := openairt.NewClient(p.apiKey)

	// Connect with context
	conn, err := client.Connect(ctx, openairt.WithModel(p.config.Model))
	if err != nil {
		logger.Error("Failed to connect to Realtime API", err)
		return nil, fmt.Errorf("realtime API connection failed: %w", err)
	}
	defer conn.Close()

	// Update session to support text input and audio output
	err = conn.SendMessage(ctx, &openairt.SessionUpdateEvent{
		Session: openairt.ClientSession{
			Modalities:        []openairt.Modality{openairt.ModalityText, openairt.ModalityAudio},
			Voice:             openairt.Voice(voice),
			OutputAudioFormat: openairt.AudioFormatPcm16,
			Instructions:      "You are a text-to-speech system. Speak the provided text naturally and clearly in Polish. Do not add any additional commentary or explanation.",
		},
	})
	if err != nil {
		logger.Error("Failed to update session", err)
		return nil, fmt.Errorf("session update failed: %w", err)
	}

	// Create conversation item with text
	err = conn.SendMessage(ctx, &openairt.ConversationItemCreateEvent{
		Item: openairt.MessageItem{
			Type: openairt.MessageItemTypeMessage,
			Role: openairt.MessageRoleUser,
			Content: []openairt.MessageContentPart{
				{
					Type: openairt.MessageContentTypeInputText,
					Text: text,
				},
			},
		},
	})
	if err != nil {
		logger.Error("Failed to create conversation item", err)
		return nil, fmt.Errorf("conversation item creation failed: %w", err)
	}

	// Request response with audio and text (API requires both)
	err = conn.SendMessage(ctx, &openairt.ResponseCreateEvent{
		Response: openairt.ResponseCreateParams{
			Modalities:        []openairt.Modality{openairt.ModalityAudio, openairt.ModalityText},
			Voice:             openairt.Voice(voice),
			OutputAudioFormat: openairt.AudioFormatPcm16,
		},
	})
	if err != nil {
		logger.Error("Failed to create response", err)
		return nil, fmt.Errorf("response creation failed: %w", err)
	}

	// Collect audio data
	var audioData []byte

	// Set timeout for audio generation
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-timeout.C:
			return nil, fmt.Errorf("timeout waiting for audio response")
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// Read message with shorter timeout
			msgCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			event, err := conn.ReadMessage(msgCtx)
			cancel()

			if err != nil {
				logger.Error("Failed to read message", err)
				return nil, fmt.Errorf("message read failed: %w", err)
			}

			switch event.ServerEventType() {
			case openairt.ServerEventTypeResponseAudioDelta:
				deltaEvent := event.(openairt.ResponseAudioDeltaEvent)
				logger.Debugf("Received audio delta: %d bytes", len(deltaEvent.Delta))

				// Decode base64 audio data
				audioChunk, err := base64.StdEncoding.DecodeString(deltaEvent.Delta)
				if err != nil {
					logger.Error("Failed to decode audio delta", err)
					continue
				}
				audioData = append(audioData, audioChunk...)

			case openairt.ServerEventTypeResponseDone:
				logger.Infof("Audio generation completed, total size: %d bytes", len(audioData))
				if len(audioData) == 0 {
					return nil, fmt.Errorf("no audio data received")
				}
				return p.convertPCM16ToWAV(audioData), nil

			case openairt.ServerEventTypeError:
				errorEvent := event.(openairt.ErrorEvent)
				logger.Error("Realtime API error", fmt.Errorf("%s: %s", errorEvent.Error.Type, errorEvent.Error.Message))
				return nil, fmt.Errorf("realtime API error: %s", errorEvent.Error.Message)

			default:
				// Ignore other events
				logger.Debugf("Received event: %s", event.ServerEventType())
			}
		}
	}
}

// convertPCM16ToWAV converts raw PCM16 audio data to WAV format
func (p *RealtimeTTSProvider) convertPCM16ToWAV(pcmData []byte) []byte {
	sampleRate := 24000 // OpenAI Realtime API uses 24kHz
	numChannels := 1    // Mono
	bitsPerSample := 16

	// WAV header
	header := make([]byte, 44)

	// RIFF header
	copy(header[0:4], "RIFF")
	putUint32(header[4:8], uint32(36+len(pcmData)))
	copy(header[8:12], "WAVE")

	// Format chunk
	copy(header[12:16], "fmt ")
	putUint32(header[16:20], 16) // PCM format chunk size
	putUint16(header[20:22], 1)  // PCM format
	putUint16(header[22:24], uint16(numChannels))
	putUint32(header[24:28], uint32(sampleRate))
	putUint32(header[28:32], uint32(sampleRate*numChannels*bitsPerSample/8)) // Byte rate
	putUint16(header[32:34], uint16(numChannels*bitsPerSample/8))            // Block align
	putUint16(header[34:36], uint16(bitsPerSample))

	// Data chunk
	copy(header[36:40], "data")
	putUint32(header[40:44], uint32(len(pcmData)))

	// Combine header and data
	wav := make([]byte, len(header)+len(pcmData))
	copy(wav, header)
	copy(wav[len(header):], pcmData)

	return wav
}

// Helper functions for WAV header
func putUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func putUint16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

// playAudio plays the audio data using the system's audio player
func (p *RealtimeTTSProvider) playAudio(audioData []byte) error {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "voicify_tts_*.wav")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write audio data to file
	if _, err := tmpFile.Write(audioData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write audio data: %w", err)
	}
	tmpFile.Close()

	// Play audio using system player
	return p.playAudioFile(tmpFile.Name())
}

// playAudioFile plays an audio file using available system players
func (p *RealtimeTTSProvider) playAudioFile(filename string) error {
	players := []string{"paplay", "aplay", "play", "ffplay"}

	for _, player := range players {
		if _, err := exec.LookPath(player); err == nil {
			var cmd *exec.Cmd

			switch player {
			case "paplay":
				cmd = exec.Command("paplay", filename)
			case "aplay":
				cmd = exec.Command("aplay", filename)
			case "play":
				cmd = exec.Command("play", filename)
			case "ffplay":
				cmd = exec.Command("ffplay", "-nodisp", "-autoexit", filename)
			}

			logger.Debugf("Playing audio with %s", player)
			if err := cmd.Run(); err != nil {
				logger.Warnf("Failed to play audio with %s: %v", player, err)
				continue
			}
			return nil
		}
	}

	return fmt.Errorf("no audio player found (tried: %v)", players)
}

// GetAvailableVoices returns list of available voices for Realtime API
func (p *RealtimeTTSProvider) GetAvailableVoices() []string {
	return []string{
		"alloy",   // Neutral, balanced
		"echo",    // Male, clear
		"fable",   // British accent
		"onyx",    // Deep male
		"nova",    // Young female (recommended for Polish)
		"shimmer", // Warm female
	}
}

// GetProviderName returns the name of the provider
func (p *RealtimeTTSProvider) GetProviderName() string {
	return "OpenAI Realtime API"
}
