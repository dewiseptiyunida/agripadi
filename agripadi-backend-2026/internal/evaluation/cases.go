package evaluation

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

var expectedExpertCaseDistribution = map[string]int{
	"positive":              15,
	"identity_only":         15,
	"damage_only":           15,
	"no_evidence":           15,
	"growth_stage_mismatch": 15,
	"severity_mismatch":     15,
}

func loadExpertCaseDefinitions(path string) ([]ExpertCaseDefinition, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("path skenario rule engine kosong")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("membaca skenario rule engine %s: %w", path, err)
	}

	var definitions []ExpertCaseDefinition
	if err := json.Unmarshal(raw, &definitions); err != nil {
		return nil, fmt.Errorf("format JSON skenario rule engine tidak valid: %w", err)
	}
	if len(definitions) != 90 {
		return nil, fmt.Errorf("instrumen rule engine wajib berisi 90 kasus, ditemukan %d", len(definitions))
	}

	seenIDs := make(map[string]struct{}, len(definitions))
	distribution := make(map[string]int, len(expectedExpertCaseDistribution))
	for index, definition := range definitions {
		definition.ID = strings.TrimSpace(definition.ID)
		definition.Category = strings.ToLower(strings.TrimSpace(definition.Category))
		definition.PestLabel = strings.ToLower(strings.TrimSpace(definition.PestLabel))
		definition.Severity = strings.ToLower(strings.TrimSpace(definition.Severity))
		definition.GrowthStage = strings.ToLower(strings.TrimSpace(definition.GrowthStage))
		definition.ExpectedRuleCode = strings.TrimSpace(definition.ExpectedRuleCode)
		for symptomIndex := range definition.Symptoms {
			definition.Symptoms[symptomIndex] = strings.TrimSpace(definition.Symptoms[symptomIndex])
		}
		definitions[index] = definition

		if definition.ID == "" {
			return nil, fmt.Errorf("kasus ke-%d tidak memiliki id", index+1)
		}
		if _, exists := seenIDs[definition.ID]; exists {
			return nil, fmt.Errorf("id kasus rule engine duplikat: %s", definition.ID)
		}
		seenIDs[definition.ID] = struct{}{}

		expectedCount, knownCategory := expectedExpertCaseDistribution[definition.Category]
		if !knownCategory || expectedCount <= 0 {
			return nil, fmt.Errorf("kategori kasus %s tidak dikenal pada %s", definition.Category, definition.ID)
		}
		distribution[definition.Category]++
		if definition.PestLabel == "" || definition.Severity == "" || definition.GrowthStage == "" {
			return nil, fmt.Errorf("kasus %s wajib memiliki pest_label, severity, dan growth_stage", definition.ID)
		}
		if definition.CNNConfidence < 0 || definition.CNNConfidence > 1 {
			return nil, fmt.Errorf("cnn_confidence kasus %s harus berada pada rentang 0-1", definition.ID)
		}
		if definition.Category == "positive" {
			if definition.ExpectedFallback || definition.ExpectedRuleCode == "" {
				return nil, fmt.Errorf("kasus positif %s wajib memiliki expected_rule_code dan expected_fallback=false", definition.ID)
			}
			if len(definition.Symptoms) == 0 {
				return nil, fmt.Errorf("kasus positif %s wajib memiliki gejala", definition.ID)
			}
		} else if !definition.ExpectedFallback {
			return nil, fmt.Errorf("kasus negatif %s wajib memiliki expected_fallback=true", definition.ID)
		}
	}

	for category, expectedCount := range expectedExpertCaseDistribution {
		if distribution[category] != expectedCount {
			return nil, fmt.Errorf("kategori %s wajib berisi %d kasus, ditemukan %d", category, expectedCount, distribution[category])
		}
	}
	return definitions, nil
}

func normalizeCaseLookup(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}
