package rule

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/gustian305/backend/internal/domain"
)

type Service struct {
	matcher  *MatcherService
	scorer   *ScorerService
	resolver *ResolverService
}

func NewService(matcher *MatcherService, scorer *ScorerService, resolver *ResolverService) *Service {
	return &Service{
		matcher:  matcher,
		scorer:   scorer,
		resolver: resolver,
	}
}

type ExecuteRequest struct {
	PestID            uuid.UUID
	SeverityID        uuid.UUID
	GrowthStageID     uuid.UUID
	SymptomIDs        []uuid.UUID
	CNNConfidence     float64
	MinimumBayesScore float64
	SeverityLevel     string
}

type ExecuteResponse struct {
	BestMatch        *ResolvedRule
	TopMatches       []ResolvedRule
	IsAmbiguous      bool
	FallbackRequired bool
	AmbiguityScore   float64
	Explainability   ExplainabilityResult
}

func (s *Service) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {

	// ========================================================
	// VALIDATION
	// ========================================================

	if req.PestID == uuid.Nil {

		return nil,
			errors.New(
				"jenis hama belum dapat dikenali",
			)
	}

	if len(req.SymptomIDs) == 0 {

		return &ExecuteResponse{
			FallbackRequired: true,
			Explainability: ExplainabilityResult{
				Reasoning: []string{
					"Gejala yang dipilih belum cukup untuk memastikan kondisi tanaman.",
				},
				ConfidenceSummary: "unknown",
				DecisionSource:    "fallback_rule_engine",
			},
		}, nil
	}

	// ========================================================
	// MATCH
	// ========================================================

	candidates, err :=
		s.findMatchesWithFallback(
			ctx,
			req,
		)

	if err != nil {
		return nil, err
	}

	// ========================================================
	// EMPTY MATCH
	// ========================================================

	if len(candidates) == 0 {

		return &ExecuteResponse{
			FallbackRequired: true,
		}, nil
	}

	// ========================================================
	// RESOLVE
	// ========================================================

	resolved, err :=
		s.resolver.Resolve(
			ResolveRequest{
				Candidates: candidates,

				CNNConfidence: req.CNNConfidence,

				SeverityLevel: req.SeverityLevel,

				TopK: 3,

				EnableFallback: true,

				AmbiguityThreshold: 0.10,
			},
		)

	if err != nil {
		return nil, err
	}

	if resolved == nil ||
		resolved.Primary == nil {

		return &ExecuteResponse{
			FallbackRequired: true,
		}, nil
	}

	return &ExecuteResponse{
		BestMatch: resolved.Primary,

		TopMatches: resolved.TopK,

		IsAmbiguous: resolved.Ambiguous,

		FallbackRequired: resolved.FallbackUsed,

		AmbiguityScore: resolved.AmbiguityScore,

		Explainability: resolved.Explainability,
	}, nil
}

func (s *Service) findMatchesWithFallback(ctx context.Context, req ExecuteRequest) ([]MatchCandidate, error) {
	// Fase pertumbuhan dan tingkat serangan merupakan fakta lapangan utama.
	// Keduanya tidak dilonggarkan saat aturan tidak cocok karena hal tersebut
	// dapat menghasilkan rekomendasi untuk kondisi tanaman yang berbeda.
	return s.matcher.FindMatches(ctx, MatchRequest{
		PestID:            req.PestID,
		SeverityID:        req.SeverityID,
		GrowthStageID:     req.GrowthStageID,
		SymptomIDs:        req.SymptomIDs,
		CNNConfidence:     req.CNNConfidence,
		MinimumBayesScore: req.MinimumBayesScore,
	})
}

func (s *Service) GetBestRule(ctx context.Context, req ExecuteRequest) (*domain.ExpertRule, error) {

	result, err :=
		s.Execute(
			ctx,
			req,
		)

	if err != nil {
		return nil, err
	}

	if result == nil ||
		result.BestMatch == nil {

		return nil, nil
	}

	return &result.BestMatch.Candidate.Rule,
		nil
}
