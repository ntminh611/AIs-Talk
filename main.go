//go:build !wails

package main

import (
	"embed"
	"flag"
	"log"
	"sync"

	"github.com/user/talk/internal/debate"
	"github.com/user/talk/internal/server"
)

//go:embed web/static/*
var staticFS embed.FS

// Global state for web server mode
var (
	globalManager *debate.Manager
	managerMu     sync.RWMutex
)

func main() {
	configPathFlag := flag.String("config", "config.yaml", "Path to config file")
	port := flag.String("port", "8080", "Server port")
	flag.Parse()

	configPath = *configPathFlag

	// Load agents
	agents := loadAgents()

	if len(agents) == 0 {
		log.Println("Warning: No agents configured. Please configure agents in config.yaml.")
	} else {
		log.Printf("Loaded %d agents", len(agents))
		for _, a := range agents {
			log.Printf("  - %s (%s) using %s", a.Name, a.ID, a.ProviderType)
		}
	}

	// Create debate manager
	globalManager = debate.NewManager(agents)

	// Create and start server with reload callback
	srv := server.NewServer(globalManager, staticFS, nil, reloadAgents)

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

// reloadAgents reloads all agents with updated config
func reloadAgents() error {
	managerMu.Lock()
	defer managerMu.Unlock()

	log.Println("Reloading agents...")
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
