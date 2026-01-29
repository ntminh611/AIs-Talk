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

// OpenAI provider implementation
type OpenAI struct {
	apiKey  string
	model   string
	baseURL string
}

// NewOpenAI creates a new OpenAI provider
func NewOpenAI(apiKey, model, baseURL string) *OpenAI {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4"
	}
	return &OpenAI{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
	}
}

func (o *OpenAI) Name() string {
	return "openai"
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func (o *OpenAI) Chat(ctx context.Context, messages []Message, opts Options) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 100)

	// Convert messages
	oaiMessages := make([]openAIMessage, len(messages))
	for i, m := range messages {
		oaiMessages[i] = openAIMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	model := opts.Model
	if model == "" {
		model = o.model
	}

	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		// Models with extended thinking require higher max_tokens (must be > thinking.budget_tokens)
		// Claude Opus 4/4.5 models have extended thinking enabled by default
		modelLower := strings.ToLower(model)
		if strings.Contains(modelLower, "thinking") ||
			strings.Contains(modelLower, "claude-opus-4") ||
			strings.Contains(modelLower, "opus-4") {
			maxTokens = 64000
		} else {
			maxTokens = 4096
		}
	}

	reqBody := openAIRequest{
		Model:       model,
		Messages:    oaiMessages,
		Stream:      true,
		MaxTokens:   maxTokens,
		Temperature: opts.Temperature,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

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

			var streamResp openAIStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) > 0 {
				content := streamResp.Choices[0].Delta.Content
				if content != "" {
					// Filter out any thinking block content that may slip through proxy
					// Skip content that looks like thinking block markers or signatures
					lowerContent := strings.ToLower(content)
					if strings.Contains(lowerContent, "thinking") ||
						strings.Contains(lowerContent, "signature") ||
						strings.HasPrefix(content, "<") && strings.Contains(content, ">") && strings.Contains(lowerContent, "think") {
						continue
					}
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
