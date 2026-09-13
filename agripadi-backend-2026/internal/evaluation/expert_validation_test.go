package evaluation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExpertValidationMetrics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ratings.csv")
	content := "expert_name,case_id,decision,critical_safety_issue,identification_score,symptom_score,phase_severity_score,pesticide_score,dose_application_score,safety_score,explanation_score\n" +
		"Pakar A,CASE-1,diterima,false,5,5,4,4,5,5,4\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	metrics, err := LoadExpertValidationMetrics(path)
	if err != nil {
		t.Fatal(err)
	}
	if !metrics.Available || metrics.CompletedRows != 1 || metrics.ExpertCount != 1 {
		t.Fatalf("unexpected metrics: %#v", metrics)
	}
	if !metrics.TargetMet {
		t.Fatalf("expected target met, got %#v", metrics)
	}
}
