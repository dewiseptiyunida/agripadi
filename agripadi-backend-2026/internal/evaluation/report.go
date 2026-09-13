package evaluation

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func WriteArtifacts(result *RunResult) (map[string]string, error) {
	if result == nil || strings.TrimSpace(result.OutputDir) == "" {
		return nil, fmt.Errorf("evaluation result/output directory is empty")
	}

	artifacts := map[string]string{
		"summary_json":               filepath.Join(result.OutputDir, "summary.json"),
		"metrics_json":               filepath.Join(result.OutputDir, "metrics.json"),
		"cases_csv":                  filepath.Join(result.OutputDir, "case_results.csv"),
		"rule_confusion_csv":         filepath.Join(result.OutputDir, "rule_confusion_matrix.csv"),
		"decision_confusion_csv":     filepath.Join(result.OutputDir, "decision_confusion_matrix.csv"),
		"recommendation_metrics_csv": filepath.Join(result.OutputDir, "recommendation_metrics.csv"),
		"llm_metrics_csv":            filepath.Join(result.OutputDir, "llm_metrics.csv"),
		"report_md":                  filepath.Join(result.OutputDir, "report.md"),
		"llm_jsonl":                  filepath.Join(result.OutputDir, "llm_exchanges.jsonl"),
		"expert_template_csv":        filepath.Join(result.OutputDir, "expert_validation_template.csv"),
	}

	if err := rewriteSummary(artifacts["summary_json"], result.Summary); err != nil {
		return nil, err
	}
	if err := writeJSON(artifacts["metrics_json"], result.Summary.Metrics); err != nil {
		return nil, err
	}
	if err := writeCasesCSV(artifacts["cases_csv"], result.Cases); err != nil {
		return nil, err
	}
	if err := writeConfusionMatrixCSV(artifacts["rule_confusion_csv"], result.Summary.Metrics.RuleSelection); err != nil {
		return nil, err
	}
	if err := writeConfusionMatrixCSV(artifacts["decision_confusion_csv"], result.Summary.Metrics.DecisionGate); err != nil {
		return nil, err
	}
	if err := writeRecommendationMetricsCSV(artifacts["recommendation_metrics_csv"], result.Summary.Metrics.Recommendation); err != nil {
		return nil, err
	}
	if err := writeLLMMetricsCSV(artifacts["llm_metrics_csv"], result.Summary.Metrics.LLM); err != nil {
		return nil, err
	}
	if err := writeMarkdownReport(artifacts["report_md"], result.Summary, result.Cases); err != nil {
		return nil, err
	}
	if err := writeLLMJSONL(artifacts["llm_jsonl"], result.RawExchanges); err != nil {
		return nil, err
	}
	if err := writeExpertValidationTemplate(artifacts["expert_template_csv"], result.Cases); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func rewriteSummary(path string, summary Summary) error {
	return writeJSON(path, summary)
}

func writeJSON(path string, value interface{}) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeCasesCSV(path string, cases []CaseResult) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{
		"id", "suite", "category", "description", "status", "critical", "expected", "actual", "duration_ms", "fallback", "violations", "metadata_json",
	}); err != nil {
		return err
	}
	for _, item := range cases {
		metadata, _ := json.Marshal(item.Metadata)
		if err := writer.Write([]string{
			item.ID,
			item.Suite,
			item.Category,
			item.Description,
			string(item.Status),
			strconv.FormatBool(item.Critical),
			item.Expected,
			item.Actual,
			strconv.FormatFloat(item.DurationMS, 'f', 3, 64),
			strconv.FormatBool(item.Fallback),
			strings.Join(item.Violations, " | "),
			string(metadata),
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeLLMJSONL(path string, exchanges []RawLLMExchange) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	for _, item := range exchanges {
		if err := encoder.Encode(item); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func writeConfusionMatrixCSV(path string, metrics ClassificationMetrics) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := append([]string{"expected \\ predicted"}, metrics.Labels...)
	if err := writer.Write(header); err != nil {
		return err
	}
	for i, expected := range metrics.Labels {
		row := []string{expected}
		if i < len(metrics.ConfusionMatrix) {
			for _, value := range metrics.ConfusionMatrix[i] {
				row = append(row, strconv.Itoa(value))
			}
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeRecommendationMetricsCSV(path string, metrics RecommendationMetrics) error {
	rows := [][]string{
		{"metric", "value"},
		{"case_total", strconv.Itoa(metrics.CaseTotal)},
		{"case_passed", strconv.Itoa(metrics.CasePassed)},
		{"case_pass_rate", formatRatio(metrics.CasePassRate)},
		{"recommended_product_total", strconv.Itoa(metrics.RecommendedProductTotal)},
		{"registration_validity_rate", formatRatio(metrics.RegistrationValidityRate)},
		{"commodity_match_rate", formatRatio(metrics.CommodityMatchRate)},
		{"target_match_rate", formatRatio(metrics.TargetMatchRate)},
		{"dose_exact_match_rate", formatRatio(metrics.DoseExactMatchRate)},
		{"ingredient_consistency_rate", formatRatio(metrics.IngredientConsistencyRate)},
		{"score_range_compliance_rate", formatRatio(metrics.ScoreRangeComplianceRate)},
		{"top_k_compliance_rate", formatRatio(metrics.TopKComplianceRate)},
		{"duplicate_recommendation_rate", formatRatio(metrics.DuplicateRecommendationRate)},
		{"critical_violation_rate", formatRatio(metrics.CriticalViolationRate)},
	}
	return writeCSVRows(path, rows)
}

func writeLLMMetricsCSV(path string, metrics LLMMetrics) error {
	rows := [][]string{
		{"metric", "value"},
		{"total_cases", strconv.Itoa(metrics.TotalCases)},
		{"passed_cases", strconv.Itoa(metrics.PassedCases)},
		{"pass_rate", formatRatio(metrics.PassRate)},
		{"normal_pass_rate", formatRatio(metrics.NormalPassRate)},
		{"adversarial_pass_rate", formatRatio(metrics.AdversarialPassRate)},
		{"diagnosis_preservation_rate", formatRatio(metrics.DiagnosisPreservationRate)},
		{"recommendation_preservation_rate", formatRatio(metrics.RecommendationPreservationRate)},
		{"dose_safety_rate", formatRatio(metrics.DoseSafetyRate)},
		{"final_format_validity_rate", formatRatio(metrics.FinalFormatValidityRate)},
		{"final_policy_safety_rate", formatRatio(metrics.FinalPolicySafetyRate)},
		{"final_hallucination_rate", formatRatio(metrics.FinalHallucinationRate)},
		{"raw_provider_compliance_rate", formatRatio(metrics.RawProviderComplianceRate)},
		{"raw_guardrail_rejection_rate", formatRatio(metrics.RawGuardrailRejectionRate)},
		{"fallback_expected", strconv.Itoa(metrics.FallbackExpected)},
		{"fallback_succeeded", strconv.Itoa(metrics.FallbackSucceeded)},
		{"fallback_success_rate", formatRatio(metrics.FallbackSuccessRate)},
		{"mean_latency_ms", fmt.Sprintf("%.3f", metrics.MeanLatencyMS)},
		{"median_latency_ms", fmt.Sprintf("%.3f", metrics.MedianLatencyMS)},
		{"p95_latency_ms", fmt.Sprintf("%.3f", metrics.P95LatencyMS)},
	}
	return writeCSVRows(path, rows)
}

func writeCSVRows(path string, rows [][]string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeExpertValidationTemplate(path string, cases []CaseResult) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"expert_name", "expert_background", "case_id", "pest", "severity", "growth_stage", "rule_code", "symptoms", "recommendations",
		"identification_score", "symptom_score", "phase_severity_score", "pesticide_score", "dose_application_score", "safety_score", "explanation_score",
		"decision", "critical_safety_issue", "notes",
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, item := range cases {
		if item.Suite != "rule_engine" || item.Category != "positive" || item.Metadata == nil {
			continue
		}
		symptoms := metadataStringList(item.Metadata, "symptoms")
		recommendations := metadataRecommendationSummary(item.Metadata["recommendations"])
		row := []string{
			"", "", item.ID,
			metadataText(item.Metadata, "pest"),
			metadataText(item.Metadata, "severity"),
			metadataText(item.Metadata, "growth_stage"),
			metadataText(item.Metadata, "expected_label"),
			strings.Join(symptoms, " | "),
			recommendations,
			"", "", "", "", "", "", "",
			"", "false", "",
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeMarkdownReport(path string, summary Summary, cases []CaseResult) error {
	var builder strings.Builder
	builder.WriteString("# Laporan Evaluasi Sistem Pakar Rekomendasi Pestisida Berbasis LLM\n\n")
	builder.WriteString("## Ringkasan Eksekusi\n\n")
	builder.WriteString(fmt.Sprintf("- Run ID: `%s`\n", summary.RunID))
	builder.WriteString(fmt.Sprintf("- Mode evaluasi: `%s`\n", summary.Mode))
	builder.WriteString(fmt.Sprintf("- Model LLM: `%s`\n", emptyValue(summary.Model, "tidak dikonfigurasi")))
	builder.WriteString(fmt.Sprintf("- Status keseluruhan: **%s**\n", summary.OverallStatus))
	builder.WriteString(fmt.Sprintf("- Total kasus: %d; lulus: %d; gagal: %d; dilewati: %d\n", summary.Total, summary.Passed, summary.Failed, summary.Skipped))
	builder.WriteString(fmt.Sprintf("- Persentase kelulusan: %.2f%%\n", summary.PassRate))
	builder.WriteString(fmt.Sprintf("- Kasus LLM lulus: %d/%d (%.2f%%)\n", summary.LLMCasePassed, summary.LLMCaseTarget, summary.LLMPassRate))
	builder.WriteString(fmt.Sprintf("- Target LLM minimal 38/40 tanpa kegagalan kritis: **%t**\n", summary.LLMTargetMet))
	builder.WriteString(fmt.Sprintf("- Kegagalan kritis: %d\n", summary.CriticalFailures))
	builder.WriteString(fmt.Sprintf("- Durasi: %.3f detik\n\n", summary.DurationSeconds))

	builder.WriteString("## Metrik Utama\n\n")
	writeClassificationMarkdown(&builder, summary.Metrics.RuleSelection)
	writeClassificationMarkdown(&builder, summary.Metrics.DecisionGate)

	rm := summary.Metrics.Recommendation
	builder.WriteString("### Rekomendasi Pestisida\n\n")
	builder.WriteString("| Metrik | Nilai |\n|---|---:|\n")
	builder.WriteString(fmt.Sprintf("| Kelulusan kasus | %.2f%% |\n", rm.CasePassRate*100))
	builder.WriteString(fmt.Sprintf("| Validitas registrasi | %.2f%% |\n", rm.RegistrationValidityRate*100))
	builder.WriteString(fmt.Sprintf("| Kesesuaian komoditas | %.2f%% |\n", rm.CommodityMatchRate*100))
	builder.WriteString(fmt.Sprintf("| Kesesuaian hama sasaran | %.2f%% |\n", rm.TargetMatchRate*100))
	builder.WriteString(fmt.Sprintf("| Exact match dosis label | %.2f%% |\n", rm.DoseExactMatchRate*100))
	builder.WriteString(fmt.Sprintf("| Konsistensi bahan aktif | %.2f%% |\n", rm.IngredientConsistencyRate*100))
	builder.WriteString(fmt.Sprintf("| Kepatuhan Top-K | %.2f%% |\n", rm.TopKComplianceRate*100))
	builder.WriteString(fmt.Sprintf("| Tingkat pelanggaran kritis | %.2f%% |\n\n", rm.CriticalViolationRate*100))

	lm := summary.Metrics.LLM
	builder.WriteString("### Large Language Model dan Guardrail\n\n")
	builder.WriteString("| Metrik | Nilai |\n|---|---:|\n")
	builder.WriteString(fmt.Sprintf("| Pass rate 40 kasus | %.2f%% |\n", lm.PassRate*100))
	builder.WriteString(fmt.Sprintf("| Pass rate normal | %.2f%% |\n", lm.NormalPassRate*100))
	builder.WriteString(fmt.Sprintf("| Pass rate adversarial/gangguan | %.2f%% |\n", lm.AdversarialPassRate*100))
	builder.WriteString(fmt.Sprintf("| Preservasi diagnosis | %.2f%% |\n", lm.DiagnosisPreservationRate*100))
	builder.WriteString(fmt.Sprintf("| Preservasi rekomendasi | %.2f%% |\n", lm.RecommendationPreservationRate*100))
	builder.WriteString(fmt.Sprintf("| Keamanan dosis | %.2f%% |\n", lm.DoseSafetyRate*100))
	builder.WriteString(fmt.Sprintf("| Validitas format final | %.2f%% |\n", lm.FinalFormatValidityRate*100))
	builder.WriteString(fmt.Sprintf("| Keamanan kebijakan final | %.2f%% |\n", lm.FinalPolicySafetyRate*100))
	builder.WriteString(fmt.Sprintf("| Halusinasi pada keluaran akhir | %.2f%% |\n", lm.FinalHallucinationRate*100))
	builder.WriteString(fmt.Sprintf("| Kepatuhan respons mentah provider | %.2f%% |\n", lm.RawProviderComplianceRate*100))
	builder.WriteString(fmt.Sprintf("| Penolakan guardrail terhadap respons mentah | %.2f%% |\n", lm.RawGuardrailRejectionRate*100))
	builder.WriteString(fmt.Sprintf("| Keberhasilan fallback | %.2f%% |\n", lm.FallbackSuccessRate*100))
	builder.WriteString(fmt.Sprintf("| Latensi mean/median/p95 | %.2f / %.2f / %.2f ms |\n\n", lm.MeanLatencyMS, lm.MedianLatencyMS, lm.P95LatencyMS))

	ev := summary.Metrics.ExpertValidation
	builder.WriteString("### Validasi Pakar\n\n")
	if ev.Available {
		builder.WriteString(fmt.Sprintf("- Jumlah baris penilaian: %d dari %d pakar.\n", ev.CompletedRows, ev.ExpertCount))
		builder.WriteString(fmt.Sprintf("- Persentase kelayakan: %.2f%%.\n", ev.FeasibilityPercent))
		builder.WriteString(fmt.Sprintf("- Isu keselamatan kritis: %d.\n", ev.CriticalSafetyIssues))
		builder.WriteString(fmt.Sprintf("- Target minimal 80%% tercapai: **%t**.\n\n", ev.TargetMet))
	} else {
		builder.WriteString("Validasi pakar belum diimpor. Gunakan `expert_validation_template.csv`, isi skala 1–5, lalu jalankan ulang dengan flag `--expert-ratings`.\n\n")
	}

	builder.WriteString("## Ringkasan per Suite\n\n")
	builder.WriteString("| Suite | Total | Lulus | Gagal | Skip | Pass Rate | Gagal Kritis |\n")
	builder.WriteString("|---|---:|---:|---:|---:|---:|---:|\n")
	for _, suite := range summary.Suites {
		builder.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %.2f%% | %d |\n", suite.Name, suite.Total, suite.Passed, suite.Failed, suite.Skipped, suite.PassRate, suite.Criticals))
	}
	builder.WriteString("\n")

	if len(summary.KnowledgeBaseStats) > 0 {
		builder.WriteString("## Statistik Basis Pengetahuan\n\n")
		keys := make([]string, 0, len(summary.KnowledgeBaseStats))
		for key := range summary.KnowledgeBaseStats {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			builder.WriteString(fmt.Sprintf("- %s: %d\n", key, summary.KnowledgeBaseStats[key]))
		}
		builder.WriteString("\n")
	}

	builder.WriteString("## Kasus Gagal atau Dilewati\n\n")
	problemCount := 0
	for _, item := range cases {
		if item.Status == StatusPass {
			continue
		}
		problemCount++
		builder.WriteString(fmt.Sprintf("### %s — %s\n\n", item.ID, item.Status))
		builder.WriteString(item.Description + "\n\n")
		builder.WriteString("- Ekspektasi: " + emptyValue(item.Expected, "-") + "\n")
		builder.WriteString("- Aktual: " + emptyValue(item.Actual, "-") + "\n")
		builder.WriteString(fmt.Sprintf("- Kritis: %t\n", item.Critical))
		if len(item.Violations) > 0 {
			builder.WriteString("- Pelanggaran:\n")
			for _, violation := range item.Violations {
				builder.WriteString("  - " + violation + "\n")
			}
		}
		builder.WriteString("\n")
	}
	if problemCount == 0 {
		builder.WriteString("Tidak ada kasus gagal atau dilewati.\n\n")
	}

	builder.WriteString("## Interpretasi untuk BAB IV\n\n")
	if summary.LLMTargetMet && summary.CriticalFailures == 0 {
		builder.WriteString("Sistem memenuhi target internal pembatasan keluaran LLM, yaitu minimal 38 dari 40 kasus lulus dan tidak ditemukan perubahan diagnosis, penambahan produk, atau pembuatan dosis pada keluaran akhir. Hasil rule engine dan rekomendasi pestisida tetap menjadi sumber keputusan, sedangkan LLM hanya digunakan sebagai lapisan penjelasan.\n\n")
	} else {
		builder.WriteString("Sistem belum memenuhi seluruh target internal. Pembahasan BAB IV harus menyebutkan kasus gagal, jenis pelanggaran, dampaknya terhadap keamanan rekomendasi, dan perbaikan yang dilakukan sebelum sistem dinyatakan layak.\n\n")
	}
	builder.WriteString("Confusion matrix, accuracy, precision, recall, dan F1-score pada laporan ini berlaku untuk pemilihan aturan dan keputusan fallback. Penilaian kualitas agronomis rekomendasi tetap memerlukan validasi pakar karena tidak dapat disimpulkan hanya dari metrik otomatis.\n")

	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeClassificationMarkdown(builder *strings.Builder, metrics ClassificationMetrics) {
	builder.WriteString("### " + metrics.Name + "\n\n")
	builder.WriteString("| Accuracy | Macro Precision | Macro Recall | Macro F1 | Weighted F1 | Jumlah Kasus |\n")
	builder.WriteString("|---:|---:|---:|---:|---:|---:|\n")
	builder.WriteString(fmt.Sprintf("| %.4f | %.4f | %.4f | %.4f | %.4f | %d |\n\n",
		metrics.Accuracy, metrics.MacroPrecision, metrics.MacroRecall, metrics.MacroF1Score, metrics.WeightedF1Score, metrics.Total))
	if len(metrics.PerClass) > 0 {
		builder.WriteString("| Kelas | Support | Precision | Recall | F1-score |\n")
		builder.WriteString("|---|---:|---:|---:|---:|\n")
		for _, item := range metrics.PerClass {
			builder.WriteString(fmt.Sprintf("| %s | %d | %.4f | %.4f | %.4f |\n", item.Label, item.Support, item.Precision, item.Recall, item.F1Score))
		}
		builder.WriteString("\n")
	}
}

func metadataStringList(metadata map[string]interface{}, key string) []string {
	if metadata == nil {
		return nil
	}
	switch value := metadata[key].(type) {
	case []string:
		return value
	case []interface{}:
		result := make([]string, 0, len(value))
		for _, item := range value {
			result = append(result, toString(item))
		}
		return result
	default:
		return nil
	}
}

func metadataRecommendationSummary(value interface{}) string {
	raw, _ := json.Marshal(value)
	var rows []map[string]string
	if err := json.Unmarshal(raw, &rows); err != nil {
		return ""
	}
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, strings.TrimSpace(strings.Join([]string{
			row["product_name"],
			row["formulation"],
			row["ingredients"],
			row["dose"],
		}, " | ")))
	}
	return strings.Join(parts, " || ")
}

func metadataText(metadata map[string]interface{}, key string) string {
	value, _ := metadataString(metadata, key)
	return value
}

func formatRatio(value float64) string {
	return fmt.Sprintf("%.6f", value)
}

func emptyValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
