package evaluation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExpertCaseDefinitions(t *testing.T) {
	definitions := make([]ExpertCaseDefinition, 0, 90)
	for category, count := range expectedExpertCaseDistribution {
		for index := 0; index < count; index++ {
			definition := ExpertCaseDefinition{
				ID:            category + "-case-" + string(rune('A'+index)),
				Category:      category,
				PestLabel:     "penggerek_batang",
				Severity:      "ringan",
				GrowthStage:   "vegetatif",
				CNNConfidence: 0.9,
				Symptoms:      []string{"gejala contoh"},
				Critical:      true,
			}
			if category == "positive" {
				definition.ExpectedRuleCode = "RULE-TEST"
			} else {
				definition.ExpectedFallback = true
			}
			definitions = append(definitions, definition)
		}
	}

	path := filepath.Join(t.TempDir(), "rule_cases.json")
	raw, err := json.Marshal(definitions)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadExpertCaseDefinitions(path)
	if err != nil {
		t.Fatalf("loadExpertCaseDefinitions() error = %v", err)
	}
	if len(loaded) != 90 {
		t.Fatalf("expected 90 definitions, got %d", len(loaded))
	}
}

func TestLoadExpertCaseDefinitionsRejectsWrongCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rule_cases.json")
	if err := os.WriteFile(path, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadExpertCaseDefinitions(path); err == nil {
		t.Fatal("expected an error for an empty instrument")
	}
}
