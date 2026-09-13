package evaluation

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
)

const fallbackLabel = "FALLBACK"

type ClassMetric struct {
	Label     string  `json:"label"`
	Support   int     `json:"support"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1Score   float64 `json:"f1_score"`
}

type ClassificationMetrics struct {
	Name              string        `json:"name"`
	Labels            []string      `json:"labels"`
	ConfusionMatrix   [][]int       `json:"confusion_matrix"`
	Total             int           `json:"total"`
	Correct           int           `json:"correct"`
	Accuracy          float64       `json:"accuracy"`
	MacroPrecision    float64       `json:"macro_precision"`
	MacroRecall       float64       `json:"macro_recall"`
	MacroF1Score      float64       `json:"macro_f1_score"`
	WeightedPrecision float64       `json:"weighted_precision"`
	WeightedRecall    float64       `json:"weighted_recall"`
	WeightedF1Score   float64       `json:"weighted_f1_score"`
	PerClass          []ClassMetric `json:"per_class"`
	Notes             []string      `json:"notes,omitempty"`
}

type RecommendationMetrics struct {
	CaseTotal                   int      `json:"case_total"`
	CasePassed                  int      `json:"case_passed"`
	CasePassRate                float64  `json:"case_pass_rate"`
	RecommendedProductTotal     int      `json:"recommended_product_total"`
	RegistrationValidityRate    float64  `json:"registration_validity_rate"`
	CommodityMatchRate          float64  `json:"commodity_match_rate"`
	TargetMatchRate             float64  `json:"target_match_rate"`
	DoseExactMatchRate          float64  `json:"dose_exact_match_rate"`
	IngredientConsistencyRate   float64  `json:"ingredient_consistency_rate"`
	ScoreRangeComplianceRate    float64  `json:"score_range_compliance_rate"`
	TopKComplianceRate          float64  `json:"top_k_compliance_rate"`
	DuplicateRecommendationRate float64  `json:"duplicate_recommendation_rate"`
	CriticalViolationRate       float64  `json:"critical_violation_rate"`
	Notes                       []string `json:"notes,omitempty"`
}

type LLMMetrics struct {
	TotalCases                     int      `json:"total_cases"`
	PassedCases                    int      `json:"passed_cases"`
	PassRate                       float64  `json:"pass_rate"`
	NormalCases                    int      `json:"normal_cases"`
	NormalPassed                   int      `json:"normal_passed"`
	NormalPassRate                 float64  `json:"normal_pass_rate"`
	AdversarialCases               int      `json:"adversarial_cases"`
	AdversarialPassed              int      `json:"adversarial_passed"`
	AdversarialPassRate            float64  `json:"adversarial_pass_rate"`
	DiagnosisPreservationRate      float64  `json:"diagnosis_preservation_rate"`
	RecommendationPreservationRate float64  `json:"recommendation_preservation_rate"`
	DoseSafetyRate                 float64  `json:"dose_safety_rate"`
	FinalFormatValidityRate        float64  `json:"final_format_validity_rate"`
	FinalPolicySafetyRate          float64  `json:"final_policy_safety_rate"`
	FinalHallucinationRate         float64  `json:"final_hallucination_rate"`
	RawProviderComplianceRate      float64  `json:"raw_provider_compliance_rate"`
	RawGuardrailRejectionRate      float64  `json:"raw_guardrail_rejection_rate"`
	FallbackExpected               int      `json:"fallback_expected"`
	FallbackSucceeded              int      `json:"fallback_succeeded"`
	FallbackSuccessRate            float64  `json:"fallback_success_rate"`
	MeanLatencyMS                  float64  `json:"mean_latency_ms"`
	MedianLatencyMS                float64  `json:"median_latency_ms"`
	P95LatencyMS                   float64  `json:"p95_latency_ms"`
	Notes                          []string `json:"notes,omitempty"`
}

type ExpertValidationMetrics struct {
	Available            bool               `json:"available"`
	CompletedRows        int                `json:"completed_rows"`
	ExpertCount          int                `json:"expert_count"`
	MeanByAspect         map[string]float64 `json:"mean_by_aspect,omitempty"`
	FeasibilityPercent   float64            `json:"feasibility_percent"`
	Accepted             int                `json:"accepted"`
	AcceptedWithRevision int                `json:"accepted_with_revision"`
	Rejected             int                `json:"rejected"`
	CriticalSafetyIssues int                `json:"critical_safety_issues"`
	TargetMet            bool               `json:"target_met"`
	Notes                []string           `json:"notes,omitempty"`
}

type MetricsReport struct {
	RuleSelection    ClassificationMetrics   `json:"rule_selection"`
	DecisionGate     ClassificationMetrics   `json:"decision_gate"`
	Recommendation   RecommendationMetrics   `json:"recommendation"`
	LLM              LLMMetrics              `json:"llm"`
	ExpertValidation ExpertValidationMetrics `json:"expert_validation"`
	Limitations      []string                `json:"limitations"`
}

type labelPair struct {
	expected string
	actual   string
}

func BuildMetrics(cases []CaseResult, expertMetrics ExpertValidationMetrics) MetricsReport {
	rulePairs := make([]labelPair, 0)
	decisionPairs := make([]labelPair, 0)

	for _, item := range cases {
		if item.Suite != "rule_engine" || item.Metadata == nil {
			continue
		}
		// Confusion matrix pemilihan rule hanya memakai kasus positif. Kasus
		// negatif menguji keputusan DECISION/FALLBACK dan dihitung terpisah pada
		// DecisionGate. Pemisahan ini mencegah kelas FALLBACK yang dominan
		// mengaburkan precision, recall, dan F1 dari 15 rule diagnosis.
		if item.Category == "positive" {
			expected, okExpected := metadataString(item.Metadata, "expected_label")
			actual, okActual := metadataString(item.Metadata, "actual_label")
			if okExpected && okActual && expected != "" && actual != "" {
				rulePairs = append(rulePairs, labelPair{expected: expected, actual: actual})
			}
		}
		expectedDecision, okExpectedDecision := metadataString(item.Metadata, "expected_decision")
		actualDecision, okActualDecision := metadataString(item.Metadata, "actual_decision")
		if okExpectedDecision && okActualDecision && expectedDecision != "" && actualDecision != "" {
			decisionPairs = append(decisionPairs, labelPair{expected: expectedDecision, actual: actualDecision})
		}
	}

	ruleMetrics := classificationMetrics("Pemilihan aturan", rulePairs)
	ruleMetrics.Notes = append(ruleMetrics.Notes,
		"Matriks ini mengukur ketepatan pemilihan 15 rule pada kasus positif yang memiliki bukti lengkap.",
		"Kasus negatif dihitung terpisah pada gerbang DECISION/FALLBACK agar kelas FALLBACK tidak mendominasi metrik per-rule.",
		"Kasus sintetis berbasis basis pengetahuan menguji konsistensi implementasi dan tidak menggantikan validasi pakar lapangan.",
	)
	decisionMetrics := classificationMetrics("Gerbang keputusan vs fallback", decisionPairs)
	decisionMetrics.Notes = append(decisionMetrics.Notes,
		"Kelas DECISION berarti bukti identitas dan kerusakan cukup; FALLBACK berarti sistem menahan diagnosis pasti.",
	)

	return MetricsReport{
		RuleSelection:    ruleMetrics,
		DecisionGate:     decisionMetrics,
		Recommendation:   recommendationMetrics(cases),
		LLM:              llmMetrics(cases),
		ExpertValidation: expertMetrics,
		Limitations: []string{
			"Precision, recall, dan F1 rule engine dihitung dari skenario uji berlabel; validitas klinis/agronomis tetap harus dikonfirmasi pakar.",
			"Precision@K, Recall@K, MRR, atau NDCG rekomendasi tidak dihitung tanpa relevance judgement produk yang disahkan pakar. Sistem melaporkan validitas registrasi, target, dosis, bahan aktif, dan konsistensi ranking sebagai metrik yang dapat diaudit.",
			"Metrik LLM dipisahkan antara kepatuhan respons mentah provider dan keamanan keluaran akhir setelah validator/fallback.",
		},
	}
}

func classificationMetrics(name string, pairs []labelPair) ClassificationMetrics {
	metrics := ClassificationMetrics{Name: name}
	if len(pairs) == 0 {
		metrics.Notes = append(metrics.Notes, "Tidak ada pasangan label yang dapat dihitung.")
		return metrics
	}

	labelSet := make(map[string]struct{})
	for _, pair := range pairs {
		labelSet[pair.expected] = struct{}{}
		labelSet[pair.actual] = struct{}{}
	}
	labels := make([]string, 0, len(labelSet))
	for label := range labelSet {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	index := make(map[string]int, len(labels))
	for i, label := range labels {
		index[label] = i
	}

	matrix := make([][]int, len(labels))
	for i := range matrix {
		matrix[i] = make([]int, len(labels))
	}
	for _, pair := range pairs {
		matrix[index[pair.expected]][index[pair.actual]]++
	}

	metrics.Labels = labels
	metrics.ConfusionMatrix = matrix
	metrics.Total = len(pairs)

	weightedPrecision := 0.0
	weightedRecall := 0.0
	weightedF1 := 0.0
	for i, label := range labels {
		tp := matrix[i][i]
		metrics.Correct += tp
		fn := 0
		fp := 0
		support := 0
		for j := range labels {
			support += matrix[i][j]
			if j != i {
				fn += matrix[i][j]
				fp += matrix[j][i]
			}
		}
		precision := safeDivide(float64(tp), float64(tp+fp))
		recall := safeDivide(float64(tp), float64(tp+fn))
		f1 := harmonicMean(precision, recall)
		metrics.PerClass = append(metrics.PerClass, ClassMetric{
			Label: label, Support: support, Precision: precision, Recall: recall, F1Score: f1,
		})
		metrics.MacroPrecision += precision
		metrics.MacroRecall += recall
		metrics.MacroF1Score += f1
		weightedPrecision += precision * float64(support)
		weightedRecall += recall * float64(support)
		weightedF1 += f1 * float64(support)
	}

	classCount := float64(len(labels))
	metrics.Accuracy = safeDivide(float64(metrics.Correct), float64(metrics.Total))
	metrics.MacroPrecision = safeDivide(metrics.MacroPrecision, classCount)
	metrics.MacroRecall = safeDivide(metrics.MacroRecall, classCount)
	metrics.MacroF1Score = safeDivide(metrics.MacroF1Score, classCount)
	metrics.WeightedPrecision = safeDivide(weightedPrecision, float64(metrics.Total))
	metrics.WeightedRecall = safeDivide(weightedRecall, float64(metrics.Total))
	metrics.WeightedF1Score = safeDivide(weightedF1, float64(metrics.Total))
	return metrics
}

func recommendationMetrics(cases []CaseResult) RecommendationMetrics {
	metrics := RecommendationMetrics{
		Notes: []string{
			"Seluruh rasio produk dihitung pada rekomendasi Top-K yang benar-benar dikembalikan service produksi.",
		},
	}
	registrationValid := 0
	commodityValid := 0
	targetValid := 0
	doseValid := 0
	ingredientValid := 0
	scoreValid := 0
	topKCompliant := 0
	duplicates := 0
	criticalCases := 0

	for _, item := range cases {
		if item.Suite != "pesticide_recommendation" {
			continue
		}
		metrics.CaseTotal++
		if item.Status == StatusPass {
			metrics.CasePassed++
		}
		if item.Critical && item.Status == StatusFail {
			criticalCases++
		}
		if item.Metadata == nil {
			continue
		}
		productCount := metadataInt(item.Metadata, "product_count")
		metrics.RecommendedProductTotal += productCount
		registrationValid += metadataInt(item.Metadata, "registration_valid_count")
		commodityValid += metadataInt(item.Metadata, "commodity_valid_count")
		targetValid += metadataInt(item.Metadata, "target_valid_count")
		doseValid += metadataInt(item.Metadata, "dose_exact_count")
		ingredientValid += metadataInt(item.Metadata, "ingredient_valid_count")
		scoreValid += metadataInt(item.Metadata, "score_valid_count")
		duplicates += metadataInt(item.Metadata, "duplicate_count")
		if metadataBool(item.Metadata, "top_k_compliant") {
			topKCompliant++
		}
	}

	metrics.CasePassRate = safeDivide(float64(metrics.CasePassed), float64(metrics.CaseTotal))
	denominator := float64(metrics.RecommendedProductTotal)
	metrics.RegistrationValidityRate = safeDivide(float64(registrationValid), denominator)
	metrics.CommodityMatchRate = safeDivide(float64(commodityValid), denominator)
	metrics.TargetMatchRate = safeDivide(float64(targetValid), denominator)
	metrics.DoseExactMatchRate = safeDivide(float64(doseValid), denominator)
	metrics.IngredientConsistencyRate = safeDivide(float64(ingredientValid), denominator)
	metrics.ScoreRangeComplianceRate = safeDivide(float64(scoreValid), denominator)
	metrics.TopKComplianceRate = safeDivide(float64(topKCompliant), float64(metrics.CaseTotal))
	metrics.DuplicateRecommendationRate = safeDivide(float64(duplicates), denominator)
	metrics.CriticalViolationRate = safeDivide(float64(criticalCases), float64(metrics.CaseTotal))
	return metrics
}

func llmMetrics(cases []CaseResult) LLMMetrics {
	metrics := LLMMetrics{
		Notes: []string{
			"Provider compliance mengukur respons mentah sebelum fallback; final policy safety mengukur keluaran yang benar-benar diteruskan sistem.",
		},
	}
	latencies := make([]float64, 0, 40)
	diagnosisPreserved := 0
	recommendationPreserved := 0
	doseSafe := 0
	formatValid := 0
	finalPolicySafe := 0
	finalHallucinations := 0
	rawEvaluated := 0
	rawAccepted := 0
	rawRejected := 0

	for _, item := range cases {
		if item.Suite != "llm" || item.Category == "preflight" {
			continue
		}
		metrics.TotalCases++
		if item.Status == StatusPass {
			metrics.PassedCases++
		}
		if item.Category == "normal" {
			metrics.NormalCases++
			if item.Status == StatusPass {
				metrics.NormalPassed++
			}
		} else {
			metrics.AdversarialCases++
			if item.Status == StatusPass {
				metrics.AdversarialPassed++
			}
		}
		if item.DurationMS >= 0 {
			latencies = append(latencies, item.DurationMS)
		}
		if metadataBool(item.Metadata, "diagnosis_preserved") {
			diagnosisPreserved++
		}
		if metadataBool(item.Metadata, "recommendation_preserved") {
			recommendationPreserved++
		}
		if metadataBool(item.Metadata, "dose_safe") {
			doseSafe++
		}
		if metadataBool(item.Metadata, "final_format_valid") {
			formatValid++
		}
		if metadataBool(item.Metadata, "final_policy_safe") {
			finalPolicySafe++
		}
		if metadataBool(item.Metadata, "final_hallucination") {
			finalHallucinations++
		}
		if _, ok := item.Metadata["raw_policy_accepted"]; ok {
			rawEvaluated++
			if metadataBool(item.Metadata, "raw_policy_accepted") {
				rawAccepted++
			} else {
				rawRejected++
			}
		}
		if metadataBool(item.Metadata, "fallback_expected") {
			metrics.FallbackExpected++
			if metadataBool(item.Metadata, "fallback_success") {
				metrics.FallbackSucceeded++
			}
		}
	}

	metrics.PassRate = safeDivide(float64(metrics.PassedCases), float64(metrics.TotalCases))
	metrics.NormalPassRate = safeDivide(float64(metrics.NormalPassed), float64(metrics.NormalCases))
	metrics.AdversarialPassRate = safeDivide(float64(metrics.AdversarialPassed), float64(metrics.AdversarialCases))
	metrics.DiagnosisPreservationRate = safeDivide(float64(diagnosisPreserved), float64(metrics.TotalCases))
	metrics.RecommendationPreservationRate = safeDivide(float64(recommendationPreserved), float64(metrics.TotalCases))
	metrics.DoseSafetyRate = safeDivide(float64(doseSafe), float64(metrics.TotalCases))
	metrics.FinalFormatValidityRate = safeDivide(float64(formatValid), float64(metrics.TotalCases))
	metrics.FinalPolicySafetyRate = safeDivide(float64(finalPolicySafe), float64(metrics.TotalCases))
	metrics.FinalHallucinationRate = safeDivide(float64(finalHallucinations), float64(metrics.TotalCases))
	metrics.RawProviderComplianceRate = safeDivide(float64(rawAccepted), float64(rawEvaluated))
	metrics.RawGuardrailRejectionRate = safeDivide(float64(rawRejected), float64(rawEvaluated))
	metrics.FallbackSuccessRate = safeDivide(float64(metrics.FallbackSucceeded), float64(metrics.FallbackExpected))
	metrics.MeanLatencyMS = mean(latencies)
	metrics.MedianLatencyMS = percentile(latencies, 0.50)
	metrics.P95LatencyMS = percentile(latencies, 0.95)
	return metrics
}

func metadataString(metadata map[string]interface{}, key string) (string, bool) {
	if metadata == nil {
		return "", false
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return "", false
	}
	result := strings.TrimSpace(toString(value))
	return result, result != ""
}

func metadataInt(metadata map[string]interface{}, key string) int {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func metadataBool(metadata map[string]interface{}, key string) bool {
	if metadata == nil {
		return false
	}
	value, ok := metadata[key]
	if !ok {
		return false
	}
	result, ok := value.(bool)
	return ok && result
}

func toString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		raw, _ := json.Marshal(typed)
		return strings.Trim(string(raw), "\"")
	}
}

func safeDivide(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

func harmonicMean(a, b float64) float64 {
	if a+b == 0 {
		return 0
	}
	return 2 * a * b / (a + b)
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func percentile(values []float64, quantile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	if quantile <= 0 {
		return copyValues[0]
	}
	if quantile >= 1 {
		return copyValues[len(copyValues)-1]
	}
	position := quantile * float64(len(copyValues)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return copyValues[lower]
	}
	weight := position - float64(lower)
	return copyValues[lower]*(1-weight) + copyValues[upper]*weight
}
