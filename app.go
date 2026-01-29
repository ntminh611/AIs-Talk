package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/user/talk/internal/debate"
)

// App struct - Wails application
type App struct {
	ctx     context.Context
	manager *debate.Manager
	mu      sync.RWMutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Load agents and create manager
	agents := loadAgents()
	a.manager = debate.NewManager(agents)

	log.Printf("App started with %d agents", len(agents))
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	if a.manager != nil && a.manager.IsRunning() {
		a.manager.Stop()
	}
	log.Println("App shutdown")
}

// AgentInfoJS represents agent info for JavaScript
type AgentInfoJS struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Color string `json:"color"`
}

// MessageJS represents a message for JavaScript
type MessageJS struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Color     string `json:"color"`
}

// DebateStatusJS represents debate status for JavaScript
type DebateStatusJS struct {
	IsRunning bool   `json:"is_running"`
	Topic     string `json:"topic"`
	Mode      string `json:"mode"`
}

// GetAgents returns list of agents
func (a *App) GetAgents() []AgentInfoJS {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.manager == nil {
		return []AgentInfoJS{}
	}

	agents := a.manager.GetAgents()
	result := make([]AgentInfoJS, len(agents))
	for i, ag := range agents {
		result[i] = AgentInfoJS{
			ID:    ag.ID,
			Name:  ag.Name,
			Role:  ag.Role,
			Color: ag.Color,
		}
	}
	return result
}

// GetStatus returns current debate status
func (a *App) GetStatus() DebateStatusJS {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.manager == nil {
		return DebateStatusJS{}
	}

	return DebateStatusJS{
		IsRunning: a.manager.IsRunning(),
		Topic:     a.manager.GetTopic(),
		Mode:      string(a.manager.GetMode()),
	}
}

// GetMessages returns all messages
func (a *App) GetMessages() []MessageJS {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.manager == nil {
		return []MessageJS{}
	}

	messages := a.manager.GetMessages()
	result := make([]MessageJS, len(messages))
	for i, m := range messages {
		result[i] = MessageJS{
			ID:        m.ID,
			AgentID:   m.AgentID,
			AgentName: m.AgentName,
			Content:   m.Content,
			Timestamp: m.Timestamp.Format(time.RFC3339),
			Color:     m.Color,
		}
	}
	return result
}

// StartDebate starts a new debate with the given topic
func (a *App) StartDebate(topic string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.manager == nil {
		return fmt.Errorf("manager not initialized")
	}

	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	err := a.manager.Start(topic)
	if err != nil {
		return err
	}

	// Emit event to frontend
	runtime.EventsEmit(a.ctx, "debate:started", map[string]string{"topic": topic})

	// Start first turn in background
	go a.executeNextTurn()

	return nil
}

// ContinueDebate continues with a new topic
func (a *App) ContinueDebate(topic string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.manager == nil {
		return fmt.Errorf("manager not initialized")
	}

	err := a.manager.Continue(topic)
	if err != nil {
		return err
	}

	runtime.EventsEmit(a.ctx, "debate:continued", map[string]string{"topic": topic})

	// Start next turn in background
	go a.executeNextTurn()

	return nil
}

// StopDebate stops the current debate
func (a *App) StopDebate() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.manager != nil {
		a.manager.Stop()
		runtime.EventsEmit(a.ctx, "debate:stopped", nil)
	}
}

// ResetDebate resets the debate
func (a *App) ResetDebate() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.manager != nil {
		a.manager.Reset()
		runtime.EventsEmit(a.ctx, "debate:reset", nil)
	}
}

// SetMode sets the debate mode
func (a *App) SetMode(mode string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.manager != nil {
		a.manager.SetMode(debate.Mode(mode))
		runtime.EventsEmit(a.ctx, "mode:changed", map[string]string{"mode": mode})
	}
}

// TriggerNextAgent triggers the next agent's turn
func (a *App) TriggerNextAgent() error {
	if a.manager == nil {
		return fmt.Errorf("manager not initialized")
	}

	if !a.manager.IsRunning() {
		return fmt.Errorf("debate not running")
	}

	go a.executeNextTurn()
	return nil
}

// TriggerAgent triggers a specific agent
func (a *App) TriggerAgent(agentID string) error {
	if a.manager == nil {
		return fmt.Errorf("manager not initialized")
	}

	if !a.manager.IsRunning() {
		return fmt.Errorf("debate not running")
	}

	go a.executeAgentTurn(agentID)
	return nil
}

// executeNextTurn executes the next agent's turn with streaming
func (a *App) executeNextTurn() {
	streamCh := make(chan debate.StreamMessage, 100)

	go func() {
		err := a.manager.NextTurn(streamCh)
		if err != nil {
			log.Printf("NextTurn error: %v", err)
			runtime.EventsEmit(a.ctx, "agent:error", map[string]string{"error": err.Error()})
		}
	}()

	a.processStream(streamCh)
}

// executeAgentTurn executes a specific agent's turn
func (a *App) executeAgentTurn(agentID string) {
	streamCh := make(chan debate.StreamMessage, 100)

	go func() {
		err := a.manager.TurnByAgent(agentID, streamCh)
		if err != nil {
			log.Printf("TurnByAgent error: %v", err)
			runtime.EventsEmit(a.ctx, "agent:error", map[string]string{"error": err.Error()})
		}
	}()

	a.processStream(streamCh)
}

// processStream processes streaming messages and emits events
func (a *App) processStream(streamCh <-chan debate.StreamMessage) {
	for msg := range streamCh {
		switch msg.Type {
		case "start":
			runtime.EventsEmit(a.ctx, "agent:start", map[string]interface{}{
				"agent_id":   msg.AgentID,
				"agent_name": msg.AgentName,
				"message_id": msg.MessageID,
				"color":      msg.Color,
			})
		case "chunk":
			runtime.EventsEmit(a.ctx, "agent:chunk", map[string]interface{}{
				"agent_id":   msg.AgentID,
				"content":    msg.Content,
				"message_id": msg.MessageID,
			})
		case "end":
			runtime.EventsEmit(a.ctx, "agent:end", map[string]interface{}{
				"agent_id":   msg.AgentID,
				"message_id": msg.MessageID,
			})
		case "error":
			runtime.EventsEmit(a.ctx, "agent:error", map[string]interface{}{
				"error": msg.Error,
			})
		}
	}
}

// ReloadAgents reloads agents from config
func (a *App) ReloadAgents() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	agents := loadAgents()
	if a.manager != nil {
		a.manager.UpdateAgents(agents)
	}

	log.Printf("Reloaded %d agents", len(agents))
	runtime.EventsEmit(a.ctx, "agents:reloaded", nil)
	return nil
}

// GetAgentConfigs returns full agent configurations
func (a *App) GetAgentConfigsJS() []AgentYAMLConfig {
	configs := GetAgentConfigs()
	log.Printf("GetAgentConfigsJS called, returning %d configs", len(configs))
	return configs
}

// AddAgentJS adds a new agent
func (a *App) AddAgentJS(cfg AgentYAMLConfig) error {
	err := AddAgent(cfg)
	if err != nil {
		return err
	}
	return a.ReloadAgents()
}

// UpdateAgentJS updates an agent
func (a *App) UpdateAgentJS(id string, cfg AgentYAMLConfig) error {
	err := UpdateAgent(id, cfg)
	if err != nil {
		return err
	}
	return a.ReloadAgents()
}

// DeleteAgentJS deletes an agent
func (a *App) DeleteAgentJS(id string) error {
	err := DeleteAgent(id)
	if err != nil {
		return err
	}
	return a.ReloadAgents()
}

// ReorderAgentsJS reorders agents
func (a *App) ReorderAgentsJS(ids []string) error {
	err := ReorderAgents(ids)
	if err != nil {
		return err
	}
	return a.ReloadAgents()
}

// GetManager returns the debate manager (internal use)
func (a *App) GetManager() *debate.Manager {
	return a.manager
}

// SaveToFile opens a save dialog and writes content to the selected file
func (a *App) SaveToFile(content string, defaultFilename string) (string, error) {
	filepath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultFilename,
		Title:           "Lưu file",
		Filters: []runtime.FileFilter{
			{DisplayName: "Markdown Files", Pattern: "*.md"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	if filepath == "" {
		return "", nil // User cancelled
	}

	// Write content with BOM for UTF-8
	bom := "\uFEFF"
	if err := os.WriteFile(filepath, []byte(bom+content), 0644); err != nil {
		return "", err
	}

	return filepath, nil
}

// OpenFile opens a file dialog and returns the file content
func (a *App) OpenFile() (string, error) {
	filepath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Mở file",
		Filters: []runtime.FileFilter{
			{DisplayName: "Markdown Files", Pattern: "*.md"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	if filepath == "" {
		return "", nil // User cancelled
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}

	// Remove BOM if present
	content := string(data)
	if len(content) >= 3 && content[:3] == "\uFEFF" {
		content = content[3:]
	}

	return content, nil
}
