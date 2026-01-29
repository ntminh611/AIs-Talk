package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/user/talk/internal/provider"
	"github.com/user/talk/internal/storage"
)

// ProxyConfig holds proxy configuration
type ProxyConfig struct {
	DefaultProvider string `json:"default_provider" yaml:"default_provider"`
	EnableLogging   bool   `json:"enable_logging" yaml:"enable_logging"`
}

// Proxy handles API gateway functionality
type Proxy struct {
	storage       *storage.Storage
	enableLogging bool
}

// NewProxy creates a new proxy instance
func NewProxy(store *storage.Storage) *Proxy {
	return &Proxy{
		storage:       store,
		enableLogging: true,
	}
}

// SetConfig updates proxy configuration
func (p *Proxy) SetConfig(cfg ProxyConfig) {
	if cfg.DefaultProvider != "" {
		p.storage.SetDefaultProvider(cfg.DefaultProvider)
	}
	p.enableLogging = cfg.EnableLogging
}

// GetConfig returns current proxy configuration
func (p *Proxy) GetConfig() ProxyConfig {
	return ProxyConfig{
		DefaultProvider: p.storage.GetDefaultProvider(),
		EnableLogging:   p.enableLogging,
	}
}

// OpenAI-compatible request/response types

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Provider    string        `json:"provider,omitempty"` // Custom field to specify provider
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage,omitempty"`
}

type Choice struct {
	Index        int          `json:"index"`
	Message      *ChatMessage `json:"message,omitempty"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason string       `json:"finish_reason,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

// HandleChatCompletions handles /v1/chat/completions endpoint
func (p *Proxy) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.respondError(w, http.StatusBadRequest, "invalid_request_error", "Invalid request body")
		return
	}

	if p.enableLogging {
		log.Printf("[Proxy] Request: model=%s, provider=%s, stream=%v, messages=%d",
			req.Model, req.Provider, req.Stream, len(req.Messages))
	}

	// Determine which provider to use
	providerType := req.Provider
	if providerType == "" {
		providerType = p.storage.GetDefaultProvider()
	}

	// Get API key for the provider
	apiKey := p.getAPIKey(providerType)
	if apiKey == "" && providerType != "ollama" {
		p.respondError(w, http.StatusUnauthorized, "authentication_error",
			fmt.Sprintf("No API key configured for provider: %s", providerType))
		return
	}

	// Create provider
	prov, err := p.createProvider(providerType, apiKey, req.Model)
	if err != nil {
		p.respondError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	// Convert messages
	messages := make([]provider.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = provider.Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	opts := provider.Options{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	if req.Stream {
		p.handleStreamingResponse(w, prov, messages, opts, req.Model)
	} else {
		p.handleNonStreamingResponse(w, prov, messages, opts, req.Model)
	}
}

func (p *Proxy) handleStreamingResponse(w http.ResponseWriter, prov provider.Provider, messages []provider.Message, opts provider.Options, model string) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		p.respondError(w, http.StatusInternalServerError, "server_error", "Streaming not supported")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	streamCh, err := prov.Chat(ctx, messages, opts)
	if err != nil {
		p.respondError(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}

	responseID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	created := time.Now().Unix()

	for chunk := range streamCh {
		if chunk.Error != nil {
			// Send error as SSE
			errData := map[string]interface{}{
				"error": map[string]string{
					"message": chunk.Error.Error(),
					"type":    "api_error",
				},
			}
			data, _ := json.Marshal(errData)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			return
		}

		if chunk.Done {
			// Send final chunk with finish_reason
			finalChunk := StreamChunk{
				ID:      responseID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []Choice{
					{
						Index:        0,
						Delta:        &ChatMessage{},
						FinishReason: "stop",
					},
				},
			}
			data, _ := json.Marshal(finalChunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		if chunk.Content != "" {
			streamChunk := StreamChunk{
				ID:      responseID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []Choice{
					{
						Index: 0,
						Delta: &ChatMessage{
							Content: chunk.Content,
						},
					},
				},
			}
			data, _ := json.Marshal(streamChunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (p *Proxy) handleNonStreamingResponse(w http.ResponseWriter, prov provider.Provider, messages []provider.Message, opts provider.Options, model string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	streamCh, err := prov.Chat(ctx, messages, opts)
	if err != nil {
		p.respondError(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}

	// Collect full response
	var fullContent strings.Builder
	for chunk := range streamCh {
		if chunk.Error != nil {
			p.respondError(w, http.StatusInternalServerError, "api_error", chunk.Error.Error())
			return
		}
		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
		}
		if chunk.Done {
			break
		}
	}

	response := ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{
			{
				Index: 0,
				Message: &ChatMessage{
					Role:    "assistant",
					Content: fullContent.String(),
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     len(messages) * 10, // Estimate
			CompletionTokens: len(fullContent.String()) / 4,
			TotalTokens:      len(messages)*10 + len(fullContent.String())/4,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (p *Proxy) getAPIKey(providerType string) string {
	switch providerType {
	case "openai":
		return p.storage.GetOpenAI()
	case "anthropic", "claude":
		return p.storage.GetAnthropic()
	case "gemini", "google":
		return p.storage.GetGemini()
	case "ollama":
		return ""
	default:
		return p.storage.GetOpenAI() // Default to OpenAI
	}
}

func (p *Proxy) createProvider(providerType, apiKey, model string) (provider.Provider, error) {
	cfg := p.storage.GetProviderConfig(providerType)

	// Use model from config if not specified in request
	if model == "" {
		model = cfg.Model
	}

	// Use base URL from config
	baseURL := cfg.BaseURL

	switch providerType {
	case "openai":
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		return provider.NewOpenAI(apiKey, model, baseURL), nil
	case "anthropic", "claude":
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		return provider.NewAnthropic(apiKey, model, baseURL), nil
	case "gemini", "google":
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com"
		}
		return provider.NewGemini(apiKey, model, baseURL), nil
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return provider.NewOllama(baseURL, model), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerType)
	}
}

func (p *Proxy) respondError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"message": message,
			"type":    errType,
		},
	})
}

// HandleModels returns available models (OpenAI-compatible /v1/models endpoint)
func (p *Proxy) HandleModels(w http.ResponseWriter, r *http.Request) {
	models := []map[string]interface{}{
		{"id": "gpt-4", "object": "model", "owned_by": "openai", "provider": "openai"},
		{"id": "gpt-3.5-turbo", "object": "model", "owned_by": "openai", "provider": "openai"},
		{"id": "claude-3-5-sonnet-20241022", "object": "model", "owned_by": "anthropic", "provider": "anthropic"},
		{"id": "claude-3-opus-20240229", "object": "model", "owned_by": "anthropic", "provider": "anthropic"},
		{"id": "gemini-pro", "object": "model", "owned_by": "google", "provider": "gemini"},
		{"id": "gemini-1.5-pro", "object": "model", "owned_by": "google", "provider": "gemini"},
		{"id": "llama2", "object": "model", "owned_by": "ollama", "provider": "ollama"},
		{"id": "mistral", "object": "model", "owned_by": "ollama", "provider": "ollama"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data":   models,
	})
}

// HandleProxyConfig handles proxy configuration endpoint
func (p *Proxy) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p.GetConfig())
}

func (p *Proxy) HandleSetConfig(w http.ResponseWriter, r *http.Request) {
	var cfg ProxyConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		p.respondError(w, http.StatusBadRequest, "invalid_request_error", "Invalid config")
		return
	}
	p.SetConfig(cfg)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// RawProxy forwards requests directly to the specified provider without transformation
func (p *Proxy) HandleRawProxy(w http.ResponseWriter, r *http.Request) {
	providerType := r.URL.Query().Get("provider")
	if providerType == "" {
		providerType = p.storage.GetDefaultProvider()
	}

	apiKey := p.getAPIKey(providerType)
	targetURL := p.getProviderURL(providerType)

	if targetURL == "" {
		p.respondError(w, http.StatusBadRequest, "invalid_request_error", "Unknown provider")
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.respondError(w, http.StatusBadRequest, "invalid_request_error", "Failed to read body")
		return
	}

	// Create proxy request
	proxyReq, err := http.NewRequest(r.Method, targetURL, strings.NewReader(string(body)))
	if err != nil {
		p.respondError(w, http.StatusInternalServerError, "server_error", "Failed to create request")
		return
	}

	// Copy headers and set auth
	proxyReq.Header.Set("Content-Type", "application/json")
	p.setAuthHeader(proxyReq, providerType, apiKey)

	// Execute request
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(proxyReq)
	if err != nil {
		p.respondError(w, http.StatusBadGateway, "api_error", err.Error())
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)

	// Stream response
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		flusher, ok := w.(http.Flusher)
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				break
			}
			w.Write(line)
			if ok {
				flusher.Flush()
			}
		}
	} else {
		io.Copy(w, resp.Body)
	}
}

func (p *Proxy) getProviderURL(providerType string) string {
	cfg := p.storage.GetProviderConfig(providerType)
	baseURL := cfg.BaseURL

	switch providerType {
	case "openai":
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		return baseURL + "/v1/chat/completions"
	case "anthropic", "claude":
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		return baseURL + "/v1/messages"
	case "gemini", "google":
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com"
		}
		model := cfg.Model
		if model == "" {
			model = "gemini-pro"
		}
		return baseURL + "/v1beta/models/" + model + ":generateContent"
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return baseURL + "/api/chat"
	default:
		return ""
	}
}

func (p *Proxy) setAuthHeader(req *http.Request, providerType, apiKey string) {
	switch providerType {
	case "openai":
		req.Header.Set("Authorization", "Bearer "+apiKey)
	case "anthropic", "claude":
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "gemini", "google":
		// Gemini uses query param for API key
		q := req.URL.Query()
		q.Set("key", apiKey)
		req.URL.RawQuery = q.Encode()
	}
}
