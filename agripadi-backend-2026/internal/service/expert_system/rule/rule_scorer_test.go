package rule

import "testing"

func TestScoreCandidateDoesNotIncreaseFromImageConfidence(t *testing.T) {
	candidate := MatchCandidate{
		MatchRatio:         0.75,
		InputCoverageRatio: 0.80,
		BayesianScore:      0.75,
	}

	scorer := NewScorerService()
	low := scorer.ScoreCandidate(ScoreRequest{
		Candidate:     candidate,
		CNNConfidence: 0.10,
	})
	high := scorer.ScoreCandidate(ScoreRequest{
		Candidate:     candidate,
		CNNConfidence: 0.99,
	})

	if low.FinalScore != high.FinalScore {
		t.Fatalf("image confidence must not change field-evidence score: low=%f high=%f", low.FinalScore, high.FinalScore)
	}
}

func TestScoreCandidateUsesRuleInputAndEvidenceCoverage(t *testing.T) {
	candidate := MatchCandidate{
		MatchRatio:            0.80,
		InputCoverageRatio:    0.60,
		EvidenceCoverageRatio: 1.00,
		BayesianScore:         1.00,
		SemanticScore:         1.00,
	}

	result := NewScorerService().ScoreCandidate(ScoreRequest{
		Candidate:     candidate,
		CNNConfidence: 1.00,
	})

	want := (0.80 * 0.40) + (0.60 * 0.40) + (1.00 * 0.20)
	if result.FinalScore != want {
		t.Fatalf("unexpected field-evidence score: got=%f want=%f", result.FinalScore, want)
	}
}
