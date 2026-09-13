package evaluation

import (
	"context"
	"errors"
	"sync"
	"time"

	expertLLM "github.com/gustian305/backend/internal/service/expert_system/llm"
)

type ScriptedClient struct {
	Response string
	Err      error
	Delay    time.Duration
	Model    string

	mu         sync.Mutex
	LastPrompt string
}

func (c *ScriptedClient) Generate(ctx context.Context, prompt string) (string, error) {
	c.mu.Lock()
	c.LastPrompt = prompt
	c.mu.Unlock()

	if c.Delay > 0 {
		timer := time.NewTimer(c.Delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	if c.Err != nil {
		return "", c.Err
	}
	return c.Response, nil
}

func (c *ScriptedClient) ModelName() string {
	if c == nil || c.Model == "" {
		return "evaluation/scripted"
	}
	return c.Model
}

func (c *ScriptedClient) Prompt() string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.LastPrompt
}

type RecordingClient struct {
	Base expertLLM.LLMClient

	mu           sync.Mutex
	LastPrompt   string
	LastResponse string
	LastError    error
	LastStarted  time.Time
	LastDuration time.Duration
}

func (c *RecordingClient) Generate(ctx context.Context, prompt string) (string, error) {
	startedAt := time.Now()
	if c == nil || c.Base == nil {
		return "", errors.New("recording llm client is not configured")
	}

	response, err := c.Base.Generate(ctx, prompt)
	c.mu.Lock()
	c.LastPrompt = prompt
	c.LastResponse = response
	c.LastError = err
	c.LastStarted = startedAt
	c.LastDuration = time.Since(startedAt)
	c.mu.Unlock()
	return response, err
}

func (c *RecordingClient) ModelName() string {
	if c == nil || c.Base == nil {
		return ""
	}
	if provider, ok := c.Base.(interface{ ModelName() string }); ok {
		return provider.ModelName()
	}
	return ""
}

func (c *RecordingClient) Snapshot() (prompt string, response string, err error, startedAt time.Time, duration time.Duration) {
	if c == nil {
		return "", "", nil, time.Time{}, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.LastPrompt, c.LastResponse, c.LastError, c.LastStarted, c.LastDuration
}

func failureClient(mode string) *ScriptedClient {
	client := &ScriptedClient{Model: "evaluation/failure"}
	switch mode {
	case "timeout":
		client.Err = context.DeadlineExceeded
	case "empty_response":
		client.Response = ""
	case "invalid_format":
		client.Response = "Jawaban bebas tanpa struktur yang diwajibkan."
	case "rate_limit":
		client.Err = errors.New("429 rate limit exceeded")
	default:
		client.Err = errors.New("simulated network error")
	}
	return client
}
