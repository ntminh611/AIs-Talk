package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/user/talk/internal/agent"
	"github.com/user/talk/internal/debate"
	"github.com/user/talk/internal/provider"
	"github.com/user/talk/internal/server"
	"github.com/user/talk/internal/storage"
	"gopkg.in/yaml.v3"
)

//go:embed web/static/*
var staticFS embed.FS

// Config represents the application configuration
type Config struct {
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	Agents []AgentYAMLConfig `yaml:"agents"`
}

// AgentYAMLConfig represents agent configuration from YAML
type AgentYAMLConfig struct {
	ID               string  `yaml:"id"`
	Name             string  `yaml:"name"`
	Role             string  `yaml:"role"`
	SystemPrompt     string  `yaml:"system_prompt"`
	Provider         string  `yaml:"provider"`
	Model            string  `yaml:"model"`
	Color            string  `yaml:"color"`
	APIKey           string  `yaml:"api_key"`
	BaseURL          string  `yaml:"base_url"`
	Temperature      float64 `yaml:"temperature,omitempty"`
	MaxTokens        int     `yaml:"max_tokens,omitempty"`
	TopP             float64 `yaml:"top_p,omitempty"`
	TopK             int     `yaml:"top_k,omitempty"`
	FrequencyPenalty float64 `yaml:"frequency_penalty,omitempty"`
	PresencePenalty  float64 `yaml:"presence_penalty,omitempty"`
}

// Global state for reloading
var (
	globalStore   *storage.Storage
	globalManager *debate.Manager
	configPath    string
	managerMu     sync.RWMutex
	globalConfig  *Config
	configMu      sync.RWMutex
)

func main() {
	configPathFlag := flag.String("config", "config.yaml", "Path to config file")
	port := flag.String("port", "8080", "Server port")
	keysFile := flag.String("keys", "api_keys.json", "Path to API keys file")
	flag.Parse()

	configPath = *configPathFlag

	// Initialize storage
	globalStore = storage.NewStorage(*keysFile)
	log.Printf("API keys file: %s", *keysFile)

	// Load agents
	agents := loadAgents()

	if len(agents) == 0 {
		log.Println("Warning: No agents configured. Please add API keys via the web UI.")
	} else {
		log.Printf("Loaded %d agents", len(agents))
		for _, a := range agents {
			log.Printf("  - %s (%s) using %s", a.Name, a.ID, a.ProviderType)
		}
	}

	// Create debate manager
	globalManager = debate.NewManager(agents)

	// Create and start server with reload callback
	srv := server.NewServer(globalManager, staticFS, globalStore, reloadAgents)

	// Set agent config management functions
	srv.SetAgentFuncs(&server.AgentConfigFuncs{
		GetConfigs: func() []server.AgentYAMLConfig {
			configs := GetAgentConfigs()
			result := make([]server.AgentYAMLConfig, len(configs))
			for i, c := range configs {
				result[i] = server.AgentYAMLConfig{
					ID:               c.ID,
					Name:             c.Name,
					Role:             c.Role,
					SystemPrompt:     c.SystemPrompt,
					Provider:         c.Provider,
					Model:            c.Model,
					Color:            c.Color,
					APIKey:           c.APIKey,
					BaseURL:          c.BaseURL,
					Temperature:      c.Temperature,
					MaxTokens:        c.MaxTokens,
					TopP:             c.TopP,
					TopK:             c.TopK,
					FrequencyPenalty: c.FrequencyPenalty,
					PresencePenalty:  c.PresencePenalty,
				}
			}
			return result
		},
		Add: func(cfg server.AgentYAMLConfig) error {
			return AddAgent(AgentYAMLConfig{
				ID:               cfg.ID,
				Name:             cfg.Name,
				Role:             cfg.Role,
				SystemPrompt:     cfg.SystemPrompt,
				Provider:         cfg.Provider,
				Model:            cfg.Model,
				Color:            cfg.Color,
				APIKey:           cfg.APIKey,
				BaseURL:          cfg.BaseURL,
				Temperature:      cfg.Temperature,
				MaxTokens:        cfg.MaxTokens,
				TopP:             cfg.TopP,
				TopK:             cfg.TopK,
				FrequencyPenalty: cfg.FrequencyPenalty,
				PresencePenalty:  cfg.PresencePenalty,
			})
		},
		Update: func(id string, cfg server.AgentYAMLConfig) error {
			return UpdateAgent(id, AgentYAMLConfig{
				ID:               cfg.ID,
				Name:             cfg.Name,
				Role:             cfg.Role,
				SystemPrompt:     cfg.SystemPrompt,
				Provider:         cfg.Provider,
				Model:            cfg.Model,
				Color:            cfg.Color,
				APIKey:           cfg.APIKey,
				BaseURL:          cfg.BaseURL,
				Temperature:      cfg.Temperature,
				MaxTokens:        cfg.MaxTokens,
				TopP:             cfg.TopP,
				TopK:             cfg.TopK,
				FrequencyPenalty: cfg.FrequencyPenalty,
				PresencePenalty:  cfg.PresencePenalty,
			})
		},
		Delete: func(id string) error {
			return DeleteAgent(id)
		},
		Reorder: func(ids []string) error {
			return ReorderAgents(ids)
		},
		Reload: reloadAgents,
	})

	addr := ":" + *port
	log.Printf("Starting server on http://localhost%s", addr)
	if err := srv.Run(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// reloadAgents reloads all agents with updated API keys
func reloadAgents() error {
	managerMu.Lock()
	defer managerMu.Unlock()

	log.Println("Reloading agents with new API keys...")
	agents := loadAgents()

	if len(agents) == 0 {
		log.Println("Warning: No agents available after reload")
	} else {
		log.Printf("Reloaded %d agents", len(agents))
		for _, a := range agents {
			log.Printf("  - %s (%s) using %s", a.Name, a.ID, a.ProviderType)
		}
	}

	// Update the manager with new agents
	globalManager.UpdateAgents(agents)
	return nil
}

// loadAgents loads agents from config file or defaults
func loadAgents() []*agent.Agent {
	var agents []*agent.Agent

	if _, err := os.Stat(configPath); err == nil {
		var loadErr error
		agents, loadErr = loadAgentsFromConfig(configPath)
		if loadErr != nil {
			log.Printf("Warning: Failed to load config: %v. Using defaults.", loadErr)
			agents = loadDefaultAgents()
		}
	} else {
		agents = loadDefaultAgents()
	}

	return agents
}

func loadAgentsFromConfig(path string) ([]*agent.Agent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Store config globally
	configMu.Lock()
	globalConfig = &config
	configMu.Unlock()

	var agents []*agent.Agent
	for _, ac := range config.Agents {
		// Get API key from config, then storage, then environment
		apiKey := ac.APIKey
		if apiKey == "" {
			apiKey = getAPIKey(ac.Provider)
		}

		baseURL := ac.BaseURL
		if ac.Provider == "ollama" && baseURL == "" {
			baseURL = globalStore.GetOllamaURL()
		}

		cfg := agent.AgentConfig{
			ID:               ac.ID,
			Name:             ac.Name,
			Role:             ac.Role,
			SystemPrompt:     ac.SystemPrompt,
			ProviderType:     ac.Provider,
			Model:            ac.Model,
			Color:            ac.Color,
			Temperature:      ac.Temperature,
			MaxTokens:        ac.MaxTokens,
			TopP:             ac.TopP,
			TopK:             ac.TopK,
			FrequencyPenalty: ac.FrequencyPenalty,
			PresencePenalty:  ac.PresencePenalty,
			ProviderConfig: provider.Config{
				Type:             ac.Provider,
				APIKey:           apiKey,
				Model:            ac.Model,
				BaseURL:          baseURL,
				Temperature:      ac.Temperature,
				MaxTokens:        ac.MaxTokens,
				TopP:             ac.TopP,
				TopK:             ac.TopK,
				FrequencyPenalty: ac.FrequencyPenalty,
				PresencePenalty:  ac.PresencePenalty,
			},
		}

		a, err := agent.NewAgent(cfg)
		if err != nil {
			log.Printf("Warning: Failed to create agent %s: %v", ac.ID, err)
			continue
		}
		agents = append(agents, a)
	}

	return agents, nil
}

func loadDefaultAgents() []*agent.Agent {
	defaults := agent.DefaultAgents()
	var agents []*agent.Agent

	for _, cfg := range defaults {
		apiKey := getAPIKey(cfg.ProviderType)

		// Skip if no API key available (except Ollama which doesn't need one)
		if apiKey == "" && cfg.ProviderType != "ollama" {
			log.Printf("Skipping agent %s: no API key for %s", cfg.ID, cfg.ProviderType)
			continue
		}

		baseURL := ""
		if cfg.ProviderType == "ollama" {
			baseURL = globalStore.GetOllamaURL()
		}

		cfg.ProviderConfig = provider.Config{
			Type:    cfg.ProviderType,
			APIKey:  apiKey,
			Model:   cfg.Model,
			BaseURL: baseURL,
		}

		a, err := agent.NewAgent(cfg)
		if err != nil {
			log.Printf("Warning: Failed to create agent %s: %v", cfg.ID, err)
			continue
		}
		agents = append(agents, a)
	}

	return agents
}

// getAPIKey gets API key from storage first, then falls back to environment
func getAPIKey(providerType string) string {
	switch providerType {
	case "openai":
		return globalStore.GetOpenAI()
	case "anthropic":
		return globalStore.GetAnthropic()
	case "gemini":
		return globalStore.GetGemini()
	case "ollama":
		return "" // Ollama doesn't need an API key
	case "grok":
		return globalStore.GetGrok()
	case "deepseek":
		return globalStore.GetDeepSeek()
	case "perplexity":
		return globalStore.GetPerplexity()
	default:
		return ""
	}
}

// GetConfig returns the current config
func GetConfig() *Config {
	configMu.RLock()
	defer configMu.RUnlock()
	if globalConfig == nil {
		return &Config{}
	}
	return globalConfig
}

// GetAgentConfigs returns all agent configs
func GetAgentConfigs() []AgentYAMLConfig {
	configMu.RLock()
	defer configMu.RUnlock()
	if globalConfig == nil {
		return []AgentYAMLConfig{}
	}
	return globalConfig.Agents
}

// SaveConfig saves the config to YAML file
func SaveConfig(config *Config) error {
	configMu.Lock()
	defer configMu.Unlock()

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	globalConfig = config
	return nil
}

// AddAgent adds a new agent to config and saves
func AddAgent(agentCfg AgentYAMLConfig) error {
	configMu.Lock()
	defer configMu.Unlock()

	if globalConfig == nil {
		globalConfig = &Config{}
	}

	// Check for duplicate ID
	for _, a := range globalConfig.Agents {
		if a.ID == agentCfg.ID {
			return fmt.Errorf("agent with ID %s already exists", agentCfg.ID)
		}
	}

	globalConfig.Agents = append(globalConfig.Agents, agentCfg)

	data, err := yaml.Marshal(globalConfig)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// UpdateAgent updates an existing agent in config and saves
func UpdateAgent(agentID string, agentCfg AgentYAMLConfig) error {
	configMu.Lock()
	defer configMu.Unlock()

	if globalConfig == nil {
		return fmt.Errorf("config not loaded")
	}

	found := false
	for i, a := range globalConfig.Agents {
		if a.ID == agentID {
			globalConfig.Agents[i] = agentCfg
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	data, err := yaml.Marshal(globalConfig)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// DeleteAgent deletes an agent from config and saves
func DeleteAgent(agentID string) error {
	configMu.Lock()
	defer configMu.Unlock()

	if globalConfig == nil {
		return fmt.Errorf("config not loaded")
	}

	found := false
	newAgents := make([]AgentYAMLConfig, 0)
	for _, a := range globalConfig.Agents {
		if a.ID == agentID {
			found = true
			continue
		}
		newAgents = append(newAgents, a)
	}

	if !found {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	globalConfig.Agents = newAgents

	data, err := yaml.Marshal(globalConfig)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// ReorderAgents reorders agents based on the provided IDs order
func ReorderAgents(agentIDs []string) error {
	configMu.Lock()
	defer configMu.Unlock()

	if globalConfig == nil {
		return fmt.Errorf("config not loaded")
	}

	// Create a map for quick lookup
	agentMap := make(map[string]AgentYAMLConfig)
	for _, a := range globalConfig.Agents {
		agentMap[a.ID] = a
	}

	// Reorder based on provided IDs
	newAgents := make([]AgentYAMLConfig, 0, len(agentIDs))
	for _, id := range agentIDs {
		if agent, ok := agentMap[id]; ok {
			newAgents = append(newAgents, agent)
			delete(agentMap, id)
		}
	}

	// Append any remaining agents not in the list
	for _, agent := range agentMap {
		newAgents = append(newAgents, agent)
	}

	globalConfig.Agents = newAgents

	data, err := yaml.Marshal(globalConfig)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
