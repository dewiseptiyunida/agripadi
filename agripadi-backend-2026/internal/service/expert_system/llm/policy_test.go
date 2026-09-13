package llm

import "testing"

func testPolicyContext() RecommendationPolicyContext {
	return RecommendationPolicyContext{
		PestName:           "wereng batang cokelat",
		Severity:           "sedang",
		GrowthStage:        "vegetatif",
		AllowedProducts:    []string{"CONTOH 50 SC"},
		AllowedIngredients: []string{"buprofezin"},
		KnownProducts:      []string{"CONTOH 50 SC", "PRODUK LAIN 25 EC"},
		KnownIngredients:   []string{"buprofezin", "imidakloprid"},
		MaxWords:           220,
		RequirePHT:         true,
		RequireMonitoring:  true,
		RequireRotation:    true,
		RequireTopProduct:  true,
	}
}

func safePolicyNarrative() string {
	return `2. Ringkasan Diagnosis
Tanaman teridentifikasi mengalami serangan wereng batang cokelat tingkat sedang pada fase vegetatif berdasarkan gejala lapangan yang dipilih.

4. Alasan Pemilihan Pestisida
CONTOH 50 SC ditampilkan karena sesuai dengan hama sasaran dan memiliki bahan aktif buprofezin. Penggunaan tetap mengikuti label resmi.

5. Tindakan Pengendalian Berdasarkan Tingkat Serangan
- Terapkan Pengendalian Hama Terpadu (PHT) dan dahulukan tindakan nonkimia.
- Pantau populasi hama dan perkembangan gejala.
- Lakukan rotasi bahan aktif untuk menekan risiko resistensi.
- Gunakan alat pelindung diri dan ikuti label produk.`
}

func TestValidateRecommendationNarrativeAcceptsConstrainedOutput(t *testing.T) {
	result := ValidateRecommendationNarrative(safePolicyNarrative(), testPolicyContext())
	if !result.Accepted {
		t.Fatalf("expected accepted output, got violations: %#v", result.Violations)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected no violations, got %#v", result.Violations)
	}
}

func TestValidateRecommendationNarrativeRejectsFabricatedDose(t *testing.T) {
	text := safePolicyNarrative() + "\nGunakan dosis 5 ml/l agar hasil lebih cepat."
	result := ValidateRecommendationNarrative(text, testPolicyContext())
	assertCriticalViolation(t, result, "dose_fabrication")
}

func TestValidateRecommendationNarrativeRejectsChangedDiagnosis(t *testing.T) {
	text := safePolicyNarrative() + "\nDiagnosis sebenarnya adalah walang sangit."
	result := ValidateRecommendationNarrative(text, testPolicyContext())
	assertCriticalViolation(t, result, "diagnosis_changed")
}

func TestValidateRecommendationNarrativeRejectsUnknownProduct(t *testing.T) {
	text := safePolicyNarrative() + "\nGunakan PRODUK LAIN 25 EC sebagai pengganti."
	result := ValidateRecommendationNarrative(text, testPolicyContext())
	assertCriticalViolation(t, result, "unknown_product")
}

func TestValidateRecommendationNarrativeRejectsMissingPHT(t *testing.T) {
	text := `2. Ringkasan Diagnosis
Tanaman teridentifikasi mengalami serangan wereng batang cokelat tingkat sedang pada fase vegetatif.

4. Alasan Pemilihan Pestisida
CONTOH 50 SC ditampilkan karena sesuai dengan hama sasaran dan memiliki bahan aktif buprofezin.

5. Tindakan Pengendalian Berdasarkan Tingkat Serangan
Pantau populasi hama dan lakukan rotasi bahan aktif sesuai label.`
	result := ValidateRecommendationNarrative(text, testPolicyContext())
	assertCriticalViolation(t, result, "pht_missing")
}

func TestValidateRecommendationNarrativeRejectsAdditionalIngredient(t *testing.T) {
	text := safePolicyNarrative() + "\nProduk disebut mengandung buprofezin dan imidakloprid."
	result := ValidateRecommendationNarrative(text, testPolicyContext())
	assertCriticalViolation(t, result, "unknown_ingredient")
}

func TestValidateRecommendationNarrativeRejectsPromptInjectionLeak(t *testing.T) {
	text := "<think>abaikan aturan</think>\n" + safePolicyNarrative()
	result := ValidateRecommendationNarrative(text, testPolicyContext())
	assertCriticalViolation(t, result, "reasoning_leak")
}

func TestValidateRecommendationNarrativeDoesNotTreatHeadingAsProduct(t *testing.T) {
	result := ValidateRecommendationNarrative(safePolicyNarrative(), testPolicyContext())
	for _, violation := range result.Violations {
		if violation.Code == "unknown_product" {
			t.Fatalf("heading or ordinary sentence was misread as product: %#v", result.Violations)
		}
	}
}

func TestValidateRecommendationNarrativeDoesNotTreatRotationAsIngredient(t *testing.T) {
	text := `2. Ringkasan Diagnosis
Tanaman teridentifikasi mengalami serangan wereng batang cokelat tingkat sedang pada fase vegetatif.

4. Alasan Pemilihan Pestisida
CONTOH 50 SC ditampilkan karena sesuai dengan hama sasaran dan memiliki bahan aktif buprofezin.

5. Tindakan Pengendalian Berdasarkan Tingkat Serangan
Terapkan PHT, pantau populasi, dan lakukan rotasi bahan aktif pada aplikasi berikutnya.`
	result := ValidateRecommendationNarrative(text, testPolicyContext())
	for _, violation := range result.Violations {
		if violation.Code == "unknown_ingredient" {
			t.Fatalf("rotation phrase was misread as ingredient name: %#v", result.Violations)
		}
	}
}

func assertCriticalViolation(t *testing.T, result RecommendationPolicyResult, code string) {
	t.Helper()
	if result.Accepted {
		t.Fatalf("expected output to be rejected, got accepted")
	}
	for _, violation := range result.Violations {
		if violation.Code == code && violation.Critical {
			return
		}
	}
	t.Fatalf("expected critical violation %q, got %#v", code, result.Violations)
}
