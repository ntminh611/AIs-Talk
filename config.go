// config.go contains shared configuration code used by both web server and Wails desktop app
package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/user/talk/internal/agent"
	"github.com/user/talk/internal/provider"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	Agents []AgentYAMLConfig `yaml:"agents"`
}

// AgentYAMLConfig represents agent configuration from YAML
type AgentYAMLConfig struct {
	ID               string  `yaml:"id" json:"id"`
	Name             string  `yaml:"name" json:"name"`
	Role             string  `yaml:"role" json:"role"`
	SystemPrompt     string  `yaml:"system_prompt" json:"system_prompt"`
	Provider         string  `yaml:"provider" json:"provider"`
	Model            string  `yaml:"model" json:"model"`
	Color            string  `yaml:"color" json:"color"`
	APIKey           string  `yaml:"api_key" json:"api_key,omitempty"`
	BaseURL          string  `yaml:"base_url" json:"base_url,omitempty"`
	Temperature      float64 `yaml:"temperature,omitempty" json:"temperature"`
	MaxTokens        int     `yaml:"max_tokens,omitempty" json:"max_tokens"`
	TopP             float64 `yaml:"top_p,omitempty" json:"top_p"`
	TopK             int     `yaml:"top_k,omitempty" json:"top_k"`
	FrequencyPenalty float64 `yaml:"frequency_penalty,omitempty" json:"frequency_penalty"`
	PresencePenalty  float64 `yaml:"presence_penalty,omitempty" json:"presence_penalty"`
}

// Global state for configuration
var (
	configPath   string
	globalConfig *Config
	configMu     sync.RWMutex
)

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
		log.Printf("Config file not found at %s, using defaults", configPath)
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
		// API key must be defined in config.yaml
		apiKey := ac.APIKey
		baseURL := ac.BaseURL

		// Default Ollama URL if not specified
		if ac.Provider == "ollama" && baseURL == "" {
			baseURL = "http://localhost:11434"
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
		// Default agents won't have API keys - they'll fail to create
		// This is expected behavior - user should configure agents in config.yaml
		cfg.ProviderConfig = provider.Config{
			Type:  cfg.ProviderType,
			Model: cfg.Model,
		}

		a, err := agent.NewAgent(cfg)
		if err != nil {
			log.Printf("Skipping default agent %s: %v", cfg.ID, err)
			continue
		}
		agents = append(agents, a)
	}

	return agents
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
		log.Printf("WARNING: GetAgentConfigs called but globalConfig is nil")
		return []AgentYAMLConfig{}
	}
	log.Printf("GetAgentConfigs: globalConfig has %d agents", len(globalConfig.Agents))
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
