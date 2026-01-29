package agent

import (
	"context"
	"fmt"

	"github.com/user/talk/internal/provider"
)

// Agent represents an AI agent with a specific role and provider
type Agent struct {
	ID               string            `json:"id" yaml:"id"`
	Name             string            `json:"name" yaml:"name"`
	Role             string            `json:"role" yaml:"role"`
	SystemPrompt     string            `json:"system_prompt" yaml:"system_prompt"`
	ProviderType     string            `json:"provider_type" yaml:"provider_type"`
	Model            string            `json:"model" yaml:"model"`
	Color            string            `json:"color" yaml:"color"`
	Temperature      float64           `json:"temperature" yaml:"temperature"`
	MaxTokens        int               `json:"max_tokens" yaml:"max_tokens"`
	TopP             float64           `json:"top_p" yaml:"top_p"`
	TopK             int               `json:"top_k" yaml:"top_k"`
	FrequencyPenalty float64           `json:"frequency_penalty" yaml:"frequency_penalty"`
	PresencePenalty  float64           `json:"presence_penalty" yaml:"presence_penalty"`
	Provider         provider.Provider `json:"-" yaml:"-"`
}

// AgentConfig holds configuration for creating an agent
type AgentConfig struct {
	ID               string          `yaml:"id"`
	Name             string          `yaml:"name"`
	Role             string          `yaml:"role"`
	SystemPrompt     string          `yaml:"system_prompt"`
	ProviderType     string          `yaml:"provider_type"`
	Model            string          `yaml:"model"`
	Color            string          `yaml:"color"`
	Temperature      float64         `yaml:"temperature"`
	MaxTokens        int             `yaml:"max_tokens"`
	TopP             float64         `yaml:"top_p"`
	TopK             int             `yaml:"top_k"`
	FrequencyPenalty float64         `yaml:"frequency_penalty"`
	PresencePenalty  float64         `yaml:"presence_penalty"`
	ProviderConfig   provider.Config `yaml:"provider_config"`
}

// Chat sends a message to the agent and returns a streaming response
func (a *Agent) Chat(ctx context.Context, messages []provider.Message, opts provider.Options) (<-chan provider.StreamChunk, error) {
	if a.Provider == nil {
		return nil, fmt.Errorf("provider not initialized for agent %s", a.ID)
	}

	// Prepend system prompt
	fullMessages := make([]provider.Message, 0, len(messages)+1)
	if a.SystemPrompt != "" {
		fullMessages = append(fullMessages, provider.Message{
			Role:    "system",
			Content: a.SystemPrompt,
		})
	}
	fullMessages = append(fullMessages, messages...)

	// Use agent's model if not specified
	if opts.Model == "" {
		opts.Model = a.Model
	}

	// Use agent's parameters if not specified
	if opts.Temperature == 0 && a.Temperature > 0 {
		opts.Temperature = a.Temperature
	}
	if opts.MaxTokens == 0 && a.MaxTokens > 0 {
		opts.MaxTokens = a.MaxTokens
	}
	if opts.TopP == 0 && a.TopP > 0 {
		opts.TopP = a.TopP
	}
	if opts.TopK == 0 && a.TopK > 0 {
		opts.TopK = a.TopK
	}
	if opts.FrequencyPenalty == 0 && a.FrequencyPenalty != 0 {
		opts.FrequencyPenalty = a.FrequencyPenalty
	}
	if opts.PresencePenalty == 0 && a.PresencePenalty != 0 {
		opts.PresencePenalty = a.PresencePenalty
	}

	return a.Provider.Chat(ctx, fullMessages, opts)
}

// NewAgent creates a new agent from config
func NewAgent(cfg AgentConfig) (*Agent, error) {
	prov, err := provider.CreateProvider(cfg.ProviderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider for agent %s: %w", cfg.ID, err)
	}

	return &Agent{
		ID:               cfg.ID,
		Name:             cfg.Name,
		Role:             cfg.Role,
		SystemPrompt:     cfg.SystemPrompt,
		ProviderType:     cfg.ProviderType,
		Model:            cfg.Model,
		Color:            cfg.Color,
		Temperature:      cfg.Temperature,
		MaxTokens:        cfg.MaxTokens,
		TopP:             cfg.TopP,
		TopK:             cfg.TopK,
		FrequencyPenalty: cfg.FrequencyPenalty,
		PresencePenalty:  cfg.PresencePenalty,
		Provider:         prov,
	}, nil
}

// DefaultAgents returns the default set of 4 agents
func DefaultAgents() []AgentConfig {
	return []AgentConfig{
		{
			ID:   "analyst",
			Name: "Analyst",
			Role: "Phân tích logic, data-driven",
			SystemPrompt: `Bạn là Analyst - một chuyên gia phân tích logic và dữ liệu.
Vai trò của bạn:
- Phân tích vấn đề một cách logic, có hệ thống
- Đưa ra các luận điểm dựa trên dữ liệu và bằng chứng
- Đánh giá các khía cạnh khác nhau của vấn đề
- Trả lời ngắn gọn, súc tích, tập trung vào logic

Khi tham gia thảo luận, hãy:
1. Phân tích các ý kiến trước đó
2. Đưa ra góc nhìn phân tích của bạn
3. Hỗ trợ bằng logic và dữ liệu nếu có`,
			ProviderType: "openai",
			Model:        "gpt-4",
			Color:        "#4A90D9",
		},
		{
			ID:   "creative",
			Name: "Creative",
			Role: "Sáng tạo, đưa ý tưởng mới",
			SystemPrompt: `Bạn là Creative - một người sáng tạo và đổi mới.
Vai trò của bạn:
- Đưa ra các ý tưởng mới, độc đáo
- Nghĩ ngoài khuôn khổ thông thường
- Tìm kiếm giải pháp sáng tạo
- Kết nối các ý tưởng khác nhau

Khi tham gia thảo luận, hãy:
1. Đưa ra góc nhìn mới mẻ
2. Đề xuất ý tưởng sáng tạo
3. Mở rộng khả năng và tiềm năng của vấn đề`,
			ProviderType: "anthropic",
			Model:        "claude-3-5-sonnet-20241022",
			Color:        "#9B59B6",
		},
		{
			ID:   "critic",
			Name: "Critic",
			Role: "Phản biện, tìm lỗ hổng",
			SystemPrompt: `Bạn là Critic - một nhà phản biện sắc sảo.
Vai trò của bạn:
- Tìm ra các điểm yếu và lỗ hổng trong lập luận
- Đặt câu hỏi thách thức các giả định
- Đưa ra các phản biện có tính xây dựng
- Giúp củng cố và hoàn thiện ý tưởng

Khi tham gia thảo luận, hãy:
1. Đánh giá các ý kiến một cách phê phán
2. Chỉ ra những điểm chưa được xem xét
3. Đề xuất cách khắc phục các vấn đề`,
			ProviderType: "gemini",
			Model:        "gemini-pro",
			Color:        "#E74C3C",
		},
		{
			ID:   "synthesizer",
			Name: "Synthesizer",
			Role: "Tổng hợp, tìm điểm chung",
			SystemPrompt: `Bạn là Synthesizer - người tổng hợp và hòa giải.
Vai trò của bạn:
- Tổng hợp các ý kiến khác nhau
- Tìm điểm chung giữa các quan điểm
- Xây dựng sự đồng thuận
- Đề xuất giải pháp tích hợp

Khi tham gia thảo luận, hãy:
1. Tóm tắt các ý kiến đã đưa ra
2. Tìm điểm mạnh của mỗi quan điểm
3. Đề xuất hướng đi tổng hợp`,
			ProviderType: "ollama",
			Model:        "llama2",
			Color:        "#27AE60",
		},
	}
}

// AgentInfo returns public info about an agent (for API response)
type AgentInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Color string `json:"color"`
}

// Info returns public info about the agent
func (a *Agent) Info() AgentInfo {
	return AgentInfo{
		ID:    a.ID,
		Name:  a.Name,
		Role:  a.Role,
		Color: a.Color,
	}
}
