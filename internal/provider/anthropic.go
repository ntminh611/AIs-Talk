package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Anthropic provider implementation
type Anthropic struct {
	apiKey  string
	model   string
	baseURL string
}

// NewAnthropic creates a new Anthropic provider
func NewAnthropic(apiKey, model, baseURL string) *Anthropic {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}
	return &Anthropic{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
	}
}

func (a *Anthropic) Name() string {
	return "anthropic"
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicStreamEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
	} `json:"content_block"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

func (a *Anthropic) Chat(ctx context.Context, messages []Message, opts Options) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 100)

	// Extract system message and convert others
	var systemMsg string
	var anthMessages []anthropicMessage

	for _, m := range messages {
		if m.Role == "system" {
			systemMsg = m.Content
		} else {
			role := m.Role
			if role == "assistant" {
				role = "assistant"
			} else {
				role = "user"
			}
			anthMessages = append(anthMessages, anthropicMessage{
				Role:    role,
				Content: m.Content,
			})
		}
	}

	// Ensure we have at least one user message
	if len(anthMessages) == 0 {
		anthMessages = append(anthMessages, anthropicMessage{
			Role:    "user",
			Content: "Hello",
		})
	}

	model := opts.Model
	if model == "" {
		model = a.model
	}

	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		// Thinking models require higher max_tokens (must be > thinking.budget_tokens)
		if strings.Contains(strings.ToLower(model), "thinking") {
			maxTokens = 64000
		} else {
			maxTokens = 4096
		}
	}

	reqBody := anthropicRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    systemMsg,
		Messages:  anthMessages,
		Stream:    true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	go func() {
		defer close(ch)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("request failed: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			ch <- StreamChunk{Error: fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))}
			return
		}

		reader := bufio.NewReader(resp.Body)
		// Track if we're inside a thinking block (should not output)
		isThinkingBlock := false
		currentBlockIndex := -1

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					ch <- StreamChunk{Done: true}
					return
				}
				ch <- StreamChunk{Error: fmt.Errorf("read error: %w", err)}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "content_block_start":
				currentBlockIndex = event.Index
				// Check if this is a thinking block (should be skipped)
				if event.ContentBlock.Type == "thinking" {
					isThinkingBlock = true
				} else {
					isThinkingBlock = false
				}
			case "content_block_stop":
				if event.Index == currentBlockIndex {
					isThinkingBlock = false
				}
			case "content_block_delta":
				// Only output text from non-thinking blocks
				if !isThinkingBlock && event.Delta.Text != "" {
					ch <- StreamChunk{Content: event.Delta.Text}
				}
			case "message_stop":
				ch <- StreamChunk{Done: true}
				return
			}
		}
	}()

	return ch, nil
}
