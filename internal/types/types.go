package types

import "github.com/sashabaranov/go-openai"

// KeyCombo interface for types that can be printed as a key combination
type KeyCombo interface {
	HasCtrl() bool
	HasShift() bool
	HasAlt() bool
	HasSuper() bool
	GetKey() string
}

type KeyBinding struct {
	Key   string `yaml:"key"`   // The actual key (e.g., "a", "b", "1", etc.)
	Ctrl  bool   `yaml:"ctrl"`  // Control key modifier
	Shift bool   `yaml:"shift"` // Shift key modifier
	Alt   bool   `yaml:"alt"`   // Alt key modifier
	Super bool   `yaml:"super"` // Super (Windows/Command) key modifier
}

// Implement KeyCombo for KeyBinding
func (kb KeyBinding) HasCtrl() bool  { return kb.Ctrl }
func (kb KeyBinding) HasShift() bool { return kb.Shift }
func (kb KeyBinding) HasAlt() bool   { return kb.Alt }
func (kb KeyBinding) HasSuper() bool { return kb.Super }
func (kb KeyBinding) GetKey() string { return kb.Key }

type YdotoolConfig struct {
	SocketPath string
}

// NOTE: Plugin-related types are now in plugin.go

type LLMProvider string

const (
	ProviderOpenAI LLMProvider = "openai"
	ProviderGroq   LLMProvider = "groq"
)

type LLMConfig struct {
	Keys          LLMKeys          `yaml:"keys"`
	Transcription LLMTranscription `yaml:"transcription"`
	Router        LLMRouter        `yaml:"router"`
}

type LLMKeys struct {
	OpenAIKey string `yaml:"openai_api_key"`
	GroqKey   string `yaml:"groq_api_key"`
}

type LLMTranscription struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	Language string `yaml:"language"`
}

type LLMRouter struct {
	Provider    string  `yaml:"provider"`
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
}

// TTSConfig holds configuration for Text-to-Speech
type TTSConfig struct {
	Provider       string            `yaml:"provider"`        // "openai", "realtime", "elevenlabs"
	Voice          string            `yaml:"voice"`           // default voice to use
	SystemPrompt   string            `yaml:"system_prompt"`   // Template for TTS instructions (use %s for text)
	Speed          float64           `yaml:"speed"`           // Speaking speed (0.25-4.0)
	OpenAI         TTSOpenAIConfig   `yaml:"openai"`
	Realtime       TTSRealtimeConfig `yaml:"realtime"`
}

// TTSOpenAIConfig holds OpenAI TTS specific configuration
type TTSOpenAIConfig struct {
	Model  string  `yaml:"model"`  // "tts-1" or "tts-1-hd"
	Speed  float64 `yaml:"speed"`  // 0.25-4.0, default 1.0
	Format string  `yaml:"format"` // "opus", "mp3", "aac", "flac"
}

// TTSRealtimeConfig holds OpenAI Realtime API TTS specific configuration
type TTSRealtimeConfig struct {
	Model string  `yaml:"model"` // "gpt-4o-realtime-preview" or "gpt-4o-mini-realtime-preview"
	Speed float64 `yaml:"speed"` // Not directly supported, default 1.0
}


type Config struct {
	RecordKey KeyBinding    `yaml:"record_key"`
	LLM       LLMConfig     `yaml:"llm"`
	TTS       TTSConfig     `yaml:"tts"`
	Ydotool   YdotoolConfig `yaml:"ydotool"`
}

func (c *Config) GetYdotoolConfig() YdotoolConfig {
	config := YdotoolConfig{
		SocketPath: c.Ydotool.SocketPath,
	}
	if config.SocketPath == "" {
		config.SocketPath = "/var/run/ydotool.sock"
	}
	return config
}

func (c *Config) GetLLMConfig() LLMConfig {
	return c.LLM
}

// GetTTSConfig returns TTS configuration with defaults
func (c *Config) GetTTSConfig() TTSConfig {
	config := c.TTS

	// Set provider default
	if config.Provider == "" {
		config.Provider = "realtime" // Default to Realtime API for better quality
	}

	// Set voice default
	if config.Voice == "" {
		config.Voice = "nova" // Good for Polish
	}

	// OpenAI TTS defaults
	if config.OpenAI.Model == "" {
		config.OpenAI.Model = "tts-1-hd"
	}
	if config.OpenAI.Speed == 0 {
		config.OpenAI.Speed = 1.0
	}
	if config.OpenAI.Format == "" {
		config.OpenAI.Format = "opus"
	}

	// Realtime API defaults
	if config.Realtime.Model == "" {
		config.Realtime.Model = "gpt-4o-realtime-preview"
	}
	if config.Realtime.Speed == 0 {
		config.Realtime.Speed = 1.0
	}

	return config
}

const (
	OpenAIModelGPT4oMini string = string(openai.GPT4oMini)
	OpenAIModelGPT4o     string = string(openai.GPT4o)
)

const (
	OpenAIModelWhisper1 string = string(openai.Whisper1)
)

// Groq LLM Models
const (
	GroqModelLLama3_8B_8192    string = "llama3-8b-8192"
	GroqModelLLama3_70B_8192   string = "llama3-70b-8192"
	GroqModelLLama3_3_70B      string = "llama3-70b-versatile"
	GroqModelLLama3_1_8B       string = "llama3-1-8b-instant"
	GroqModelMixtral8x7B_32768 string = "mixtral-8x7b-32768"
	GroqModelGemma2_9B         string = "gemma2-9b-it"
	GroqModelLLamaGuard3_8B    string = "llama-guard-3-8b"
)

// Groq Whisper Models
const (
	GroqModelWhisperLargeV3          string = "whisper-large-v3"
	GroqModelWhisperLargeV3Turbo     string = "whisper-large-v3-turbo"
	GroqModelDistilWhisperLargeV3_EN string = "distil-whisper-large-v3-en"
)

// Groq Preview Models - should be used only for evaluation
const (
	GroqModelLLama3_3_70B_SpecDec string = "llama-3.3-70b-specdec"
	GroqModelLLama3_2_1B_Preview  string = "llama-3.2-1b-preview"
	GroqModelLLama3_2_3B_Preview  string = "llama-3.2-3b-preview"
	GroqModelLLama3_2_11B_Vision  string = "llama-3.2-11b-vision-preview"
	GroqModelLLama3_2_90B_Vision  string = "llama-3.2-90b-vision-preview"
)
