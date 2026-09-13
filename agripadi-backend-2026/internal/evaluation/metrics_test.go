package evaluation

import (
	"math"
	"testing"
)

func TestClassificationMetrics(t *testing.T) {
	metrics := classificationMetrics("test", []labelPair{
		{expected: "A", actual: "A"},
		{expected: "A", actual: "B"},
		{expected: "B", actual: "B"},
		{expected: "B", actual: "B"},
	})
	if metrics.Total != 4 || metrics.Correct != 3 {
		t.Fatalf("unexpected counts: total=%d correct=%d", metrics.Total, metrics.Correct)
	}
	if math.Abs(metrics.Accuracy-0.75) > 1e-9 {
		t.Fatalf("unexpected accuracy: %f", metrics.Accuracy)
	}
	if len(metrics.ConfusionMatrix) != 2 {
		t.Fatalf("unexpected matrix size: %d", len(metrics.ConfusionMatrix))
	}
}

func TestPercentile(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	if got := percentile(values, 0.5); got != 3 {
		t.Fatalf("median=%f, want 3", got)
	}
	if got := percentile(values, 0.95); got <= 4 || got > 5 {
		t.Fatalf("p95=%f, want >4 and <=5", got)
	}
}

func TestBuildMetricsReadsRuleMetadata(t *testing.T) {
	cases := []CaseResult{
		{Suite: "rule_engine", Category: "positive", Status: StatusPass, Metadata: map[string]interface{}{
			"expected_label": "RULE-A", "actual_label": "RULE-A",
			"expected_decision": "DECISION", "actual_decision": "DECISION",
		}},
		{Suite: "rule_engine", Category: "no_evidence", Status: StatusPass, Metadata: map[string]interface{}{
			"expected_label": fallbackLabel, "actual_label": fallbackLabel,
			"expected_decision": fallbackLabel, "actual_decision": fallbackLabel,
		}},
	}
	metrics := BuildMetrics(cases, ExpertValidationMetrics{})
	if metrics.RuleSelection.Accuracy != 1 {
		t.Fatalf("rule accuracy=%f, want 1", metrics.RuleSelection.Accuracy)
	}
	if metrics.DecisionGate.MacroF1Score != 1 {
		t.Fatalf("decision macro f1=%f, want 1", metrics.DecisionGate.MacroF1Score)
	}
}
