package rule

import "math"

type ScorerService struct{}

func NewScorerService() *ScorerService { return &ScorerService{} }

type ScoreRequest struct {
	Candidate     MatchCandidate
	CNNConfidence float64
	SeverityLevel string
}

type ScoreResult struct {
	BaseScore          float64
	WeightedScore      float64
	BayesianScore      float64
	SemanticScore      float64
	CNNScore           float64
	SeverityMultiplier float64
	PriorityBoost      float64
	AdaptiveConfidence float64
	FinalScore         float64
	Reasoning          []string
}

func (s *ScorerService) ScoreCandidate(req ScoreRequest) ScoreResult {
	candidate := req.Candidate
	base := candidate.FinalProbability
	if base <= 0 {
		base = (candidate.MatchRatio * 0.40) +
			(candidate.InputCoverageRatio * 0.40) +
			(candidate.EvidenceCoverageRatio * 0.20)
	}
	final := clamp(base)

	// Hasil foto tetap disimpan sebagai informasi awal, tetapi tidak menambah
	// nilai keputusan. Diagnosis harus ditopang bukti keberadaan hama dan
	// kerusakan tanaman yang dipilih pengguna.
	return ScoreResult{
		BaseScore:          base,
		WeightedScore:      final,
		BayesianScore:      candidate.BayesianScore,
		SemanticScore:      0,
		CNNScore:           clamp(req.CNNConfidence),
		SeverityMultiplier: 1.0,
		PriorityBoost:      0,
		AdaptiveConfidence: final,
		FinalScore:         final,
		Reasoning: append([]string{
			"Jenis hama dari foto digunakan sebagai petunjuk awal.",
			"Keputusan ditentukan oleh gejala lapangan, fase padi, dan tingkat serangan yang sesuai.",
		}, candidate.Reasoning...),
	}
}

func (s *ScorerService) calculateDynamicWeight(candidate MatchCandidate) float64 {
	weight := candidate.Rule.ConfidenceScore
	if weight <= 0 {
		weight = 0.85
	}
	return math.Max(0.75, math.Min(1.0, weight))
}

func (s *ScorerService) calculateCNNContribution(confidence float64) float64 {
	return clamp(confidence)
}

func clamp(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
