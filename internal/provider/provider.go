package provider

import (
	"context"
	"fmt"
)

// Message represents a chat message
type Message struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// StreamChunk represents a chunk of streamed response
type StreamChunk struct {
	Content string
	Done    bool
	Error   error
}

// Options contains configuration for a chat request
type Options struct {
	Model            string
	MaxTokens        int
	Temperature      float64
	TopP             float64
	TopK             int
	FrequencyPenalty float64
	PresencePenalty  float64
}

// Provider interface for AI providers
type Provider interface {
	Name() string
	Chat(ctx context.Context, messages []Message, opts Options) (<-chan StreamChunk, error)
}

// Config holds provider configuration
type Config struct {
	Type             string  `yaml:"type"`
	APIKey           string  `yaml:"api_key"`
	Model            string  `yaml:"model"`
	BaseURL          string  `yaml:"base_url,omitempty"`
	MaxTokens        int     `yaml:"max_tokens,omitempty"`
	Temperature      float64 `yaml:"temperature,omitempty"`
	TopP             float64 `yaml:"top_p,omitempty"`
	TopK             int     `yaml:"top_k,omitempty"`
	FrequencyPenalty float64 `yaml:"frequency_penalty,omitempty"`
	PresencePenalty  float64 `yaml:"presence_penalty,omitempty"`
}

// Registry holds all available providers
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(name string, p Provider) {
	r.providers[name] = p
}

// Get returns a provider by name
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return p, nil
}

// CreateProvider creates a provider based on config
func CreateProvider(cfg Config) (Provider, error) {
	switch cfg.Type {
	case "openai":
		return NewOpenAI(cfg.APIKey, cfg.Model, cfg.BaseURL), nil
	case "anthropic":
		return NewAnthropic(cfg.APIKey, cfg.Model, cfg.BaseURL), nil
	case "gemini":
		return NewGemini(cfg.APIKey, cfg.Model, cfg.BaseURL), nil
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return NewOllama(baseURL, cfg.Model), nil
	case "grok":
		return NewGrok(cfg.APIKey, cfg.Model, cfg.BaseURL), nil
	case "deepseek":
		return NewDeepSeek(cfg.APIKey, cfg.Model, cfg.BaseURL), nil
	case "perplexity":
		return NewPerplexity(cfg.APIKey, cfg.Model, cfg.BaseURL), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
}
