package expertSystem

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gustian305/backend/internal/domain"
)

func TestAdjustDosageBySeverityKeepsOfficialLabelRange(t *testing.T) {
	minDose := 0.75
	maxDose := 1.5
	dosage := domain.PesticideDosage{
		PestID:   uuid.New(),
		DoseRaw:  "0.75-1.5",
		MinDose:  &minDose,
		MaxDose:  &maxDose,
		DoseUnit: "ml/l",
	}

	for _, severity := range []string{"low", "medium", "high"} {
		got := adjustDosageBySeverity(dosage, severity)
		if got.DoseRaw != "0.75-1.5" {
			t.Fatalf("severity %q must not change official label dose, got %q", severity, got.DoseRaw)
		}
	}
}

func TestFormatRecommendationMessageUsesRequestedMobileSections(t *testing.T) {
	message := FormatRecommendationMessage(
		&DiagnosisRecommendationResult{
			Pest: &domain.Pest{
				Name: "wereng batang cokelat",
			},
			DetectionConfidence: 0.95,
			RuleConfidence:      0.86,
			Severity:            "berat",
			GrowthStage:         "vegetatif",
			LLMSummary:          "Tanaman kemungkinan kuat terserang wereng batang cokelat.",
		},
	)

	for _, expected := range []string{
		"Hasil pemeriksaan:",
		"Tingkat kecocokan:",
		"Kondisi serangan:",
		"Fase padi:",
		"Penjelasan sederhana:",
		"Pilihan bahan aktif:",
		"Waktu penggunaan:",
		"Cara penggunaan:",
		"Langkah berikutnya:",
		"Catatan keamanan:",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("expected message to contain %q, got:\n%s", expected, message)
		}
	}

	if !strings.Contains(message, "Tanaman kemungkinan kuat terserang wereng batang cokelat.") {
		t.Fatalf("expected LLM farmer summary to be shown, got:\n%s", message)
	}
}

func TestFormatDiagnosisConfidenceDoesNotUseCNNWhenRuleFallsBack(t *testing.T) {
	result := &DiagnosisRecommendationResult{
		DetectionConfidence: 0.99,
		RuleConfidence:      0,
		RuleResult:          nil,
	}

	got := formatDiagnosisConfidenceForUser(result)
	if got == "Kuat" || got == "Cukup" {
		t.Fatalf("fallback result must not be shown as strong rule confidence, got %q", got)
	}
	if got != "Perlu verifikasi gejala" {
		t.Fatalf("unexpected fallback confidence label: %q", got)
	}
}

func TestSafeLLMSummaryAcceptsLarvaPenggerekAlias(t *testing.T) {
	result := &DiagnosisRecommendationResult{
		Pest:       &domain.Pest{Name: "larva penggerek batang padi"},
		LLMSummary: "Larva penggerek batang merusak bagian dalam batang sehingga tanaman perlu segera dipantau.",
	}

	got := safeLLMSummaryForFarmer(result)
	if got == "" {
		t.Fatal("current penggerek batang alias must not be rejected as another pest")
	}
}

func TestSanitizeLLMNarrativeRejectsInventedDoseAndOtherPest(t *testing.T) {
	result := &DiagnosisRecommendationResult{
		Pest: &domain.Pest{Name: "walang sangit"},
	}

	if got := sanitizeLLMNarrativeForResult(result, "Gunakan 2 ml/l agar hasil lebih cepat."); got != "" {
		t.Fatalf("dose-bearing LLM narrative must be rejected, got %q", got)
	}
	if got := sanitizeLLMNarrativeForResult(result, "Kemungkinan lain adalah wereng batang cokelat."); got != "" {
		t.Fatalf("other-pest narrative must be rejected, got %q", got)
	}
	if got := sanitizeLLMNarrativeForResult(result, "Walang sangit mengisap bulir yang sedang berisi."); got == "" {
		t.Fatal("valid farmer narrative should be preserved")
	}
}
