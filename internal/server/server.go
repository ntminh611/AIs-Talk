package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/user/talk/internal/debate"
	"github.com/user/talk/internal/proxy"
	"github.com/user/talk/internal/storage"
)

// AgentConfigFuncs holds callbacks for agent config management
type AgentConfigFuncs struct {
	GetConfigs func() []AgentYAMLConfig
	Add        func(AgentYAMLConfig) error
	Update     func(string, AgentYAMLConfig) error
	Delete     func(string) error
	Reorder    func([]string) error
	Reload     func() error
}

// AgentYAMLConfig represents agent configuration from YAML
type AgentYAMLConfig struct {
	ID               string  `json:"id" yaml:"id"`
	Name             string  `json:"name" yaml:"name"`
	Role             string  `json:"role" yaml:"role"`
	SystemPrompt     string  `json:"system_prompt" yaml:"system_prompt"`
	Provider         string  `json:"provider" yaml:"provider"`
	Model            string  `json:"model" yaml:"model"`
	Color            string  `json:"color" yaml:"color"`
	APIKey           string  `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	BaseURL          string  `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	Temperature      float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	MaxTokens        int     `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	TopP             float64 `json:"top_p,omitempty" yaml:"top_p,omitempty"`
	TopK             int     `json:"top_k,omitempty" yaml:"top_k,omitempty"`
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty" yaml:"frequency_penalty,omitempty"`
	PresencePenalty  float64 `json:"presence_penalty,omitempty" yaml:"presence_penalty,omitempty"`
}

// Server represents the HTTP server
type Server struct {
	router  *chi.Mux
	manager *debate.Manager
	hub     *Hub
	storage *storage.Storage
	proxy   *proxy.Proxy
	// Callback to reload agents when API keys change
	reloadAgents func() error
	// Agent config management functions
	agentFuncs *AgentConfigFuncs
}

// NewServer creates a new server
func NewServer(manager *debate.Manager, staticFS embed.FS, store *storage.Storage, reloadFn func() error) *Server {
	s := &Server{
		router:       chi.NewRouter(),
		manager:      manager,
		hub:          NewHub(),
		storage:      store,
		proxy:        proxy.NewProxy(store),
		reloadAgents: reloadFn,
	}

	s.setupRoutes(staticFS)
	return s
}

// SetAgentFuncs sets the agent config management functions
func (s *Server) SetAgentFuncs(funcs *AgentConfigFuncs) {
	s.agentFuncs = funcs
}

func (s *Server) setupRoutes(staticFS embed.FS) {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(corsMiddleware)

	// API routes
	s.router.Route("/api", func(r chi.Router) {
		r.Get("/agents", s.handleGetAgents)
		r.Get("/agents/configs", s.handleGetAgentConfigs)
		r.Post("/agents", s.handleAddAgent)
		r.Put("/agents/{agentID}", s.handleUpdateAgent)
		r.Delete("/agents/{agentID}", s.handleDeleteAgent)
		r.Post("/agents/reorder", s.handleReorderAgents)
		r.Get("/debate/status", s.handleGetStatus)
		r.Post("/debate/start", s.handleStartDebate)
		r.Post("/debate/continue", s.handleContinueDebate)
		r.Post("/debate/stop", s.handleStopDebate)
		r.Post("/debate/next", s.handleNextTurn)
		r.Post("/debate/agent/{agentID}", s.handleAgentTurn)
		r.Get("/debate/messages", s.handleGetMessages)
		r.Post("/debate/mode", s.handleSetMode)
		r.Post("/debate/reset", s.handleReset)

		// API Keys management (legacy)
		r.Get("/settings/keys", s.handleGetAPIKeys)
		r.Post("/settings/keys", s.handleSaveAPIKeys)
		r.Get("/settings/keys/status", s.handleGetKeyStatus)

		// Full config management
		r.Get("/settings/config", s.handleGetConfig)
		r.Post("/settings/config", s.handleSaveConfig)
		r.Get("/settings/run", s.handleGetRunConfig)
		r.Post("/settings/run", s.handleSaveRunConfig)

		// Proxy settings
		r.Get("/settings/proxy", s.proxy.HandleGetConfig)
		r.Post("/settings/proxy", s.proxy.HandleSetConfig)
	})

	// OpenAI-compatible API Gateway (proxy)
	s.router.Route("/v1", func(r chi.Router) {
		r.Post("/chat/completions", s.proxy.HandleChatCompletions)
		r.Get("/models", s.proxy.HandleModels)
		r.Post("/proxy", s.proxy.HandleRawProxy) // Raw proxy to any provider
	})

	// WebSocket
	s.router.Get("/ws", s.handleWebSocket)

	// Static files
	subFS, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		log.Printf("Warning: could not load static files: %v", err)
		return
	}

	fileServer := http.FileServer(http.FS(subFS))
	s.router.Handle("/*", fileServer)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Run starts the server
func (s *Server) Run(addr string) error {
	// Start WebSocket hub
	go s.hub.Run()

	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

// API Handlers

func (s *Server) handleGetAgents(w http.ResponseWriter, r *http.Request) {
	agents := s.manager.GetAgents()
	respondJSON(w, http.StatusOK, agents)
}

func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"is_running": s.manager.IsRunning(),
		"topic":      s.manager.GetTopic(),
		"mode":       s.manager.GetMode(),
	}
	respondJSON(w, http.StatusOK, status)
}

type startRequest struct {
	Topic string `json:"topic"`
}

func (s *Server) handleStartDebate(w http.ResponseWriter, r *http.Request) {
	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Topic == "" {
		respondError(w, http.StatusBadRequest, "Topic is required")
		return
	}

	if err := s.manager.Start(req.Topic); err != nil {
		respondError(w, http.StatusConflict, err.Error())
		return
	}

	// Broadcast to all clients
	s.hub.Broadcast(map[string]interface{}{
		"type":  "debate_started",
		"topic": req.Topic,
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "started", "topic": req.Topic})
}

func (s *Server) handleContinueDebate(w http.ResponseWriter, r *http.Request) {
	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Topic == "" {
		respondError(w, http.StatusBadRequest, "Topic is required")
		return
	}

	if err := s.manager.Continue(req.Topic); err != nil {
		respondError(w, http.StatusConflict, err.Error())
		return
	}

	// Broadcast topic change to all clients
	s.hub.Broadcast(map[string]interface{}{
		"type":      "topic_changed",
		"topic":     req.Topic,
		"continued": true,
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "continued", "topic": req.Topic})
}

func (s *Server) handleStopDebate(w http.ResponseWriter, r *http.Request) {
	s.manager.Stop()

	// Broadcast to all clients
	s.hub.Broadcast(map[string]interface{}{
		"type": "debate_stopped",
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) handleNextTurn(w http.ResponseWriter, r *http.Request) {
	if !s.manager.IsRunning() {
		respondError(w, http.StatusBadRequest, "Debate is not running")
		return
	}

	// Create channel for streaming
	streamCh := make(chan debate.StreamMessage, 100)

	// Start turn in goroutine
	go func() {
		defer close(streamCh)
		if err := s.manager.NextTurn(streamCh); err != nil {
			log.Printf("Error in NextTurn: %v", err)
		}
	}()

	// Stream to WebSocket clients
	go func() {
		for msg := range streamCh {
			s.hub.Broadcast(msg)
		}
	}()

	respondJSON(w, http.StatusOK, map[string]string{"status": "processing"})
}

func (s *Server) handleAgentTurn(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		respondError(w, http.StatusBadRequest, "Agent ID is required")
		return
	}

	if !s.manager.IsRunning() {
		respondError(w, http.StatusBadRequest, "Debate is not running")
		return
	}

	// Create channel for streaming
	streamCh := make(chan debate.StreamMessage, 100)

	// Start turn in goroutine
	go func() {
		defer close(streamCh)
		if err := s.manager.TurnByAgent(agentID, streamCh); err != nil {
			log.Printf("Error in TurnByAgent: %v", err)
		}
	}()

	// Stream to WebSocket clients
	go func() {
		for msg := range streamCh {
			s.hub.Broadcast(msg)
		}
	}()

	respondJSON(w, http.StatusOK, map[string]string{"status": "processing", "agent_id": agentID})
}

func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	messages := s.manager.GetMessages()
	respondJSON(w, http.StatusOK, messages)
}

type modeRequest struct {
	Mode string `json:"mode"`
}

func (s *Server) handleSetMode(w http.ResponseWriter, r *http.Request) {
	var req modeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	mode := debate.Mode(req.Mode)
	if mode != debate.ModeRoundRobin && mode != debate.ModeFreeForm {
		respondError(w, http.StatusBadRequest, "Invalid mode")
		return
	}

	s.manager.SetMode(mode)

	// Broadcast mode change
	s.hub.Broadcast(map[string]interface{}{
		"type": "mode_changed",
		"mode": mode,
	})

	respondJSON(w, http.StatusOK, map[string]string{"mode": string(mode)})
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	s.manager.Reset()

	// Broadcast reset
	s.hub.Broadcast(map[string]interface{}{
		"type": "debate_reset",
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ServeWs(s.hub, w, r)
}

// Helper functions

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// API Keys handlers

func (s *Server) handleGetAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys := s.storage.GetKeys()
	// Mask the keys for security (only show last 4 chars)
	response := map[string]string{
		"openai":     maskKey(keys.OpenAI),
		"anthropic":  maskKey(keys.Anthropic),
		"gemini":     maskKey(keys.Gemini),
		"ollama_url": keys.OllamaURL,
	}
	respondJSON(w, http.StatusOK, response)
}

type saveKeysRequest struct {
	OpenAI    string `json:"openai"`
	Anthropic string `json:"anthropic"`
	Gemini    string `json:"gemini"`
	OllamaURL string `json:"ollama_url"`
}

func (s *Server) handleSaveAPIKeys(w http.ResponseWriter, r *http.Request) {
	var req saveKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get current keys to preserve unchanged ones
	currentKeys := s.storage.GetKeys()

	// Only update non-empty values (allow clearing by sending empty string)
	newKeys := storage.APIKeys{
		OpenAI:    req.OpenAI,
		Anthropic: req.Anthropic,
		Gemini:    req.Gemini,
		OllamaURL: req.OllamaURL,
	}

	// If a field is the masked version, keep the old value
	if req.OpenAI == maskKey(currentKeys.OpenAI) {
		newKeys.OpenAI = currentKeys.OpenAI
	}
	if req.Anthropic == maskKey(currentKeys.Anthropic) {
		newKeys.Anthropic = currentKeys.Anthropic
	}
	if req.Gemini == maskKey(currentKeys.Gemini) {
		newKeys.Gemini = currentKeys.Gemini
	}

	if err := s.storage.SetKeys(newKeys); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save API keys")
		return
	}

	// Reload agents with new keys
	if s.reloadAgents != nil {
		if err := s.reloadAgents(); err != nil {
			log.Printf("Warning: Failed to reload agents: %v", err)
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Server) handleGetKeyStatus(w http.ResponseWriter, r *http.Request) {
	status := s.storage.GetKeyStatus()
	respondJSON(w, http.StatusOK, status)
}

// maskKey returns a masked version of the key for display
func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "****"
	}
	return "****" + key[len(key)-4:]
}

// Agent management handlers

func (s *Server) handleGetAgentConfigs(w http.ResponseWriter, r *http.Request) {
	if s.agentFuncs == nil || s.agentFuncs.GetConfigs == nil {
		respondError(w, http.StatusNotImplemented, "Agent config management not available")
		return
	}
	configs := s.agentFuncs.GetConfigs()
	// Mask API keys for security
	for i := range configs {
		configs[i].APIKey = maskKey(configs[i].APIKey)
	}
	respondJSON(w, http.StatusOK, configs)
}

func (s *Server) handleAddAgent(w http.ResponseWriter, r *http.Request) {
	if s.agentFuncs == nil || s.agentFuncs.Add == nil {
		respondError(w, http.StatusNotImplemented, "Agent config management not available")
		return
	}

	var agentCfg AgentYAMLConfig
	if err := json.NewDecoder(r.Body).Decode(&agentCfg); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if agentCfg.ID == "" || agentCfg.Name == "" {
		respondError(w, http.StatusBadRequest, "Agent ID and name are required")
		return
	}

	if err := s.agentFuncs.Add(agentCfg); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Reload agents
	if s.agentFuncs.Reload != nil {
		s.agentFuncs.Reload()
	}

	// Broadcast agents update
	s.hub.Broadcast(map[string]interface{}{
		"type": "agents_updated",
	})

	respondJSON(w, http.StatusCreated, map[string]string{"status": "created", "id": agentCfg.ID})
}

func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	if s.agentFuncs == nil || s.agentFuncs.Update == nil {
		respondError(w, http.StatusNotImplemented, "Agent config management not available")
		return
	}

	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		respondError(w, http.StatusBadRequest, "Agent ID is required")
		return
	}

	var agentCfg AgentYAMLConfig
	if err := json.NewDecoder(r.Body).Decode(&agentCfg); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// If API key is masked, get the original from config
	if s.agentFuncs.GetConfigs != nil {
		configs := s.agentFuncs.GetConfigs()
		for _, cfg := range configs {
			if cfg.ID == agentID && agentCfg.APIKey == maskKey(cfg.APIKey) {
				agentCfg.APIKey = cfg.APIKey
				break
			}
		}
	}

	if err := s.agentFuncs.Update(agentID, agentCfg); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Reload agents
	if s.agentFuncs.Reload != nil {
		s.agentFuncs.Reload()
	}

	// Broadcast agents update
	s.hub.Broadcast(map[string]interface{}{
		"type": "agents_updated",
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated", "id": agentID})
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	if s.agentFuncs == nil || s.agentFuncs.Delete == nil {
		respondError(w, http.StatusNotImplemented, "Agent config management not available")
		return
	}

	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		respondError(w, http.StatusBadRequest, "Agent ID is required")
		return
	}

	if err := s.agentFuncs.Delete(agentID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Reload agents
	if s.agentFuncs.Reload != nil {
		s.agentFuncs.Reload()
	}

	// Broadcast agents update
	s.hub.Broadcast(map[string]interface{}{
		"type": "agents_updated",
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": agentID})
}

type reorderRequest struct {
	AgentIDs []string `json:"agent_ids"`
}

func (s *Server) handleReorderAgents(w http.ResponseWriter, r *http.Request) {
	if s.agentFuncs == nil || s.agentFuncs.Reorder == nil {
		respondError(w, http.StatusNotImplemented, "Agent config management not available")
		return
	}

	var req reorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := s.agentFuncs.Reorder(req.AgentIDs); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Reload agents
	if s.agentFuncs.Reload != nil {
		s.agentFuncs.Reload()
	}

	// Broadcast agents update
	s.hub.Broadcast(map[string]interface{}{
		"type": "agents_updated",
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "reordered"})
}

// Full config handlers

type configResponse struct {
	Provider  string                 `json:"provider"`
	OpenAI    providerConfigResponse `json:"openai"`
	Anthropic providerConfigResponse `json:"anthropic"`
	Gemini    providerConfigResponse `json:"gemini"`
	Ollama    providerConfigResponse `json:"ollama"`
	Run       storage.RunConfig      `json:"run"`
}

type providerConfigResponse struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.storage.GetConfig()

	// Mask API keys for security
	response := configResponse{
		Provider: cfg.Provider,
		OpenAI: providerConfigResponse{
			APIKey:  maskKey(cfg.OpenAI.APIKey),
			BaseURL: cfg.OpenAI.BaseURL,
			Model:   cfg.OpenAI.Model,
		},
		Anthropic: providerConfigResponse{
			APIKey:  maskKey(cfg.Anthropic.APIKey),
			BaseURL: cfg.Anthropic.BaseURL,
			Model:   cfg.Anthropic.Model,
		},
		Gemini: providerConfigResponse{
			APIKey:  maskKey(cfg.Gemini.APIKey),
			BaseURL: cfg.Gemini.BaseURL,
			Model:   cfg.Gemini.Model,
		},
		Ollama: providerConfigResponse{
			APIKey:  "",
			BaseURL: cfg.Ollama.BaseURL,
			Model:   cfg.Ollama.Model,
		},
		Run: cfg.Run,
	}

	respondJSON(w, http.StatusOK, response)
}

type saveConfigRequest struct {
	Provider  string                 `json:"provider"`
	OpenAI    storage.ProviderConfig `json:"openai"`
	Anthropic storage.ProviderConfig `json:"anthropic"`
	Gemini    storage.ProviderConfig `json:"gemini"`
	Ollama    storage.ProviderConfig `json:"ollama"`
	Run       storage.RunConfig      `json:"run"`
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	var req saveConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get current config to preserve unchanged API keys
	currentCfg := s.storage.GetConfig()

	// If API key is masked version, keep the old value
	if req.OpenAI.APIKey == maskKey(currentCfg.OpenAI.APIKey) {
		req.OpenAI.APIKey = currentCfg.OpenAI.APIKey
	}
	if req.Anthropic.APIKey == maskKey(currentCfg.Anthropic.APIKey) {
		req.Anthropic.APIKey = currentCfg.Anthropic.APIKey
	}
	if req.Gemini.APIKey == maskKey(currentCfg.Gemini.APIKey) {
		req.Gemini.APIKey = currentCfg.Gemini.APIKey
	}

	newConfig := storage.Config{
		Provider:  req.Provider,
		OpenAI:    req.OpenAI,
		Anthropic: req.Anthropic,
		Gemini:    req.Gemini,
		Ollama:    req.Ollama,
		Run:       req.Run,
	}

	if err := s.storage.SetConfig(newConfig); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save config")
		return
	}

	// Reload agents with new config
	if s.reloadAgents != nil {
		if err := s.reloadAgents(); err != nil {
			log.Printf("Warning: Failed to reload agents: %v", err)
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Server) handleGetRunConfig(w http.ResponseWriter, r *http.Request) {
	runCfg := s.storage.GetRunConfig()
	respondJSON(w, http.StatusOK, runCfg)
}

func (s *Server) handleSaveRunConfig(w http.ResponseWriter, r *http.Request) {
	var req storage.RunConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := s.storage.SetRunConfig(req); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save run config")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}
