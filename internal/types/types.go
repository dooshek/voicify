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

type Config struct {
	RecordKey KeyBinding    `yaml:"record_key"`
	LLM       LLMConfig     `yaml:"llm"`
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
