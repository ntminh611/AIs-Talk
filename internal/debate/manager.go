package debate

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/user/talk/internal/agent"
	"github.com/user/talk/internal/provider"
)

// Regex patterns for cleaning thinking blocks and signatures
var (
	// XML-style blocks
	thinkingBlockRegex    = regexp.MustCompile(`(?s)<thinking[^>]*>.*?</thinking>`)
	antThinkingBlockRegex = regexp.MustCompile(`(?s)<antThinking[^>]*>.*?</antThinking>`)
	signatureBlockRegex   = regexp.MustCompile(`(?s)<signature[^>]*>.*?</signature>`)

	// Attributes and JSON
	signatureAttrRegex = regexp.MustCompile(`\s*signature\s*=\s*"[^"]*"`)
	jsonThinkingRegex  = regexp.MustCompile(`(?s)\{[^{}]*"type"\s*:\s*"thinking"[^{}]*\}`)

	// More aggressive patterns for any thinking/signature content
	anyThinkingTagRegex  = regexp.MustCompile(`(?i)</?thinking[^>]*>`)
	anySignatureRegex    = regexp.MustCompile(`(?i)"signature"\s*:\s*"[^"]*"`)
	thinkingContentRegex = regexp.MustCompile(`(?s)\[\s*\{[^]]*"type"\s*:\s*"thinking"[^]]*\}\s*\]`)
	contentArrayRegex    = regexp.MustCompile(`(?s)\[\s*\{[^]]*"type"\s*:\s*"(thinking|text)"[^]]*\}\s*(,\s*\{[^]]*\}\s*)*\]`)
)

// Mode represents the debate mode
type Mode string

const (
	ModeRoundRobin Mode = "round_robin"
	ModeFreeForm   Mode = "free_form"
)

// Message represents a debate message
type Message struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	AgentName string    `json:"agent_name"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Color     string    `json:"color"`
}

// StreamMessage represents a streaming message chunk
type StreamMessage struct {
	Type      string `json:"type"` // "start", "chunk", "end", "error"
	AgentID   string `json:"agent_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	Content   string `json:"content,omitempty"`
	MessageID string `json:"message_id,omitempty"`
	Color     string `json:"color,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Manager manages the debate between agents
type Manager struct {
	agents           []*agent.Agent
	messages         []Message
	topic            string
	mode             Mode
	currentIndex     int
	isRunning        bool
	isTurnInProgress bool // Prevents concurrent turns
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	msgCounter       int
}

// NewManager creates a new debate manager
func NewManager(agents []*agent.Agent) *Manager {
	return &Manager{
		agents:   agents,
		messages: make([]Message, 0),
		mode:     ModeRoundRobin,
	}
}

// SetMode sets the debate mode
func (m *Manager) SetMode(mode Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mode = mode
}

// GetMode returns current debate mode
func (m *Manager) GetMode() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mode
}

// Start begins a debate with the given topic
func (m *Manager) Start(topic string) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return fmt.Errorf("debate is already running")
	}
	m.topic = topic
	m.messages = make([]Message, 0)
	m.currentIndex = 0
	m.isRunning = true
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.mu.Unlock()
	return nil
}

// Continue changes topic but keeps conversation history
func (m *Manager) Continue(newTopic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Add a system message about topic change
	if len(m.messages) > 0 {
		m.msgCounter++
		m.messages = append(m.messages, Message{
			ID:        fmt.Sprintf("msg_%d", m.msgCounter),
			AgentID:   "system",
			AgentName: "Hệ thống",
			Content:   fmt.Sprintf("--- Chuyển sang chủ đề mới: %s ---", newTopic),
			Timestamp: time.Now(),
			Color:     "#888888",
		})
	}

	m.topic = newTopic
	m.isRunning = true
	m.ctx, m.cancel = context.WithCancel(context.Background())
	return nil
}

// Stop stops the current debate
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
	m.isRunning = false
}

// IsRunning returns whether a debate is currently running
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

// GetTopic returns the current topic
func (m *Manager) GetTopic() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.topic
}

// GetMessages returns all messages
func (m *Manager) GetMessages() []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Message, len(m.messages))
	copy(result, m.messages)
	return result
}

// GetAgents returns agent info
func (m *Manager) GetAgents() []agent.AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]agent.AgentInfo, len(m.agents))
	for i, a := range m.agents {
		result[i] = a.Info()
	}
	return result
}

// NextTurn executes the next agent's turn and streams the response
func (m *Manager) NextTurn(streamCh chan<- StreamMessage) error {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return fmt.Errorf("debate is not running")
	}
	if m.isTurnInProgress {
		m.mu.Unlock()
		return fmt.Errorf("a turn is already in progress")
	}
	if len(m.agents) == 0 {
		m.mu.Unlock()
		return fmt.Errorf("no agents available")
	}
	m.isTurnInProgress = true

	var currentAgent *agent.Agent
	if m.mode == ModeRoundRobin {
		currentAgent = m.agents[m.currentIndex]
		m.currentIndex = (m.currentIndex + 1) % len(m.agents)
	} else {
		// Free form: pick next agent based on context
		currentAgent = m.selectNextAgent()
	}

	ctx := m.ctx
	m.mu.Unlock()

	// Build conversation context
	messages := m.buildContext(currentAgent)

	// Generate message ID
	m.mu.Lock()
	m.msgCounter++
	msgID := fmt.Sprintf("msg_%d", m.msgCounter)
	m.mu.Unlock()

	// Send start event
	streamCh <- StreamMessage{
		Type:      "start",
		AgentID:   currentAgent.ID,
		AgentName: currentAgent.Name,
		MessageID: msgID,
		Color:     currentAgent.Color,
	}

	// Get streaming response
	respCh, err := currentAgent.Chat(ctx, messages, provider.Options{})
	if err != nil {
		streamCh <- StreamMessage{
			Type:  "error",
			Error: err.Error(),
		}
		return err
	}

	// Collect full response
	var fullContent strings.Builder

	for chunk := range respCh {
		if chunk.Error != nil {
			// Check if it's a context cancellation (user stopped debate)
			if chunk.Error == context.Canceled || strings.Contains(chunk.Error.Error(), "context canceled") {
				m.mu.Lock()
				m.isTurnInProgress = false
				m.mu.Unlock()
				// Don't send error, just end gracefully
				streamCh <- StreamMessage{
					Type:      "end",
					AgentID:   currentAgent.ID,
					MessageID: msgID,
				}
				return nil
			}
			m.mu.Lock()
			m.isTurnInProgress = false
			m.mu.Unlock()
			streamCh <- StreamMessage{
				Type:  "error",
				Error: chunk.Error.Error(),
			}
			return chunk.Error
		}

		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
			streamCh <- StreamMessage{
				Type:      "chunk",
				AgentID:   currentAgent.ID,
				Content:   chunk.Content,
				MessageID: msgID,
			}
		}

		if chunk.Done {
			break
		}
	}

	// Store the message (clean content before storing)
	rawContent := fullContent.String()
	cleanedContent := cleanMessageContent(rawContent)

	// Debug: log if content was cleaned
	if len(rawContent) != len(cleanedContent) {
		fmt.Printf("[DEBUG] Content cleaned: %d -> %d bytes\n", len(rawContent), len(cleanedContent))
	}

	m.mu.Lock()
	m.messages = append(m.messages, Message{
		ID:        msgID,
		AgentID:   currentAgent.ID,
		AgentName: currentAgent.Name,
		Content:   cleanedContent,
		Timestamp: time.Now(),
		Color:     currentAgent.Color,
	})
	m.isTurnInProgress = false
	m.mu.Unlock()

	// Send end event
	streamCh <- StreamMessage{
		Type:      "end",
		AgentID:   currentAgent.ID,
		MessageID: msgID,
	}

	return nil
}

// TurnByAgent executes a specific agent's turn by agent ID
func (m *Manager) TurnByAgent(agentID string, streamCh chan<- StreamMessage) error {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return fmt.Errorf("debate is not running")
	}
	if m.isTurnInProgress {
		m.mu.Unlock()
		return fmt.Errorf("a turn is already in progress")
	}
	if len(m.agents) == 0 {
		m.mu.Unlock()
		return fmt.Errorf("no agents available")
	}

	// Find the agent by ID
	var targetAgent *agent.Agent
	for _, a := range m.agents {
		if a.ID == agentID {
			targetAgent = a
			break
		}
	}

	if targetAgent == nil {
		m.mu.Unlock()
		return fmt.Errorf("agent not found: %s", agentID)
	}

	ctx := m.ctx
	m.isTurnInProgress = true
	m.mu.Unlock()

	// Build conversation context
	messages := m.buildContext(targetAgent)

	// Generate message ID
	m.mu.Lock()
	m.msgCounter++
	msgID := fmt.Sprintf("msg_%d", m.msgCounter)
	m.mu.Unlock()

	// Send start event
	streamCh <- StreamMessage{
		Type:      "start",
		AgentID:   targetAgent.ID,
		AgentName: targetAgent.Name,
		MessageID: msgID,
		Color:     targetAgent.Color,
	}

	// Get streaming response
	respCh, err := targetAgent.Chat(ctx, messages, provider.Options{})
	if err != nil {
		streamCh <- StreamMessage{
			Type:  "error",
			Error: err.Error(),
		}
		return err
	}

	// Collect full response
	var fullContent strings.Builder

	for chunk := range respCh {
		if chunk.Error != nil {
			// Check if it's a context cancellation (user stopped debate)
			if chunk.Error == context.Canceled || strings.Contains(chunk.Error.Error(), "context canceled") {
				m.mu.Lock()
				m.isTurnInProgress = false
				m.mu.Unlock()
				// Don't send error, just end gracefully
				streamCh <- StreamMessage{
					Type:      "end",
					AgentID:   targetAgent.ID,
					MessageID: msgID,
				}
				return nil
			}
			m.mu.Lock()
			m.isTurnInProgress = false
			m.mu.Unlock()
			streamCh <- StreamMessage{
				Type:  "error",
				Error: chunk.Error.Error(),
			}
			return chunk.Error
		}

		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
			streamCh <- StreamMessage{
				Type:      "chunk",
				AgentID:   targetAgent.ID,
				Content:   chunk.Content,
				MessageID: msgID,
			}
		}

		if chunk.Done {
			break
		}
	}

	// Store the message (clean content before storing)
	rawContent := fullContent.String()
	cleanedContent := cleanMessageContent(rawContent)

	// Debug: log if content was cleaned
	if len(rawContent) != len(cleanedContent) {
		fmt.Printf("[DEBUG] Content cleaned: %d -> %d bytes\n", len(rawContent), len(cleanedContent))
	}

	m.mu.Lock()
	m.messages = append(m.messages, Message{
		ID:        msgID,
		AgentID:   targetAgent.ID,
		AgentName: targetAgent.Name,
		Content:   cleanedContent,
		Timestamp: time.Now(),
		Color:     targetAgent.Color,
	})
	m.isTurnInProgress = false
	m.mu.Unlock()

	// Send end event
	streamCh <- StreamMessage{
		Type:      "end",
		AgentID:   targetAgent.ID,
		MessageID: msgID,
	}

	return nil
}

// buildContext builds the conversation context for an agent
func (m *Manager) buildContext(currentAgent *agent.Agent) []provider.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messages := make([]provider.Message, 0)

	// Add topic as initial context
	topicMsg := fmt.Sprintf(`Chủ đề thảo luận: "%s"

Các thành viên tham gia:
`, m.topic)

	for _, a := range m.agents {
		topicMsg += fmt.Sprintf("- %s: %s\n", a.Name, a.Role)
	}

	topicMsg += fmt.Sprintf(`
Bạn là %s. Hãy đưa ra ý kiến của bạn về chủ đề này.
Trả lời bằng tiếng Việt, ngắn gọn (2-4 đoạn). KHÔNG cần ghi tên của bạn ở đầu câu trả lời.`, currentAgent.Name)

	messages = append(messages, provider.Message{
		Role:    "user",
		Content: topicMsg,
	})

	// Add previous messages as context
	// IMPORTANT: Always use "user" role for all previous messages to avoid
	// Claude API thinking block signature validation errors.
	// When using thinking models, the API requires valid signatures in assistant
	// messages with thinking blocks, but we cannot preserve those signatures.
	// The solution is to treat all previous messages as "user" role with agent name prefix.
	for i, msg := range m.messages {
		// Skip system messages
		if msg.AgentID == "system" {
			continue
		}

		// Clean content: remove any thinking block markers that may have leaked
		content := cleanMessageContent(msg.Content)

		// Debug: check for any remaining thinking markers
		if strings.Contains(content, "<thinking") || strings.Contains(content, "signature") {
			fmt.Printf("[DEBUG] Message %d still contains thinking markers after cleaning!\n", i)
			fmt.Printf("[DEBUG] First 200 chars: %.200s\n", content)
		}

		// Always add agent name as context prefix for all messages
		// This ensures Claude doesn't try to validate thinking signatures
		content = fmt.Sprintf("**%s** nói:\n%s", msg.AgentName, content)

		messages = append(messages, provider.Message{
			Role:    "user",
			Content: content,
		})
	}

	// Add prompt for continuation
	if len(m.messages) > 0 {
		messages = append(messages, provider.Message{
			Role:    "user",
			Content: "Hãy tiếp tục thảo luận. Phản hồi các ý kiến trước đó và đưa ra quan điểm của bạn. KHÔNG ghi tên bạn ở đầu.",
		})
	}

	return messages
}

// cleanMessageContent removes any thinking block markers or metadata from content
func cleanMessageContent(content string) string {
	if content == "" {
		return ""
	}

	result := content

	// First pass: Remove complete blocks using regex
	result = thinkingBlockRegex.ReplaceAllString(result, "")
	result = antThinkingBlockRegex.ReplaceAllString(result, "")
	result = signatureBlockRegex.ReplaceAllString(result, "")
	result = signatureAttrRegex.ReplaceAllString(result, "")
	result = jsonThinkingRegex.ReplaceAllString(result, "")

	// Additional aggressive cleaning
	result = anyThinkingTagRegex.ReplaceAllString(result, "")
	result = anySignatureRegex.ReplaceAllString(result, "")
	result = thinkingContentRegex.ReplaceAllString(result, "")
	result = contentArrayRegex.ReplaceAllString(result, "")

	// Remove [thinking]...[/thinking] blocks
	for {
		start := strings.Index(result, "[thinking]")
		if start == -1 {
			break
		}
		end := strings.Index(result, "[/thinking]")
		if end == -1 {
			result = result[:start]
			break
		}
		result = result[:start] + result[end+len("[/thinking]"):]
	}

	// Remove lines that contain any thinking/signature markers
	lines := strings.Split(result, "\n")
	cleanLines := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lowerTrimmed := strings.ToLower(trimmed)

		// Skip empty lines that result from cleaning
		if trimmed == "" || trimmed == "," || trimmed == "{" || trimmed == "}" || trimmed == "[" || trimmed == "]" {
			continue
		}

		// Skip lines that contain thinking/signature markers
		if strings.Contains(lowerTrimmed, "thinking") ||
			strings.Contains(lowerTrimmed, "signature") ||
			strings.HasPrefix(trimmed, `"type"`) ||
			strings.HasPrefix(trimmed, `{"type"`) ||
			strings.HasPrefix(trimmed, `[{"type"`) {
			continue
		}

		cleanLines = append(cleanLines, line)
	}

	result = strings.TrimSpace(strings.Join(cleanLines, "\n"))

	// Final cleanup: remove any remaining JSON artifacts
	result = strings.ReplaceAll(result, `""`, "")
	result = strings.ReplaceAll(result, `{}`, "")
	result = strings.ReplaceAll(result, `[]`, "")

	return strings.TrimSpace(result)
}

// selectNextAgent selects the next agent in free-form mode
func (m *Manager) selectNextAgent() *agent.Agent {
	// Simple implementation: rotate but skip if just spoke
	if len(m.messages) == 0 {
		return m.agents[0]
	}

	lastSpeaker := m.messages[len(m.messages)-1].AgentID

	// Find next agent that isn't the last speaker
	for i, a := range m.agents {
		if a.ID != lastSpeaker {
			m.currentIndex = i
			return a
		}
	}

	// Fallback to next in rotation
	m.currentIndex = (m.currentIndex + 1) % len(m.agents)
	return m.agents[m.currentIndex]
}

// Reset clears the debate state
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
	m.messages = make([]Message, 0)
	m.topic = ""
	m.isRunning = false
	m.currentIndex = 0
	m.msgCounter = 0
}

// UpdateAgents updates the agents list (used when API keys change)
func (m *Manager) UpdateAgents(agents []*agent.Agent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents = agents
	m.currentIndex = 0
}
