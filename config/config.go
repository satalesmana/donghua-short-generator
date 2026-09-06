package config

import (
	"os"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// LLM / Transcription API Keys
	NinerouterAPIKey  string
	NinerouterBaseURL string
	NinerouterModel   string
	OpenAIAPIKey      string
	OpenAIBaseURL     string

	// Workspace paths
	WorkspaceDir string
	InputDir     string
	TempDir      string
	OutputDir    string

	// TTS settings (Fish Audio via OpenRouter)
	TTSVoice   string
	TTSLocale  string
	TTSModel   string
	TTSAPIKey  string
	TTSBaseURL string

	// Video rendering settings
	TargetWidth  int
	TargetHeight int
	FrameRate    int
	VideoCodec   string
	AudioCodec   string

	// Transcription settings
	TranscriptionModel string
	TranscriptionLang  string

	// Local Whisper.cpp
	WhisperBinary    string
	WhisperModelPath string

	// Timeout / retry
	HTTPTimeout time.Duration
	MaxRetries  int

	// LLM analysis
	LLMTemperature float64
	LLMMaxTokens   int
}

// LoadConfig reads environment variables and fills defaults.
func LoadConfig() *Config {
	return &Config{
		// API Keys & URLs (LOKAL)
		NinerouterAPIKey:  getEnvOrDefault("NINEROUTER_API_KEY", ""),
		NinerouterBaseURL: getEnvOrDefault("NINEROUTER_BASE_URL", "https://api.9router.com/v1"),
		NinerouterModel:   getEnvOrDefault("NINEROUTER_MODEL", "FreeModel"),

		OpenAIAPIKey:  getEnvOrDefault("OPENAI_API_KEY", ""),
		OpenAIBaseURL: getEnvOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),

		// Directories
		WorkspaceDir: getEnvOrDefault("WORKSPACE_DIR", "./workspace"),
		InputDir:     getEnvOrDefault("INPUT_DIR", "./workspace/input"),
		TempDir:      getEnvOrDefault("TEMP_DIR", "./workspace/temp"),
		OutputDir:    getEnvOrDefault("OUTPUT_DIR", "./workspace/output"),

		// TTS (Fish Audio via OpenRouter)
		TTSVoice:   getEnvOrDefault("TTS_VOICE", "74cb8f9136944fc7b132455773cb5a27"),
		TTSLocale:  getEnvOrDefault("TTS_LOCALE", "id-ID"),
		TTSModel:   getEnvOrDefault("TTS_MODEL", "fish-audio/s2.1-pro-free:free"),
		TTSAPIKey:  getEnvOrDefault("OPENAI_API_KEY", ""),
		TTSBaseURL: getEnvOrDefault("OPENAI_BASE_URL", "https://openrouter.ai/api/v1"),

		// Video (9:16 Shorts)
		TargetWidth:  1080,
		TargetHeight: 1920,
		FrameRate:    30,
		VideoCodec:   "libx264",
		AudioCodec:   "aac",

		// Transcription
		TranscriptionModel: getEnvOrDefault("WHISPER_MODEL", "whisper-1"),
		TranscriptionLang:  getEnvOrDefault("WHISPER_LANG", "zh"),

		// Local Whisper.cpp
		WhisperBinary:    getEnvOrDefault("WHISPER_BINARY", "whisper-cli"),
		WhisperModelPath: getEnvOrDefault("WHISPER_MODEL_PATH", ""),

		// Timeouts
		HTTPTimeout: 300 * time.Second,
		MaxRetries:  3,

		// LLM analysis
		LLMTemperature: 0.7,
		LLMMaxTokens:   8192,
	}
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
