package expertSystem

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gustian305/backend/internal/domain"
	"github.com/gustian305/backend/internal/service/expert_system/diagnose"
	"gorm.io/datatypes"
)

type symptomFlowCatalogStub struct {
	pest     domain.Pest
	symptoms []domain.Symptom
}

func (s symptomFlowCatalogStub) FindPestByLabelName(_ context.Context, _ string) (*domain.Pest, error) {
	pest := s.pest
	return &pest, nil
}

func (s symptomFlowCatalogStub) FindSymptomsByPestID(_ context.Context, _ uuid.UUID) ([]domain.Symptom, error) {
	return append([]domain.Symptom(nil), s.symptoms...), nil
}

func TestAdditionalSymptomQuestionOffersMissingIdentityEvidence(t *testing.T) {
	pestID := uuid.New()
	damageID := uuid.New()
	session := symptomFlowSession(t, []diagnose.NormalizedSymptomData{
		{
			InputText:          "Daun bagian bawah mulai menguning",
			MatchedSymptomID:   &damageID,
			MatchedSymptomName: "Daun bagian bawah mulai menguning",
			Confidence:         1,
			SymptomType:        "damage",
			RuleRole:           "severity_anchor",
		},
	})

	repo := symptomFlowCatalogStub{
		pest: domain.Pest{ID: pestID, Name: "wereng batang cokelat", LabelName: "wereng_batang_cokelat"},
		symptoms: []domain.Symptom{
			{
				ID:                 uuid.New(),
				Name:               "Wereng terlihat di pangkal batang",
				SymptomType:        "identity",
				RuleRole:           "identity",
				UserObservable:     true,
				RecommendedForRule: true,
				IsCoreSymptom:      true,
			},
			{
				ID:                 damageID,
				Name:               "Daun bagian bawah mulai menguning",
				SymptomType:        "damage",
				RuleRole:           "severity_anchor",
				UserObservable:     true,
				RecommendedForRule: true,
				IsCoreSymptom:      true,
			},
		},
	}

	response := buildAdditionalSymptomQuestionResponse(context.Background(), repo, session)
	if response == nil {
		t.Fatal("expected response")
	}
	if !strings.Contains(strings.ToLower(response.Message), "tanda keberadaan hama") {
		t.Fatalf("expected missing identity guidance, got: %s", response.Message)
	}
	if len(response.Actions) != 1 {
		t.Fatalf("expected one identity action, got %d: %#v", len(response.Actions), response.Actions)
	}
	if response.Actions[0].Value != "Wereng terlihat di pangkal batang" {
		t.Fatalf("unexpected action: %#v", response.Actions[0])
	}
}

func TestAdditionalSymptomQuestionOffersMissingDamageEvidence(t *testing.T) {
	pestID := uuid.New()
	identityID := uuid.New()
	session := symptomFlowSession(t, []diagnose.NormalizedSymptomData{
		{
			InputText:          "Wereng terlihat di pangkal batang",
			MatchedSymptomID:   &identityID,
			MatchedSymptomName: "Wereng terlihat di pangkal batang",
			Confidence:         1,
			SymptomType:        "identity",
			RuleRole:           "identity",
		},
	})

	repo := symptomFlowCatalogStub{
		pest: domain.Pest{ID: pestID, Name: "wereng batang cokelat", LabelName: "wereng_batang_cokelat"},
		symptoms: []domain.Symptom{
			{
				ID:                 identityID,
				Name:               "Wereng terlihat di pangkal batang",
				SymptomType:        "identity",
				RuleRole:           "identity",
				UserObservable:     true,
				RecommendedForRule: true,
				IsCoreSymptom:      true,
			},
			{
				ID:                 uuid.New(),
				Name:               "Daun bagian bawah mulai menguning",
				SymptomType:        "damage",
				RuleRole:           "severity_anchor",
				UserObservable:     true,
				RecommendedForRule: true,
				IsCoreSymptom:      true,
			},
		},
	}

	response := buildAdditionalSymptomQuestionResponse(context.Background(), repo, session)
	if response == nil {
		t.Fatal("expected response")
	}
	if !strings.Contains(strings.ToLower(response.Message), "gejala kerusakan") {
		t.Fatalf("expected missing damage guidance, got: %s", response.Message)
	}
	if len(response.Actions) != 1 {
		t.Fatalf("expected one damage action, got %d: %#v", len(response.Actions), response.Actions)
	}
	if response.Actions[0].Value != "Daun bagian bawah mulai menguning" {
		t.Fatalf("unexpected action: %#v", response.Actions[0])
	}
}

func symptomFlowSession(t *testing.T, normalized []diagnose.NormalizedSymptomData) *domain.ExpertSession {
	t.Helper()
	data := diagnose.SymptomSessionData{Normalized: normalized}
	for _, item := range normalized {
		data.UserInputs = append(data.UserInputs, item.InputText)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal symptom data: %v", err)
	}
	return &domain.ExpertSession{
		ID:            uuid.New(),
		DetectedLabel: "wereng_batang_cokelat",
		Symptoms:      datatypes.JSON(raw),
	}
}

func TestInitialSymptomChoicesContainIdentityAndDamage(t *testing.T) {
	symptoms := []domain.Symptom{
		{
			ID:                 uuid.New(),
			Name:               "Daun bagian bawah mulai menguning",
			SymptomType:        "damage",
			RuleRole:           "severity_anchor",
			UserObservable:     true,
			RecommendedForRule: true,
			IsCoreSymptom:      true,
		},
		{
			ID:                 uuid.New(),
			Name:               "Wereng terlihat di pangkal batang",
			SymptomType:        "identity",
			RuleRole:           "identity",
			UserObservable:     true,
			RecommendedForRule: true,
			IsCoreSymptom:      true,
		},
		{
			ID:                 uuid.New(),
			Name:               "Tanaman mengering seperti terbakar",
			SymptomType:        "damage",
			RuleRole:           "damage",
			UserObservable:     true,
			RecommendedForRule: true,
		},
	}

	selected := selectStateFlowSymptoms(symptoms, 2)
	if len(selected) != 2 {
		t.Fatalf("expected two choices, got %d", len(selected))
	}

	hasIdentity := false
	hasDamage := false
	for _, item := range selected {
		switch symptomEvidenceKind(item) {
		case "identity":
			hasIdentity = true
		case "damage":
			hasDamage = true
		}
	}

	if !hasIdentity || !hasDamage {
		t.Fatalf("expected balanced identity and damage choices, got %#v", selected)
	}
}

func TestInferGrowthStageEvidenceUsesMatchedDamageMetadata(t *testing.T) {
	identityID := uuid.New()
	damageID := uuid.New()
	session := symptomFlowSession(t, []diagnose.NormalizedSymptomData{
		{
			InputText:          "Larva ditemukan di dalam batang",
			MatchedSymptomID:   &identityID,
			MatchedSymptomName: "Larva ditemukan di dalam batang",
			Confidence:         1,
			SymptomType:        "identity",
			RuleRole:           "identity",
			GrowthStage:        "vegetatif|generatif",
			Severity:           "ringan|sedang|berat",
		},
		{
			InputText:          "Beberapa anakan dalam rumpun mati",
			MatchedSymptomID:   &damageID,
			MatchedSymptomName: "Beberapa anakan dalam rumpun mati",
			Confidence:         1,
			SymptomType:        "damage",
			RuleRole:           "damage",
			GrowthStage:        "vegetatif",
			Severity:           "sedang|berat",
		},
	})
	session.DetectedLabel = "penggerek_batang"

	evidence := inferGrowthStageEvidenceFromSession(session)
	if evidence.RequiredStage != "vegetatif" {
		t.Fatalf("expected vegetative stage from damage metadata, got %#v", evidence)
	}

	response := buildGrowthStageEvidenceConflictResponse(session.ID, session, "generatif")
	if response == nil {
		t.Fatal("expected conflict response for generative selection")
	}
	if response.State != "collect_growth_stage" {
		t.Fatalf("expected collect_growth_stage state, got %s", response.State)
	}
}
