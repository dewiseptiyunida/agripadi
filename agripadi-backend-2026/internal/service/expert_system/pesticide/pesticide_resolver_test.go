package pesticide

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gustian305/backend/internal/domain"
)

func TestResolverDoesNotChangeRankingFromSeverityAlone(t *testing.T) {
	pestID := uuid.New()
	repo := &fakePesticideRepo{items: []domain.Pesticide{
		validProduct("AREA WG", "WG", pestID, "200", "g/ha", "Bahan A"),
		validProduct("SPOT SL", "SL", pestID, "2", "ml/l", "Bahan B"),
	}}

	resolver := NewResolverService(repo)
	high, err := resolver.Resolve(context.Background(), ResolveRequest{PestID: pestID, Severity: "high", TopK: 2})
	if err != nil {
		t.Fatalf("resolve high severity: %v", err)
	}
	low, err := resolver.Resolve(context.Background(), ResolveRequest{PestID: pestID, Severity: "low", TopK: 2})
	if err != nil {
		t.Fatalf("resolve low severity: %v", err)
	}

	if len(high) != 2 || len(low) != 2 {
		t.Fatalf("expected two recommendations for both severities")
	}
	for i := range high {
		if high[i].Pesticide.ProductName != low[i].Pesticide.ProductName {
			t.Fatalf("severity alone must not change product order: high=%s low=%s", high[i].Pesticide.ProductName, low[i].Pesticide.ProductName)
		}
		if high[i].ScoreBreakdown.SeverityFit != 0.50 || low[i].ScoreBreakdown.SeverityFit != 0.50 {
			t.Fatalf("severity fit must remain neutral without verified label metadata")
		}
	}
}

func TestResolverSkipsProductWithoutTargetDosage(t *testing.T) {
	pestID := uuid.New()
	otherPestID := uuid.New()
	invalid := validProduct("NO TARGET DOSE", "WG", otherPestID, "200", "g/ha", "Bahan A")
	valid := validProduct("VALID TARGET DOSE", "WG", pestID, "200", "g/ha", "Bahan B")

	resolver := NewResolverService(&fakePesticideRepo{items: []domain.Pesticide{invalid, valid}})
	results, err := resolver.Resolve(context.Background(), ResolveRequest{PestID: pestID, Severity: "high", TopK: 5})
	if err != nil {
		t.Fatalf("resolve recommendations: %v", err)
	}
	if len(results) != 1 || results[0].Pesticide.ProductName != "VALID TARGET DOSE" {
		t.Fatalf("expected only product with target-specific dose, got %#v", results)
	}
}

func TestResolverUsesNeutralSafetyWithoutVerifiedLabelMetadata(t *testing.T) {
	pestID := uuid.New()
	a := validProduct("PRODUCT A", "SC", pestID, "1-2", "ml/l", "chlorpyrifos")
	b := validProduct("PRODUCT B", "SC", pestID, "1-2", "ml/l", "abamectin")

	resolver := NewResolverService(&fakePesticideRepo{items: []domain.Pesticide{a, b}})
	results, err := resolver.Resolve(context.Background(), ResolveRequest{PestID: pestID, Severity: "low", TopK: 2})
	if err != nil {
		t.Fatalf("resolve recommendations: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected two recommendations, got %d", len(results))
	}
	for _, result := range results {
		if result.ScoreBreakdown.Safety != 0.50 {
			t.Fatalf("expected neutral safety score without verified product label metadata, got %.2f", result.ScoreBreakdown.Safety)
		}
	}
}

func TestResolverDoesNotGuessIngredientEfficacy(t *testing.T) {
	pestID := uuid.New()
	a := validProduct("PRODUCT A", "SC", pestID, "1", "ml/l", "Emamectin Benzoate")
	b := validProduct("PRODUCT B", "SC", pestID, "1", "ml/l", "Pimetrozin")

	resolver := NewResolverService(&fakePesticideRepo{items: []domain.Pesticide{a, b}})
	results, err := resolver.Resolve(context.Background(), ResolveRequest{PestID: pestID, PestName: "wereng batang cokelat", Severity: "medium", TopK: 2})
	if err != nil {
		t.Fatalf("resolve recommendations: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected two recommendations, got %d", len(results))
	}
	if results[0].ScoreBreakdown.IngredientFit != results[1].ScoreBreakdown.IngredientFit {
		t.Fatalf("ingredient efficacy must not be guessed without a validated effectiveness table")
	}
}

func TestResolverRejectsExpiredFutureAndNonRiceProducts(t *testing.T) {
	pestID := uuid.New()
	now := time.Now()
	valid := validProduct("VALID", "SC", pestID, "1", "ml/l", "Bahan A")
	expired := validProduct("EXPIRED", "SC", pestID, "1", "ml/l", "Bahan B")
	past := now.Add(-24 * time.Hour)
	expired.ExpiredAt = &past
	future := validProduct("FUTURE", "SC", pestID, "1", "ml/l", "Bahan C")
	future.RegisteredAt = now.Add(24 * time.Hour)
	nonRice := validProduct("NON RICE", "SC", pestID, "1", "ml/l", "Bahan D")
	nonRice.Commodity = "jagung"

	resolver := NewResolverService(&fakePesticideRepo{items: []domain.Pesticide{valid, expired, future, nonRice}})
	results, err := resolver.Resolve(context.Background(), ResolveRequest{PestID: pestID, TopK: 10})
	if err != nil {
		t.Fatalf("resolve recommendations: %v", err)
	}
	if len(results) != 1 || results[0].Pesticide.ProductName != "VALID" {
		t.Fatalf("expected only active rice registration, got %#v", results)
	}
}

type fakePesticideRepo struct {
	items []domain.Pesticide
}

func (r *fakePesticideRepo) FindByPestID(context.Context, uuid.UUID) ([]domain.Pesticide, error) {
	return r.items, nil
}

func validProduct(name, formulation string, pestID uuid.UUID, doseRaw, doseUnit, ingredient string) domain.Pesticide {
	return domain.Pesticide{
		ID:            uuid.New(),
		ProductName:   name,
		PesticideType: "INSECTICIDE",
		Formulation:   formulation,
		Commodity:     "padi",
		RegisteredAt:  time.Now().Add(-24 * time.Hour),
		Ingredients: []domain.PesticideIngredient{
			{Name: ingredient, ConcentrationRaw: "contoh"},
		},
		Dosages: []domain.PesticideDosage{
			{PestID: pestID, DoseRaw: doseRaw, DoseUnit: doseUnit},
		},
	}
}
