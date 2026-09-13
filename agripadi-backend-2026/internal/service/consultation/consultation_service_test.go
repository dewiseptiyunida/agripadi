package consultation

import (
	"context"
	"strings"
	"testing"
)

type captureMultiTurnClient struct {
	systemPrompt string
	messages     []ConversationTurn
}

func (c *captureMultiTurnClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "jawaban satu putaran", nil
}

func (c *captureMultiTurnClient) GenerateChat(ctx context.Context, systemPrompt string, messages []ConversationTurn) (string, error) {
	c.systemPrompt = systemPrompt
	c.messages = append([]ConversationTurn(nil), messages...)
	return "Gunakan dosis yang tercantum pada hasil sebelumnya dan tetap ikuti label resmi produk.", nil
}

func TestProcessMessageWithHistorySendsMultiTurnContext(t *testing.T) {
	client := &captureMultiTurnClient{}
	service := NewConsultationService(client)

	response, err := service.ProcessMessageWithHistory(
		context.Background(),
		"Kalau yang tadi, kapan dipakai?",
		[]ConversationTurn{
			{Role: "user", Content: "Tanaman saya terkena wereng batang cokelat."},
			{Role: "assistant", Content: "Hasil pemeriksaan menunjukkan wereng batang cokelat tingkat sedang pada fase vegetatif."},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response == nil || !strings.Contains(response.Message, "hasil sebelumnya") {
		t.Fatalf("unexpected response: %#v", response)
	}
	if len(client.messages) != 3 {
		t.Fatalf("expected 3 messages including current turn, got %d", len(client.messages))
	}
	if client.messages[1].Role != "assistant" || !strings.Contains(client.messages[1].Content, "wereng batang cokelat") {
		t.Fatalf("assistant context was not preserved: %#v", client.messages)
	}
	if !strings.Contains(client.systemPrompt, "riwayat percakapan") {
		t.Fatalf("system prompt must instruct the model to use history")
	}
}

func TestPrepareConversationHistoryRemovesCurrentDuplicateAndLimitsTurns(t *testing.T) {
	history := make([]ConversationTurn, 0, 20)
	for index := 0; index < 18; index++ {
		role := "user"
		if index%2 == 1 {
			role = "assistant"
		}
		history = append(history, ConversationTurn{Role: role, Content: "pesan riwayat"})
	}
	history = append(history, ConversationTurn{Role: "user", Content: "pertanyaan terbaru"})

	prepared := PrepareConversationHistory(history, "pertanyaan terbaru")
	if len(prepared) > MaxConsultationContextTurns {
		t.Fatalf("history exceeds turn limit: %d", len(prepared))
	}
	for _, item := range prepared {
		if item.Role == "user" && item.Content == "pertanyaan terbaru" {
			t.Fatalf("current message must not be duplicated in history")
		}
	}
}
