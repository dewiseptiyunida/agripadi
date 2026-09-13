package evaluation

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

var expertValidationAspects = []string{
	"identification_score",
	"symptom_score",
	"phase_severity_score",
	"pesticide_score",
	"dose_application_score",
	"safety_score",
	"explanation_score",
}

func LoadExpertValidationMetrics(path string) (ExpertValidationMetrics, error) {
	metrics := ExpertValidationMetrics{
		Available:    false,
		MeanByAspect: map[string]float64{},
	}
	path = strings.TrimSpace(path)
	if path == "" {
		metrics.Notes = append(metrics.Notes, "Belum ada berkas penilaian pakar. Isi expert_validation_template.csv lalu jalankan ulang dengan --expert-ratings.")
		return metrics, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return metrics, fmt.Errorf("open expert ratings %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return metrics, fmt.Errorf("read expert ratings header: %w", err)
	}
	index := make(map[string]int, len(header))
	for i, column := range header {
		index[strings.ToLower(strings.TrimSpace(column))] = i
	}
	for _, required := range append([]string{"expert_name", "case_id", "decision", "critical_safety_issue"}, expertValidationAspects...) {
		if _, exists := index[required]; !exists {
			return metrics, fmt.Errorf("expert ratings missing required column %q", required)
		}
	}

	experts := make(map[string]struct{})
	sums := make(map[string]float64, len(expertValidationAspects))
	totalScore := 0.0
	maxScore := 0.0

	for rowNumber := 2; ; rowNumber++ {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return metrics, fmt.Errorf("read expert ratings row %d: %w", rowNumber, readErr)
		}
		caseID := valueAt(record, index["case_id"])
		expertName := valueAt(record, index["expert_name"])
		if caseID == "" && expertName == "" {
			continue
		}
		if caseID == "" || expertName == "" {
			return metrics, fmt.Errorf("expert ratings row %d requires expert_name and case_id", rowNumber)
		}

		rowScore := 0.0
		for _, aspect := range expertValidationAspects {
			value := valueAt(record, index[aspect])
			score, parseErr := strconv.Atoi(value)
			if parseErr != nil || score < 1 || score > 5 {
				return metrics, fmt.Errorf("expert ratings row %d column %s must be integer 1-5", rowNumber, aspect)
			}
			sums[aspect] += float64(score)
			rowScore += float64(score)
		}

		decision := normalizeDecision(valueAt(record, index["decision"]))
		switch decision {
		case "accepted":
			metrics.Accepted++
		case "accepted_with_revision":
			metrics.AcceptedWithRevision++
		case "rejected":
			metrics.Rejected++
		default:
			return metrics, fmt.Errorf("expert ratings row %d has invalid decision %q", rowNumber, valueAt(record, index["decision"]))
		}
		if parseCSVBool(valueAt(record, index["critical_safety_issue"])) {
			metrics.CriticalSafetyIssues++
		}

		experts[strings.ToLower(expertName)] = struct{}{}
		metrics.CompletedRows++
		totalScore += rowScore
		maxScore += float64(len(expertValidationAspects) * 5)
	}

	metrics.Available = metrics.CompletedRows > 0
	metrics.ExpertCount = len(experts)
	if metrics.CompletedRows == 0 {
		metrics.Notes = append(metrics.Notes, "Berkas penilaian pakar tidak memiliki baris yang terisi.")
		return metrics, nil
	}
	for _, aspect := range expertValidationAspects {
		metrics.MeanByAspect[aspect] = sums[aspect] / float64(metrics.CompletedRows)
	}
	metrics.FeasibilityPercent = safeDivide(totalScore, maxScore) * 100
	metrics.TargetMet = metrics.FeasibilityPercent >= 80 && metrics.CriticalSafetyIssues == 0 && metrics.Rejected == 0
	if !metrics.TargetMet {
		metrics.Notes = append(metrics.Notes, "Target validasi pakar belum tercapai: kelayakan minimal 80%, tidak ada penolakan, dan tidak ada isu keselamatan kritis.")
	}
	return metrics, nil
}

func normalizeDecision(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "_", "-", "_", "/", "_")
	value = replacer.Replace(value)
	switch value {
	case "diterima", "accept", "accepted":
		return "accepted"
	case "diterima_dengan_perbaikan", "accepted_with_revision", "accept_with_revision":
		return "accepted_with_revision"
	case "ditolak", "reject", "rejected":
		return "rejected"
	default:
		return value
	}
}

func parseCSVBool(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "1", "true", "ya", "yes", "y":
		return true
	default:
		return false
	}
}

func valueAt(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}
