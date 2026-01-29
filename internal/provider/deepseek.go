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

// DeepSeek provider implementation
// Uses OpenAI-compatible API
type DeepSeek struct {
	apiKey  string
	model   string
	baseURL string
}

// NewDeepSeek creates a new DeepSeek provider
func NewDeepSeek(apiKey, model, baseURL string) *DeepSeek {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	if model == "" {
		model = "deepseek-chat"
	}
	return &DeepSeek{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
	}
}

func (d *DeepSeek) Name() string {
	return "deepseek"
}

type deepseekRequest struct {
	Model       string            `json:"model"`
	Messages    []deepseekMessage `json:"messages"`
	Stream      bool              `json:"stream"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
}

type deepseekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepseekStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func (d *DeepSeek) Chat(ctx context.Context, messages []Message, opts Options) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 100)

	// Convert messages
	dsMessages := make([]deepseekMessage, len(messages))
	for i, m := range messages {
		dsMessages[i] = deepseekMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	model := opts.Model
	if model == "" {
		model = d.model
	}

	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	reqBody := deepseekRequest{
		Model:       model,
		Messages:    dsMessages,
		Stream:      true,
		MaxTokens:   maxTokens,
		Temperature: opts.Temperature,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", d.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

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
			if data == "[DONE]" {
				ch <- StreamChunk{Done: true}
				return
			}

			var streamResp deepseekStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) > 0 {
				content := streamResp.Choices[0].Delta.Content
				if content != "" {
					ch <- StreamChunk{Content: content}
				}
				if streamResp.Choices[0].FinishReason == "stop" {
					ch <- StreamChunk{Done: true}
					return
				}
			}
		}
	}()

	return ch, nil
}
