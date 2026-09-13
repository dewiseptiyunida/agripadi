package rule

import (
	"errors"
	"math"
	"sort"
)

type ResolverService struct {
	scorer *ScorerService
}

func NewResolverService(
	scorer *ScorerService,
) *ResolverService {

	return &ResolverService{
		scorer: scorer,
	}
}

type ResolveRequest struct {
	Candidates         []MatchCandidate
	CNNConfidence      float64
	SeverityLevel      string
	TopK               int
	EnableFallback     bool
	AmbiguityThreshold float64
}

type ResolveResponse struct {
	Primary        *ResolvedRule
	TopK           []ResolvedRule
	Ambiguous      bool
	AmbiguityScore float64
	FallbackUsed   bool
	Explainability ExplainabilityResult
}

type ResolvedRule struct {
	Rank                int
	Candidate           MatchCandidate
	Score               ScoreResult
	ExplainabilityScore float64
}

type ExplainabilityResult struct {
	Reasoning         []string
	ConfidenceSummary string
	DecisionSource    string
}

func (s *ResolverService) Resolve(req ResolveRequest) (*ResolveResponse, error) {

	if len(req.Candidates) == 0 {

		if !req.EnableFallback {

			return nil, nil
		}

		return s.buildFallbackResponse(), nil
	}

	results := make(
		[]ResolvedRule,
		0,
		len(req.Candidates),
	)

	// ========================================================
	// SCORE ALL CANDIDATES
	// ========================================================

	for _, candidate := range req.Candidates {

		score :=
			s.scorer.ScoreCandidate(
				ScoreRequest{
					Candidate: candidate,

					CNNConfidence: req.CNNConfidence,

					SeverityLevel: req.SeverityLevel,
				},
			)

		explainability :=
			s.calculateExplainability(
				candidate,
				score,
			)

		results = append(
			results,
			ResolvedRule{
				Candidate: candidate,

				Score: score,

				ExplainabilityScore: explainability,
			},
		)
	}

	// ========================================================
	// RANKING
	// ========================================================

	sort.Slice(
		results,
		func(i, j int) bool {

			if results[i].Score.FinalScore !=
				results[j].Score.FinalScore {

				return results[i].Score.FinalScore >
					results[j].Score.FinalScore
			}

			// ====================================================
			// EXPLAINABILITY
			// ====================================================

			if results[i].ExplainabilityScore !=
				results[j].ExplainabilityScore {

				return results[i].ExplainabilityScore >
					results[j].ExplainabilityScore
			}

			// ====================================================
			// MATCH RATIO
			// ====================================================

			if results[i].Candidate.MatchRatio !=
				results[j].Candidate.MatchRatio {

				return results[i].Candidate.MatchRatio >
					results[j].Candidate.MatchRatio
			}

			// ====================================================
			// PRIORITY
			// ====================================================

			return results[i].Candidate.Rule.Priority <
				results[j].Candidate.Rule.Priority
		},
	)

	// ========================================================
	// ASSIGN RANK
	// ========================================================

	for i := range results {

		results[i].Rank = i + 1
	}

	// ========================================================
	// TOP K
	// ========================================================

	topK :=
		s.buildTopK(
			results,
			req.TopK,
		)

	if len(topK) == 0 {

		return nil,
			errors.New(
				"no resolved diagnosis",
			)
	}

	primary := topK[0]

	// ========================================================
	// AMBIGUITY DETECTION
	// ========================================================

	ambiguous,
		ambiguityScore :=
		s.detectAmbiguity(
			topK,
			req.AmbiguityThreshold,
		)

	// ========================================================
	// EXPLAINABILITY
	// ========================================================

	explainability :=
		s.buildExplainability(
			primary,
			ambiguous,
		)

	return &ResolveResponse{
		Primary: &primary,

		TopK: topK,

		Ambiguous: ambiguous,

		AmbiguityScore: ambiguityScore,

		FallbackUsed: false,

		Explainability: explainability,
	}, nil
}

func (s *ResolverService) buildTopK(results []ResolvedRule, k int) []ResolvedRule {

	if k <= 0 {
		k = 3
	}

	if len(results) <= k {
		return results
	}

	return results[:k]
}

func (s *ResolverService) calculateExplainability(candidate MatchCandidate, score ScoreResult) float64 {
	// Nilai penjelasan memakai dasar yang sama dengan keputusan agar tidak ada
	// penilaian tersembunyi dari keyakinan foto atau kemiripan semantik.
	_ = score
	return clampExplainability(
		(candidate.MatchRatio * 0.65) +
			(candidate.InputCoverageRatio * 0.35),
	)
}

func (s *ResolverService) detectAmbiguity(results []ResolvedRule, threshold float64) (bool, float64) {

	if len(results) < 2 {

		return false, 0
	}

	if threshold <= 0 {
		threshold = 0.08
	}

	first := results[0]

	second := results[1]

	diff :=
		math.Abs(
			first.Score.FinalScore -
				second.Score.FinalScore,
		)

	ambiguity :=
		1 - diff

	return diff <= threshold,
		clampExplainability(
			ambiguity,
		)
}

func (s *ResolverService) buildExplainability(primary ResolvedRule, ambiguous bool) ExplainabilityResult {

	reasoning := make(
		[]string,
		0,
	)

	reasoning = append(
		reasoning,
		"Hasil dipilih dari kecocokan jenis hama, gejala lapangan, fase tanaman, dan tingkat serangan.",
	)

	reasoning = append(
		reasoning,
		"Foto digunakan sebagai petunjuk awal dan diperiksa kembali dengan gejala yang dipilih.",
	)

	if ambiguous {

		reasoning = append(
			reasoning,
			"Beberapa kemungkinan memiliki nilai yang berdekatan; periksa kembali gejala di lahan.",
		)
	}

	reasoning = append(
		reasoning,
		primary.Score.Reasoning...,
	)

	confidence := "low"

	switch {

	case primary.Score.FinalScore >= 0.85:
		confidence = "very_high"

	case primary.Score.FinalScore >= 0.70:
		confidence = "high"

	case primary.Score.FinalScore >= 0.55:
		confidence = "medium"
	}

	return ExplainabilityResult{
		Reasoning: reasoning,

		ConfidenceSummary: confidence,

		DecisionSource: "pemeriksaan_terpadu",
	}
}

func (s *ResolverService) buildFallbackResponse() *ResolveResponse {

	return &ResolveResponse{
		Primary: nil,

		TopK: []ResolvedRule{},

		Ambiguous: true,

		AmbiguityScore: 1.0,

		FallbackUsed: true,

		Explainability: ExplainabilityResult{
			Reasoning: []string{
				"Kombinasi gejala belum cukup cocok dengan data pemeriksaan.",
				"Periksa kembali foto, gejala, fase tanaman, dan tingkat serangan.",
			},

			ConfidenceSummary: "unknown",

			DecisionSource: "perlu_pemeriksaan_ulang",
		},
	}
}

func clampExplainability(value float64) float64 {

	return math.Max(
		0,
		math.Min(
			1,
			value,
		),
	)
}
