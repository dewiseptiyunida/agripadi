package rule

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/gustian305/backend/internal/domain"
)

func TestFindMatchesRequiresIdentityAndDamageEvidence(t *testing.T) {
	pestID := uuid.New()
	severityID := uuid.New()
	growthStageID := uuid.New()
	identityID := uuid.New()
	damageID := uuid.New()
	otherID := uuid.New()

	repo := &fakeRuleRepository{
		rules: []domain.ExpertRule{
			{
				ID:                uuid.New(),
				PestID:            pestID,
				SeverityID:        severityID,
				GrowthStageID:     growthStageID,
				Code:              "RULE-WBC-SED-VEG-01",
				MinimumMatchScore: 0.55,
				ConfidenceScore:   0.85,
				Priority:          2,
				IsActive:          true,
				Symptoms: []domain.ExpertRuleSymptom{
					{
						SymptomID: identityID,
						Weight:    0.90,
						Symptom: domain.Symptom{
							ID:          identityID,
							RuleRole:    "identity",
							SymptomType: "identity",
						},
					},
					{
						SymptomID: damageID,
						Weight:    0.85,
						Symptom: domain.Symptom{
							ID:          damageID,
							RuleRole:    "damage",
							SymptomType: "damage",
						},
					},
				},
			},
		},
	}

	matcher := NewMatcherService(repo, nil)

	matches, err := matcher.FindMatches(
		context.Background(),
		MatchRequest{
			PestID:        pestID,
			SeverityID:    severityID,
			GrowthStageID: growthStageID,
			SymptomIDs:    []uuid.UUID{identityID, damageID, otherID},
			CNNConfidence: 0.50,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected identity + damage evidence to match, got %d", len(matches))
	}

	identityOnly, err := matcher.FindMatches(
		context.Background(),
		MatchRequest{
			PestID:        pestID,
			SeverityID:    severityID,
			GrowthStageID: growthStageID,
			SymptomIDs:    []uuid.UUID{identityID, otherID},
			CNNConfidence: 0.95,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(identityOnly) != 0 {
		t.Fatalf("expected identity-only evidence to be rejected, got %d", len(identityOnly))
	}
}

type fakeRuleRepository struct {
	rules []domain.ExpertRule
}

func (r *fakeRuleRepository) FindActiveRulesByPestID(
	_ context.Context,
	pestID uuid.UUID,
) ([]domain.ExpertRule, error) {
	result := make([]domain.ExpertRule, 0, len(r.rules))

	for _, rule := range r.rules {
		if rule.PestID == pestID && rule.IsActive {
			result = append(result, rule)
		}
	}

	return result, nil
}

func TestFindMatchesAcceptsIdentityEvidenceFromAnotherSeverityRule(t *testing.T) {
	pestID := uuid.New()
	heavyID := uuid.New()
	mediumID := uuid.New()
	generativeID := uuid.New()
	visibleID := uuid.New()
	stickyID := uuid.New()
	hopperburnID := uuid.New()

	repo := &fakeRuleRepository{rules: []domain.ExpertRule{
		{
			ID:                uuid.New(),
			PestID:            pestID,
			SeverityID:        heavyID,
			GrowthStageID:     generativeID,
			Severity:          domain.Severity{ID: heavyID, Name: "berat"},
			GrowthStage:       domain.GrowthStage{ID: generativeID, Name: "generatif"},
			Code:              "RULE-WBC-BER-GEN",
			IsActive:          true,
			MinimumMatchScore: 0.60,
			Symptoms: []domain.ExpertRuleSymptom{
				{SymptomID: visibleID, Weight: 1, Symptom: domain.Symptom{ID: visibleID, Name: "Wereng terlihat di pangkal batang", RuleRole: "identity", SymptomType: "identity", GrowthStage: "vegetatif|generatif", Severity: "ringan|sedang|berat"}},
				{SymptomID: hopperburnID, Weight: 1, Symptom: domain.Symptom{ID: hopperburnID, Name: "Hopperburn", RuleRole: "severity_anchor", SymptomType: "damage", GrowthStage: "vegetatif|generatif", Severity: "berat"}},
			},
		},
		{
			ID:            uuid.New(),
			PestID:        pestID,
			SeverityID:    mediumID,
			GrowthStageID: generativeID,
			Severity:      domain.Severity{ID: mediumID, Name: "sedang"},
			GrowthStage:   domain.GrowthStage{ID: generativeID, Name: "generatif"},
			Code:          "RULE-WBC-SED-GEN",
			IsActive:      true,
			Symptoms: []domain.ExpertRuleSymptom{
				{SymptomID: stickyID, Weight: 1, Symptom: domain.Symptom{ID: stickyID, Name: "Pangkal batang terasa lengket karena embun madu", RuleRole: "identity", SymptomType: "identity", GrowthStage: "vegetatif|generatif", Severity: "ringan|sedang|berat"}},
			},
		},
	}}

	matcher := NewMatcherService(repo, nil)
	matches, err := matcher.FindMatches(context.Background(), MatchRequest{
		PestID:        pestID,
		SeverityID:    heavyID,
		GrowthStageID: generativeID,
		SymptomIDs:    []uuid.UUID{stickyID, hopperburnID},
		CNNConfidence: 0.95,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected valid heavy WBC match, got %d", len(matches))
	}
	if !matches[0].HasIdentityEvidence || !matches[0].HasDamageEvidence {
		t.Fatalf("expected identity and damage evidence, got %#v", matches[0])
	}
	if matches[0].FinalProbability < 0.60 {
		t.Fatalf("expected score to pass threshold, got %.4f", matches[0].FinalProbability)
	}
}

func TestFindMatchesRejectsDamageEvidenceFromWrongGrowthStage(t *testing.T) {
	pestID := uuid.New()
	heavyID := uuid.New()
	generativeID := uuid.New()
	vegetativeID := uuid.New()
	larvaID := uuid.New()
	malaiID := uuid.New()
	anakanID := uuid.New()

	repo := &fakeRuleRepository{rules: []domain.ExpertRule{
		{
			ID:                uuid.New(),
			PestID:            pestID,
			SeverityID:        heavyID,
			GrowthStageID:     generativeID,
			Severity:          domain.Severity{ID: heavyID, Name: "berat"},
			GrowthStage:       domain.GrowthStage{ID: generativeID, Name: "generatif"},
			Code:              "RULE-PB-BER-GEN",
			IsActive:          true,
			MinimumMatchScore: 0.60,
			Symptoms: []domain.ExpertRuleSymptom{
				{SymptomID: larvaID, Weight: 1, Symptom: domain.Symptom{ID: larvaID, Name: "Larva ditemukan di dalam batang", RuleRole: "identity", SymptomType: "identity", GrowthStage: "vegetatif|generatif", Severity: "ringan|sedang|berat"}},
				{SymptomID: malaiID, Weight: 1, Symptom: domain.Symptom{ID: malaiID, Name: "Banyak malai putih", RuleRole: "severity_anchor", SymptomType: "damage", GrowthStage: "generatif", Severity: "berat"}},
			},
		},
		{
			ID:            uuid.New(),
			PestID:        pestID,
			SeverityID:    heavyID,
			GrowthStageID: vegetativeID,
			Severity:      domain.Severity{ID: heavyID, Name: "berat"},
			GrowthStage:   domain.GrowthStage{ID: vegetativeID, Name: "vegetatif"},
			Code:          "RULE-PB-BER-VEG",
			IsActive:      true,
			Symptoms: []domain.ExpertRuleSymptom{
				{SymptomID: anakanID, Weight: 1, Symptom: domain.Symptom{ID: anakanID, Name: "Beberapa anakan dalam rumpun mati", RuleRole: "damage", SymptomType: "damage", GrowthStage: "vegetatif", Severity: "sedang|berat"}},
			},
		},
	}}

	matcher := NewMatcherService(repo, nil)
	matches, err := matcher.FindMatches(context.Background(), MatchRequest{
		PestID:        pestID,
		SeverityID:    heavyID,
		GrowthStageID: generativeID,
		SymptomIDs:    []uuid.UUID{larvaID, anakanID},
		CNNConfidence: 0.95,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected stage-conflicting damage to be rejected, got %d", len(matches))
	}
}

func TestFindMatchesRejectsEvidenceFromWrongSeverity(t *testing.T) {
	pestID := uuid.New()
	lightID := uuid.New()
	generativeID := uuid.New()
	identityID := uuid.New()
	heavyDamageID := uuid.New()
	lightDamageID := uuid.New()

	repo := &fakeRuleRepository{rules: []domain.ExpertRule{
		{
			ID:                uuid.New(),
			PestID:            pestID,
			SeverityID:        lightID,
			GrowthStageID:     generativeID,
			Severity:          domain.Severity{ID: lightID, Name: "ringan"},
			GrowthStage:       domain.GrowthStage{ID: generativeID, Name: "generatif"},
			Code:              "RULE-WS-RIN-GEN",
			IsActive:          true,
			MinimumMatchScore: 0.55,
			Symptoms: []domain.ExpertRuleSymptom{
				{SymptomID: identityID, Weight: 1, Symptom: domain.Symptom{ID: identityID, Name: "Walang sangit terlihat", RuleRole: "identity", SymptomType: "identity", GrowthStage: "generatif", Severity: "ringan|sedang|berat"}},
				{SymptomID: lightDamageID, Weight: 1, Symptom: domain.Symptom{ID: lightDamageID, Name: "Bintik cokelat pada gabah", RuleRole: "damage", SymptomType: "damage", GrowthStage: "generatif", Severity: "ringan"}},
			},
		},
		{
			ID:            uuid.New(),
			PestID:        pestID,
			SeverityID:    uuid.New(),
			GrowthStageID: generativeID,
			Severity:      domain.Severity{Name: "berat"},
			GrowthStage:   domain.GrowthStage{ID: generativeID, Name: "generatif"},
			Code:          "RULE-WS-BER-GEN",
			IsActive:      true,
			Symptoms: []domain.ExpertRuleSymptom{
				{SymptomID: heavyDamageID, Weight: 1, Symptom: domain.Symptom{ID: heavyDamageID, Name: "Banyak gabah hampa", RuleRole: "severity_anchor", SymptomType: "damage", GrowthStage: "generatif", Severity: "berat"}},
			},
		},
	}}

	matcher := NewMatcherService(repo, nil)
	matches, err := matcher.FindMatches(context.Background(), MatchRequest{
		PestID:        pestID,
		SeverityID:    lightID,
		GrowthStageID: generativeID,
		SymptomIDs:    []uuid.UUID{identityID, heavyDamageID},
		CNNConfidence: 0.99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected severity-conflicting evidence to be rejected, got %d", len(matches))
	}
}
