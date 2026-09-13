package consultation

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gustian305/backend/internal/dto"
	"github.com/gustian305/backend/logger"
)

type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// MultiTurnLLMClient digunakan ketika penyedia LLM mendukung daftar pesan
// percakapan. Riwayat dikirim sebagai peran user/assistant sehingga pertanyaan
// lanjutan dapat dipahami tanpa meminta petani mengulang konteks dari awal.
type MultiTurnLLMClient interface {
	GenerateChat(ctx context.Context, systemPrompt string, messages []ConversationTurn) (string, error)
}

type ConversationTurn struct {
	Role    string
	Content string
}

type ConsultationService struct {
	llmClient LLMClient
}

func NewConsultationService(
	llmClient LLMClient,
) *ConsultationService {
	return &ConsultationService{
		llmClient: llmClient,
	}
}

const (
	MaxConsultationMessageLength = 1000
	DefaultLLMTimeout            = 30 * time.Second
	MaxConsultationContextTurns  = 14
	MaxConsultationContextChars  = 12000
)

func ValidateConsultationMessage(message string) error {

	message = strings.TrimSpace(message)

	if message == "" {
		return errors.New(
			"message cannot be empty",
		)
	}

	if len(message) > MaxConsultationMessageLength {

		return errors.New(
			"message too long",
		)
	}

	return nil
}

func (s *ConsultationService) ProcessMessage(ctx context.Context, message string) (*dto.OrchestratorResponse, error) {
	return s.ProcessMessageWithHistory(ctx, message, nil)
}

func (s *ConsultationService) ProcessMessageWithHistory(
	ctx context.Context,
	message string,
	history []ConversationTurn,
) (*dto.OrchestratorResponse, error) {
	startedAt := time.Now()
	operation := "ProcessMessageWithHistory"

	message = strings.TrimSpace(message)
	suggestImage := ShouldSuggestImage(message)
	logger.Request(
		"app.consultation",
		operation,
		slog.Int("message_length", len(message)),
		slog.Int("history_turn_count", len(history)),
		slog.Bool("suggest_image", suggestImage),
	)

	if err := ValidateConsultationMessage(message); err != nil {
		logger.Failure("app.consultation", operation, startedAt, err)
		return nil, err
	}

	preparedHistory := PrepareConversationHistory(history, message)

	if s.llmClient == nil {
		response := BuildLocalConsultationResponse(message)
		result := s.BuildChatResponse(response, suggestImage)
		logger.Response(
			"app.consultation",
			operation,
			startedAt,
			slog.String("source", "local_fallback"),
			slog.Int("response_length", len(result.Message)),
			slog.Int("action_count", len(result.Actions)),
		)
		return result, nil
	}

	llmCtx, cancel := withDefaultTimeout(ctx, DefaultLLMTimeout)
	defer cancel()

	var response string
	var err error

	if multiTurnClient, ok := s.llmClient.(MultiTurnLLMClient); ok {
		messages := append(
			append(make([]ConversationTurn, 0, len(preparedHistory)+1), preparedHistory...),
			ConversationTurn{Role: "user", Content: message},
		)
		response, err = multiTurnClient.GenerateChat(
			llmCtx,
			BuildConsultationSystemPrompt(),
			messages,
		)
	} else {
		response, err = s.llmClient.Generate(
			llmCtx,
			BuildConsultationPromptWithHistory(preparedHistory, message),
		)
	}

	if err != nil {
		logger.Failure(
			"app.consultation",
			"llm.Generate",
			startedAt,
			err,
			slog.String("fallback", "local_response"),
		)
		response = BuildLocalConsultationResponse(message)
	} else {
		response = SanitizeConsultationResponse(response)
	}

	if response == "" {
		response = BuildLocalConsultationResponse(message)
	}

	if suggestImage && !strings.Contains(strings.ToLower(response), "upload gambar") {
		response += "\n\n" + BuildImageSuggestion()
	}

	result := s.BuildChatResponse(response, suggestImage)
	logger.Response(
		"app.consultation",
		operation,
		startedAt,
		slog.Int("history_turn_count", len(preparedHistory)),
		slog.Int("response_length", len(result.Message)),
		slog.Int("action_count", len(result.Actions)),
	)

	return result, nil
}

func (s *ConsultationService) BuildChatResponse(message string, suggestImage bool) *dto.OrchestratorResponse {

	actions := make(
		[]dto.ChatAction,
		0,
	)

	if suggestImage {

		actions = append(
			actions,
			dto.ChatAction{
				Type: dto.ActionUploadImage,

				Label: "Upload Gambar Hama",

				Value: "upload_image",
			},
		)
	}

	return &dto.OrchestratorResponse{
		SessionID: uuid.New(),

		Mode: dto.ConversationModeConsultation,

		State: dto.DiagnoseFlowStateIdle,

		Message: message,

		Actions: actions,

		CreatedAt: time.Now(),
	}
}

func BuildConsultationSystemPrompt() string {
	return strings.TrimSpace(`
Anda adalah pendamping percakapan AgriPadi untuk petani padi.

Gunakan riwayat percakapan untuk memahami kata rujukan dan pertanyaan lanjutan, misalnya "yang tadi", "kalau sudah berat", "berapa dosisnya", atau "kapan dipakai". Jangan meminta pengguna mengulang informasi yang sudah ada pada percakapan. Jika konteks sebelumnya belum cukup, ajukan satu pertanyaan klarifikasi yang paling penting.

Gaya bicara:
- hangat, natural, dan ramah seperti penyuluh yang sedang berbicara dengan petani
- gunakan bahasa Indonesia sederhana dan kalimat pendek
- hindari istilah teknis pemrograman, nama komponen sistem, dan bahasa laporan
- maksimal 5 paragraf pendek atau poin singkat

Aturan keselamatan dan ruang lingkup:
- jangan mengubah hasil diagnosis, jenis hama, tingkat serangan, fase tanaman, produk, bahan aktif, dosis, atau waktu aplikasi yang sudah ditetapkan sistem
- saat menjelaskan hasil diagnosis sebelumnya, gunakan data yang benar-benar ada dalam riwayat percakapan
- jangan membuat dosis, campuran, interval, atau klaim pestisida baru
- jika petani bertanya tentang dosis atau produk, arahkan pada nilai yang tercatat di hasil sebelumnya dan tetap ingatkan untuk mengikuti label resmi
- jangan membuat diagnosis pasti tanpa foto hama dan gejala yang cukup
- bila pertanyaan mengarah pada serangan hama tetapi belum ada pemeriksaan foto, sarankan mengunggah gambar hama yang terlihat
- berikan langkah praktis yang aman dan utamakan Pengendalian Hama Terpadu
- jangan menampilkan proses berpikir, reasoning internal, atau tag <think>
- keluarkan hanya jawaban akhir untuk pengguna
`)
}

func BuildConsultationPrompt(message string) string {
	return BuildConsultationPromptWithHistory(nil, message)
}

func BuildConsultationPromptWithHistory(history []ConversationTurn, message string) string {
	var builder strings.Builder
	builder.WriteString(BuildConsultationSystemPrompt())
	builder.WriteString("\n\nRIWAYAT PERCAKAPAN:\n")

	if len(history) == 0 {
		builder.WriteString("Belum ada riwayat sebelumnya.\n")
	} else {
		for _, turn := range history {
			role := "Petani"
			if strings.EqualFold(turn.Role, "assistant") {
				role = "AgriPadi"
			}
			builder.WriteString(role + ": " + strings.TrimSpace(turn.Content) + "\n")
		}
	}

	builder.WriteString("\nPERTANYAAN TERBARU PETANI:\n")
	builder.WriteString(strings.TrimSpace(message))
	return builder.String()
}

func PrepareConversationHistory(history []ConversationTurn, currentMessage string) []ConversationTurn {
	currentMessage = strings.TrimSpace(currentMessage)
	cleaned := make([]ConversationTurn, 0, len(history))

	for _, item := range history {
		role := strings.ToLower(strings.TrimSpace(item.Role))
		content := strings.TrimSpace(item.Content)
		if content == "" || (role != "user" && role != "assistant") {
			continue
		}
		if len([]rune(content)) > 2200 {
			runes := []rune(content)
			content = strings.TrimSpace(string(runes[:2200])) + "…"
		}
		cleaned = append(cleaned, ConversationTurn{Role: role, Content: content})
	}

	// Pesan pengguna terbaru sudah diteruskan terpisah. Hapus duplikasi terakhir
	// jika riwayat diambil sesudah pesan tersebut disimpan ke database.
	for index := len(cleaned) - 1; index >= 0; index-- {
		if cleaned[index].Role == "user" && cleaned[index].Content == currentMessage {
			cleaned = append(cleaned[:index], cleaned[index+1:]...)
			break
		}
	}

	if len(cleaned) > MaxConsultationContextTurns {
		cleaned = cleaned[len(cleaned)-MaxConsultationContextTurns:]
	}

	totalChars := 0
	start := len(cleaned)
	for index := len(cleaned) - 1; index >= 0; index-- {
		length := len([]rune(cleaned[index].Content))
		if totalChars+length > MaxConsultationContextChars && start < len(cleaned) {
			break
		}
		totalChars += length
		start = index
	}

	if start > 0 && start < len(cleaned) {
		cleaned = cleaned[start:]
	}

	return cleaned
}

func BuildFallbackResponse() string {

	return strings.TrimSpace(`
Saya bisa bantu konsultasi umum tentang budidaya padi.

Untuk dugaan serangan hama, upload gambar hama yang terlihat agar sistem diagnosis bisa mengklasifikasikan hama dan memberi rekomendasi yang lebih tepat.
`)
}

func BuildLocalConsultationResponse(message string) string {

	normalized := strings.ToLower(
		strings.TrimSpace(message),
	)

	switch {
	case isGreeting(normalized):
		return "Halo, siap bantu ya. Anda bisa cerita soal kondisi padi, pemupukan, air sawah, fase pertumbuhan, atau upload gambar hama kalau ingin dibantu klasifikasi hama."

	case strings.Contains(normalized, "padi"):
		return "Padi memang perlu dipantau rutin, terutama air, warna daun, pertumbuhan anakan, dan kondisi malai. Kalau Anda mau, ceritakan umur tanaman, fase pertumbuhan, kondisi lahan, dan gejala yang terlihat supaya saya bisa bantu lebih tepat. Kalau ada dugaan serangan hama, upload gambar hama yang terlihat juga boleh."

	case ShouldSuggestImage(normalized):
		return "Dari cerita Anda, kemungkinan ada kaitannya dengan serangan hama padi. Supaya lebih aman dan tepat, coba upload gambar hama yang terlihat. Nanti sistem akan bantu lanjutkan dengan pertanyaan gejala dan tingkat keparahan."

	default:
		return "Saya bisa bantu konsultasi umum seputar padi. Coba jelaskan kondisi tanaman, umur atau fase pertumbuhan, kondisi air, warna daun, dan masalah yang sedang Anda lihat."
	}
}

func isGreeting(message string) bool {

	greetings := []string{
		"halo",
		"hai",
		"hi",
		"hello",
		"pagi",
		"siang",
		"sore",
		"malam",
		"assalamualaikum",
	}

	for _, greeting := range greetings {
		if message == greeting ||
			strings.HasPrefix(message, greeting+" ") {

			return true
		}
	}

	return false
}

func SanitizeConsultationResponse(text string) string {

	text = strings.TrimSpace(text)

	text = removeThinkingBlocks(text)

	text = strings.TrimSpace(text)

	text = strings.ReplaceAll(
		text,
		"```",
		"",
	)

	text = strings.ReplaceAll(
		text,
		"**",
		"",
	)

	replacements := map[string]string{
		"upload gambar tanaman padi yang menunjukkan gejala serangan hama atau penyakit": "upload gambar hama yang terlihat pada tanaman padi",
		"upload gambar tanaman agar sistem diagnosis bisa memeriksa label hama":          "upload gambar hama agar sistem diagnosis bisa mengklasifikasikan hama",
		"upload gambar tanaman atau hama yang terlihat":                                  "upload gambar hama yang terlihat",
		"gambar tanaman padi yang menunjukkan gejala serangan hama atau penyakit":        "gambar hama yang terlihat pada tanaman padi",
		"gambar tanaman agar sistem diagnosis":                                           "gambar hama agar sistem diagnosis",
	}

	for oldValue, newValue := range replacements {
		text = strings.ReplaceAll(text, oldValue, newValue)
	}

	text = strings.ReplaceAll(
		text,
		"\r\n",
		"\n",
	)

	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(
			text,
			"\n\n\n",
			"\n\n",
		)
	}

	return strings.TrimSpace(text)
}

func IsDiagnoseIntent(message string) bool {

	message = strings.ToLower(
		strings.TrimSpace(message),
	)

	keywords := []string{
		"hama",
		"wereng",
		"ulat",
		"serangga",
		"bercak",
		"daun kuning",
		"menguning",
		"layu",
		"busuk",
		"rusak",
		"mati",
		"kering",
		"jamur",
		"berlubang",
		"berwarna coklat",
		"berwarna hitam",
		"batang patah",
		"tanaman roboh",
	}

	for _, keyword := range keywords {

		if strings.Contains(
			message,
			keyword,
		) {
			return true
		}
	}

	return false
}

func ShouldSuggestImage(message string) bool {

	return IsDiagnoseIntent(message)
}

func BuildImageSuggestion() string {

	return `
Kalau ingin hasil diagnosis yang lebih akurat, silakan upload gambar hama yang terlihat pada tanaman padi agar sistem bisa membantu klasifikasi hama.
`
}

func withDefaultTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {

	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(
		ctx,
		timeout,
	)
}

func removeThinkingBlocks(text string) string {

	for {

		start := strings.Index(text, "<think>")
		end := strings.Index(text, "</think>")

		if start < 0 ||
			end < 0 ||
			end <= start {

			break
		}

		text =
			text[:start] +
				text[end+len("</think>"):]
	}

	// fallback
	text = strings.ReplaceAll(text, "<think>", "")
	text = strings.ReplaceAll(text, "</think>", "")
	text = strings.ReplaceAll(text, "<think/>", "")
	text = strings.ReplaceAll(text, "<think />", "")

	return strings.TrimSpace(text)
}
