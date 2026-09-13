package consultation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gustian305/backend/logger"
)

type OpenAICompatibleClient struct {
	apiURL     string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewOpenAICompatibleClient(
	apiURL string,
	apiKey string,
	model string,
	timeout time.Duration,
) *OpenAICompatibleClient {

	apiURL = strings.TrimRight(
		strings.TrimSpace(apiURL),
		"/",
	)

	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)

	if timeout <= 0 {
		timeout = DefaultLLMTimeout
	}

	if apiURL == "" ||
		apiKey == "" ||
		model == "" {

		return nil
	}

	return &OpenAICompatibleClient{
		apiURL: apiURL,
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type chatCompletionRequest struct {
	Model               string                  `json:"model"`
	Messages            []chatCompletionMessage `json:"messages"`
	Temperature         float64                 `json:"temperature"`
	TopP                float64                 `json:"top_p,omitempty"`
	MaxCompletionTokens int                     `json:"max_completion_tokens"`
	ReasoningEffort     string                  `json:"reasoning_effort,omitempty"`
}

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message      chatCompletionMessage `json:"message"`
		FinishReason string                `json:"finish_reason"`
	} `json:"choices"`

	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *OpenAICompatibleClient) Generate(ctx context.Context, prompt string) (string, error) {
	return c.GenerateChat(
		ctx,
		strings.TrimSpace(`
Anda adalah AI agronomis pendamping budidaya padi.
Jawab secara faktual, singkat, mudah dipahami petani, dan jangan mengarang diagnosis, produk, dosis, atau waktu aplikasi.
Jangan tampilkan proses berpikir atau tag <think>.
`),
		[]ConversationTurn{{Role: "user", Content: prompt}},
	)
}

func (c *OpenAICompatibleClient) GenerateChat(
	ctx context.Context,
	systemPrompt string,
	messages []ConversationTurn,
) (string, error) {
	startedAt := time.Now()
	operation := "GenerateChat"

	model := ""
	if c != nil {
		model = c.model
	}

	logger.Request(
		"llm.client",
		operation,
		slog.String("model", model),
		slog.Int("message_count", len(messages)),
	)

	if c == nil || c.httpClient == nil || c.apiURL == "" || c.apiKey == "" || c.model == "" {
		err := errors.New("llm client is not configured")
		logger.Failure("llm.client", operation, startedAt, err)
		return "", err
	}

	requestMessages := make([]chatCompletionMessage, 0, len(messages)+1)
	if systemPrompt = strings.TrimSpace(systemPrompt); systemPrompt != "" {
		requestMessages = append(requestMessages, chatCompletionMessage{Role: "system", Content: systemPrompt})
	}

	for _, item := range messages {
		role := strings.ToLower(strings.TrimSpace(item.Role))
		content := strings.TrimSpace(item.Content)
		if content == "" || (role != "user" && role != "assistant") {
			continue
		}
		requestMessages = append(requestMessages, chatCompletionMessage{Role: role, Content: content})
	}

	if len(requestMessages) == 0 || (len(requestMessages) == 1 && requestMessages[0].Role == "system") {
		err := errors.New("llm conversation messages are empty")
		logger.Failure("llm.client", operation, startedAt, err)
		return "", err
	}

	body := chatCompletionRequest{
		Model:               c.model,
		Messages:            requestMessages,
		Temperature:         0.2,
		TopP:                0.8,
		MaxCompletionTokens: 2048,
	}
	if strings.Contains(strings.ToLower(c.model), "qwen") {
		// Qwen digunakan dalam mode nonberpikir agar respons konsultasi singkat,
		// cepat, dan riwayat hanya menyimpan jawaban akhir untuk multi-turn.
		body.ReasoningEffort = "none"
	}

	payload, err := json.Marshal(body)
	if err != nil {
		logger.Failure("llm.client", operation, startedAt, err)
		return "", err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.apiURL+"/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		logger.Failure("llm.client", operation, startedAt, err)
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Failure("llm.client", operation, startedAt, err)
		return "", err
	}
	defer resp.Body.Close()

	var result chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logger.Failure(
			"llm.client",
			operation,
			startedAt,
			err,
			slog.Int("status_code", resp.StatusCode),
		)
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(resp.Status)
		if result.Error != nil && strings.TrimSpace(result.Error.Message) != "" {
			message = strings.TrimSpace(result.Error.Message)
		}
		err := errors.New(message)
		logger.Failure(
			"llm.client",
			operation,
			startedAt,
			err,
			slog.Int("status_code", resp.StatusCode),
		)
		return "", err
	}

	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		err := errors.New("empty llm response")
		logger.Failure(
			"llm.client",
			operation,
			startedAt,
			err,
			slog.Int("status_code", resp.StatusCode),
		)
		return "", err
	}

	content := strings.TrimSpace(result.Choices[0].Message.Content)
	finishReason := result.Choices[0].FinishReason
	logger.Response(
		"llm.client",
		operation,
		startedAt,
		slog.String("model", c.model),
		slog.Int("status_code", resp.StatusCode),
		slog.Int("response_length", len(content)),
		slog.String("finish_reason", finishReason),
	)

	return content, nil
}

func (c *OpenAICompatibleClient) ModelName() string {
	if c == nil {
		return ""
	}
	return c.model
}
