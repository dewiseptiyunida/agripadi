package diagnose

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestMergeExistingSymptomInputsAccumulatesOneTapSelections(t *testing.T) {
	previous := SymptomSessionData{
		UserInputs: []string{"Wereng terlihat di pangkal batang"},
	}
	raw, err := json.Marshal(previous)
	if err != nil {
		t.Fatalf("marshal previous symptoms: %v", err)
	}

	got := mergeExistingSymptomInputs(raw, []string{
		"Daun bagian bawah mulai menguning",
		"wereng terlihat di pangkal batang",
	})

	if len(got) != 2 {
		t.Fatalf("expected two accumulated unique symptoms, got %d: %#v", len(got), got)
	}
	if got[0] != "Wereng terlihat di pangkal batang" {
		t.Fatalf("unexpected first symptom: %q", got[0])
	}
	if got[1] != "Daun bagian bawah mulai menguning" {
		t.Fatalf("unexpected second symptom: %q", got[1])
	}
}

func TestNeedsMoreFieldSymptomsRequiresIdentityAndDamage(t *testing.T) {
	identityID := uuid.New()
	damageID := uuid.New()

	tests := []struct {
		name string
		data *SymptomSessionData
		want bool
	}{
		{
			name: "identity only",
			data: &SymptomSessionData{Normalized: []NormalizedSymptomData{
				{MatchedSymptomID: &identityID, Confidence: 1, RuleRole: "identity", SymptomType: "identity"},
			}},
			want: true,
		},
		{
			name: "damage only",
			data: &SymptomSessionData{Normalized: []NormalizedSymptomData{
				{MatchedSymptomID: &damageID, Confidence: 1, RuleRole: "damage", SymptomType: "damage"},
			}},
			want: true,
		},
		{
			name: "identity and damage",
			data: &SymptomSessionData{Normalized: []NormalizedSymptomData{
				{MatchedSymptomID: &identityID, Confidence: 1, RuleRole: "identity", SymptomType: "identity"},
				{MatchedSymptomID: &damageID, Confidence: 1, RuleRole: "severity_anchor", SymptomType: "damage"},
			}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsMoreFieldSymptomsBeforeSeverity(tt.data); got != tt.want {
				t.Fatalf("needsMoreFieldSymptomsBeforeSeverity() = %v, want %v", got, tt.want)
			}
		})
	}
}
