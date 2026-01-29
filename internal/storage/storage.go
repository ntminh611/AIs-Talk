package storage

import (
	"encoding/json"
	"os"
	"sync"
)

// ProviderConfig holds configuration for a single provider
type ProviderConfig struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

// RunConfig holds runtime configuration for debates
type RunConfig struct {
	Temperature    float64 `json:"temperature"`
	Rounds         int     `json:"rounds"`
	StopAfterTurns int     `json:"stop_after_turns"`
}

// Config holds all configuration
type Config struct {
	Provider   string         `json:"provider"`
	OpenAI     ProviderConfig `json:"openai"`
	Anthropic  ProviderConfig `json:"anthropic"`
	Gemini     ProviderConfig `json:"gemini"`
	Ollama     ProviderConfig `json:"ollama"`
	Grok       ProviderConfig `json:"grok"`
	DeepSeek   ProviderConfig `json:"deepseek"`
	Perplexity ProviderConfig `json:"perplexity"`
	Run        RunConfig      `json:"run"`
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		Provider: "openai",
		OpenAI: ProviderConfig{
			APIKey:  "",
			BaseURL: "https://api.openai.com",
			Model:   "gpt-4o-mini",
		},
		Anthropic: ProviderConfig{
			APIKey:  "",
			BaseURL: "https://api.anthropic.com",
			Model:   "claude-3-5-sonnet-latest",
		},
		Gemini: ProviderConfig{
			APIKey:  "",
			BaseURL: "https://generativelanguage.googleapis.com",
			Model:   "gemini-1.5-flash-latest",
		},
		Ollama: ProviderConfig{
			APIKey:  "",
			BaseURL: "http://localhost:11434",
			Model:   "llama2",
		},
		Grok: ProviderConfig{
			APIKey:  "",
			BaseURL: "https://api.x.ai/v1",
			Model:   "grok-beta",
		},
		DeepSeek: ProviderConfig{
			APIKey:  "",
			BaseURL: "https://api.deepseek.com/v1",
			Model:   "deepseek-chat",
		},
		Perplexity: ProviderConfig{
			APIKey:  "",
			BaseURL: "https://api.perplexity.ai",
			Model:   "llama-3.1-sonar-small-128k-online",
		},
		Run: RunConfig{
			Temperature:    0.7,
			Rounds:         3,
			StopAfterTurns: 0,
		},
	}
}

// Storage handles persistent storage of configuration
type Storage struct {
	filePath string
	config   Config
	mu       sync.RWMutex
}

// NewStorage creates a new storage instance
func NewStorage(filePath string) *Storage {
	s := &Storage{
		filePath: filePath,
		config:   DefaultConfig(),
	}
	s.Load() // Try to load existing config
	return s
}

// Load reads configuration from file
func (s *Storage) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, use defaults
		}
		return err
	}

	// Start with defaults, then overlay with saved config
	config := DefaultConfig()
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	s.config = config
	return nil
}

// Save writes configuration to file
func (s *Storage) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0600) // 0600 = owner read/write only
}

// GetConfig returns a copy of the configuration
func (s *Storage) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// SetConfig updates all configuration
func (s *Storage) SetConfig(config Config) error {
	s.mu.Lock()
	s.config = config
	s.mu.Unlock()
	return s.Save()
}

// GetDefaultProvider returns the default provider name
func (s *Storage) GetDefaultProvider() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Provider
}

// SetDefaultProvider sets the default provider
func (s *Storage) SetDefaultProvider(provider string) error {
	s.mu.Lock()
	s.config.Provider = provider
	s.mu.Unlock()
	return s.Save()
}

// GetProviderConfig returns configuration for a specific provider
func (s *Storage) GetProviderConfig(provider string) ProviderConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch provider {
	case "openai":
		return s.config.OpenAI
	case "anthropic", "claude":
		return s.config.Anthropic
	case "gemini", "google":
		return s.config.Gemini
	case "ollama":
		return s.config.Ollama
	case "grok":
		return s.config.Grok
	case "deepseek":
		return s.config.DeepSeek
	case "perplexity":
		return s.config.Perplexity
	default:
		return s.config.OpenAI
	}
}

// SetProviderConfig updates configuration for a specific provider
func (s *Storage) SetProviderConfig(provider string, cfg ProviderConfig) error {
	s.mu.Lock()
	switch provider {
	case "openai":
		s.config.OpenAI = cfg
	case "anthropic", "claude":
		s.config.Anthropic = cfg
	case "gemini", "google":
		s.config.Gemini = cfg
	case "ollama":
		s.config.Ollama = cfg
	case "grok":
		s.config.Grok = cfg
	case "deepseek":
		s.config.DeepSeek = cfg
	case "perplexity":
		s.config.Perplexity = cfg
	}
	s.mu.Unlock()
	return s.Save()
}

// GetRunConfig returns runtime configuration
func (s *Storage) GetRunConfig() RunConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Run
}

// SetRunConfig updates runtime configuration
func (s *Storage) SetRunConfig(cfg RunConfig) error {
	s.mu.Lock()
	s.config.Run = cfg
	s.mu.Unlock()
	return s.Save()
}

// Backward compatible helper methods

// GetOpenAI returns the OpenAI API key
func (s *Storage) GetOpenAI() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config.OpenAI.APIKey != "" {
		return s.config.OpenAI.APIKey
	}
	return os.Getenv("OPENAI_API_KEY")
}

// GetAnthropic returns the Anthropic API key
func (s *Storage) GetAnthropic() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config.Anthropic.APIKey != "" {
		return s.config.Anthropic.APIKey
	}
	return os.Getenv("ANTHROPIC_API_KEY")
}

// GetGemini returns the Gemini API key
func (s *Storage) GetGemini() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config.Gemini.APIKey != "" {
		return s.config.Gemini.APIKey
	}
	return os.Getenv("GEMINI_API_KEY")
}

// GetOllamaURL returns the Ollama base URL
func (s *Storage) GetOllamaURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config.Ollama.BaseURL != "" {
		return s.config.Ollama.BaseURL
	}
	return "http://localhost:11434"
}

// GetGrok returns the Grok API key
func (s *Storage) GetGrok() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config.Grok.APIKey != "" {
		return s.config.Grok.APIKey
	}
	return os.Getenv("GROK_API_KEY")
}

// GetDeepSeek returns the DeepSeek API key
func (s *Storage) GetDeepSeek() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config.DeepSeek.APIKey != "" {
		return s.config.DeepSeek.APIKey
	}
	return os.Getenv("DEEPSEEK_API_KEY")
}

// GetPerplexity returns the Perplexity API key
func (s *Storage) GetPerplexity() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config.Perplexity.APIKey != "" {
		return s.config.Perplexity.APIKey
	}
	return os.Getenv("PERPLEXITY_API_KEY")
}

// HasAnyKey returns true if at least one API key is configured
func (s *Storage) HasAnyKey() bool {
	return s.GetOpenAI() != "" || s.GetAnthropic() != "" || s.GetGemini() != "" ||
		s.GetGrok() != "" || s.GetDeepSeek() != "" || s.GetPerplexity() != ""
}

// GetKeyStatus returns which providers have keys configured
func (s *Storage) GetKeyStatus() map[string]bool {
	return map[string]bool{
		"openai":     s.GetOpenAI() != "",
		"anthropic":  s.GetAnthropic() != "",
		"gemini":     s.GetGemini() != "",
		"ollama":     true, // Ollama doesn't need a key
		"grok":       s.GetGrok() != "",
		"deepseek":   s.GetDeepSeek() != "",
		"perplexity": s.GetPerplexity() != "",
	}
}

// Legacy APIKeys struct for backward compatibility
type APIKeys struct {
	OpenAI    string `json:"openai"`
	Anthropic string `json:"anthropic"`
	Gemini    string `json:"gemini"`
	OllamaURL string `json:"ollama_url"`
}

// GetKeys returns API keys in legacy format
func (s *Storage) GetKeys() APIKeys {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return APIKeys{
		OpenAI:    s.config.OpenAI.APIKey,
		Anthropic: s.config.Anthropic.APIKey,
		Gemini:    s.config.Gemini.APIKey,
		OllamaURL: s.config.Ollama.BaseURL,
	}
}

// SetKeys updates API keys from legacy format
func (s *Storage) SetKeys(keys APIKeys) error {
	s.mu.Lock()
	s.config.OpenAI.APIKey = keys.OpenAI
	s.config.Anthropic.APIKey = keys.Anthropic
	s.config.Gemini.APIKey = keys.Gemini
	if keys.OllamaURL != "" {
		s.config.Ollama.BaseURL = keys.OllamaURL
	}
	s.mu.Unlock()
	return s.Save()
}
