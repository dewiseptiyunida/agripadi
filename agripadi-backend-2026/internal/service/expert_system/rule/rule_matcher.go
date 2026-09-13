package rule

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/gustian305/backend/internal/domain"
)

type RuleRepository interface {
	FindActiveRulesByPestID(ctx context.Context, pestID uuid.UUID) ([]domain.ExpertRule, error)
}

// PestSymptomCatalogRepository bersifat opsional. Repository produksi
// mengimplementasikannya agar matcher dapat memeriksa seluruh gejala valid
// milik hama, termasuk gejala identitas yang belum dicantumkan pada rule lama.
type PestSymptomCatalogRepository interface {
	FindSymptomsByPestID(ctx context.Context, pestID uuid.UUID) ([]domain.Symptom, error)
}

type SemanticMatcher interface {
	CalculateSimilarity(ctx context.Context, inputSymptoms []uuid.UUID, ruleSymptoms []uuid.UUID) (float64, error)
}

type MatcherService struct {
	repo            RuleRepository
	semanticMatcher SemanticMatcher
}

func NewMatcherService(repo RuleRepository, semanticMatcher SemanticMatcher) *MatcherService {
	return &MatcherService{
		repo:            repo,
		semanticMatcher: semanticMatcher,
	}
}

type MatchRequest struct {
	PestID            uuid.UUID
	SeverityID        uuid.UUID
	GrowthStageID     uuid.UUID
	SymptomIDs        []uuid.UUID
	CNNConfidence     float64
	MinimumBayesScore float64
}

type MatchCandidate struct {
	Rule              domain.ExpertRule
	MatchedSymptoms   []uuid.UUID
	UnmatchedSymptoms []uuid.UUID
	MatchedWeight     float64
	MatchRatio        float64

	// InputCoverageRatio mencakup gejala terpilih yang cocok langsung dengan
	// rule maupun bukti lapangan valid untuk hama, fase, dan tingkat serangan.
	// Hal ini penting karena satu tanda identitas dapat berlaku lintas tingkat
	// serangan walaupun tidak diulang pada setiap rule severity.
	InputCoverageRatio           float64
	CompatibleInputCoverageRatio float64
	InputSymptomCount            int
	CompatibleSymptoms           []uuid.UUID
	HasIdentityEvidence          bool
	HasDamageEvidence            bool
	HasGrowthStageConflict       bool
	HasSeverityConflict          bool
	ConflictingSymptoms          []uuid.UUID
	EvidenceCoverageRatio        float64

	BayesianScore    float64
	SemanticScore    float64
	FinalProbability float64
	Confidence       float64
	Reasoning        []string
}

func (s *MatcherService) FindMatches(ctx context.Context, req MatchRequest) ([]MatchCandidate, error) {
	rules, err := s.repo.FindActiveRulesByPestID(ctx, req.PestID)
	if err != nil {
		return nil, err
	}

	// Katalog ini dibangun dari seluruh rule hama yang sama. Dengan begitu,
	// bukti identitas yang bersifat lintas severity (contoh: embun madu pada WBC)
	// tetap dapat dipakai untuk menguatkan rule berat meskipun gejala tersebut
	// hanya dicantumkan pada rule ringan/sedang di basis pengetahuan lama.
	symptomCatalog := buildPestSymptomCatalog(rules)
	if catalogRepo, ok := s.repo.(PestSymptomCatalogRepository); ok {
		symptoms, catalogErr := catalogRepo.FindSymptomsByPestID(ctx, req.PestID)
		if catalogErr != nil {
			return nil, catalogErr
		}
		mergePestSymptomsIntoCatalog(symptomCatalog, symptoms)
	}
	results := make([]MatchCandidate, 0)

	for _, expertRule := range rules {
		if req.SeverityID != uuid.Nil && expertRule.SeverityID != req.SeverityID {
			continue
		}
		if req.GrowthStageID != uuid.Nil && expertRule.GrowthStageID != req.GrowthStageID {
			continue
		}

		candidate, err := s.buildCandidate(ctx, expertRule, req)
		if err != nil {
			return nil, err
		}

		s.applyCompatibleFieldEvidence(&candidate, req, symptomCatalog)
		// Fakta fase dan tingkat serangan merupakan kondisi wajib. Jika pengguna
		// memilih gejala terstruktur yang secara eksplisit bertentangan dengan
		// salah satu fakta tersebut, sistem menahan keputusan daripada mengabaikan
		// gejala yang kontradiktif dan memilih rule lain.
		if candidate.HasGrowthStageConflict || candidate.HasSeverityConflict {
			continue
		}
		if len(candidate.CompatibleSymptoms) == 0 {
			continue
		}
		if !s.hasSufficientFieldEvidence(expertRule, candidate) {
			continue
		}

		minimumMatchScore := s.effectiveMinimumMatchScore(
			expertRule.MinimumMatchScore,
			req,
			candidate,
		)
		if candidate.FinalProbability < minimumMatchScore {
			continue
		}
		if candidate.BayesianScore < req.MinimumBayesScore {
			continue
		}

		results = append(results, candidate)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].FinalProbability > results[j].FinalProbability
	})
	return results, nil
}

func mergePestSymptomsIntoCatalog(catalog map[uuid.UUID]domain.Symptom, symptoms []domain.Symptom) {
	for _, symptom := range symptoms {
		if symptom.ID == uuid.Nil {
			continue
		}
		catalog[symptom.ID] = symptom
	}
}

func buildPestSymptomCatalog(rules []domain.ExpertRule) map[uuid.UUID]domain.Symptom {
	catalog := make(map[uuid.UUID]domain.Symptom)
	for _, expertRule := range rules {
		for _, item := range expertRule.Symptoms {
			if item.SymptomID == uuid.Nil {
				continue
			}
			symptom := item.Symptom
			if symptom.ID == uuid.Nil {
				symptom.ID = item.SymptomID
			}
			catalog[item.SymptomID] = symptom
		}
	}
	return catalog
}

func (s *MatcherService) buildCandidate(ctx context.Context, expertRule domain.ExpertRule, req MatchRequest) (MatchCandidate, error) {
	inputMap := make(map[uuid.UUID]struct{})
	for _, item := range req.SymptomIDs {
		if item != uuid.Nil {
			inputMap[item] = struct{}{}
		}
	}

	matched := make([]uuid.UUID, 0)
	unmatched := make([]uuid.UUID, 0)
	ruleSymptoms := make([]uuid.UUID, 0)
	totalWeight := 0.0
	matchedWeight := 0.0
	reasoning := make([]string, 0)

	for _, item := range expertRule.Symptoms {
		weight := item.Weight
		if weight <= 0 {
			weight = item.Symptom.DefaultWeight
		}
		if weight <= 0 {
			weight = 1
		}

		totalWeight += weight
		ruleSymptoms = append(ruleSymptoms, item.SymptomID)
		if _, ok := inputMap[item.SymptomID]; ok {
			matched = append(matched, item.SymptomID)
			matchedWeight += weight
			reasoning = append(reasoning, "Gejala cocok langsung dengan rule terpilih.")
			continue
		}
		unmatched = append(unmatched, item.SymptomID)
	}

	matchRatio := 0.0
	if totalWeight > 0 {
		matchRatio = clampProbability(matchedWeight / totalWeight)
	}

	inputCoverageRatio := 0.0
	if len(inputMap) > 0 {
		inputCoverageRatio = float64(len(matched)) / float64(len(inputMap))
	}

	bayes := s.calculateBayesianProbability(matchRatio, req.CNNConfidence, expertRule.ConfidenceScore)

	// Gejala berasal dari pilihan terstruktur, sehingga semantic matcher tidak
	// boleh menaikkan keputusan di luar ID gejala yang telah dinormalisasi.
	_ = ctx
	_ = ruleSymptoms
	semanticScore := 0.0
	final := s.calculateHybridProbability(matchRatio, inputCoverageRatio, 0, bayes, semanticScore, req.CNNConfidence)
	confidence := s.calculateConfidence(final)

	return MatchCandidate{
		Rule:               expertRule,
		MatchedSymptoms:    matched,
		UnmatchedSymptoms:  unmatched,
		MatchedWeight:      matchedWeight,
		MatchRatio:         matchRatio,
		InputCoverageRatio: inputCoverageRatio,
		InputSymptomCount:  len(inputMap),
		BayesianScore:      bayes,
		SemanticScore:      semanticScore,
		FinalProbability:   final,
		Confidence:         confidence,
		Reasoning:          reasoning,
	}, nil
}

func (s *MatcherService) applyCompatibleFieldEvidence(candidate *MatchCandidate, req MatchRequest, catalog map[uuid.UUID]domain.Symptom) {
	if candidate == nil {
		return
	}

	inputMap := make(map[uuid.UUID]struct{})
	for _, id := range req.SymptomIDs {
		if id != uuid.Nil {
			inputMap[id] = struct{}{}
		}
	}

	compatible := make([]uuid.UUID, 0, len(inputMap))
	hasIdentity := false
	hasDamage := false

	for id := range inputMap {
		symptom, ok := catalog[id]
		if !ok {
			continue
		}

		kind := fieldEvidenceKind(symptom)
		if kind == "" {
			continue
		}

		stageMatches := metadataValueMatches(
			symptom.GrowthStage,
			candidate.Rule.GrowthStage.Name,
			normalizeGrowthStageValue,
		)
		severityMatches := metadataValueMatches(
			symptom.Severity,
			candidate.Rule.Severity.Name,
			normalizeSeverityValue,
		)

		if !stageMatches {
			candidate.HasGrowthStageConflict = true
			candidate.ConflictingSymptoms = append(candidate.ConflictingSymptoms, id)
			continue
		}
		if !severityMatches {
			candidate.HasSeverityConflict = true
			candidate.ConflictingSymptoms = append(candidate.ConflictingSymptoms, id)
			continue
		}

		compatible = append(compatible, id)
		switch kind {
		case "identity":
			hasIdentity = true
		case "damage":
			hasDamage = true
		}
	}

	coverage := 0.0
	if len(inputMap) > 0 {
		coverage = float64(len(compatible)) / float64(len(inputMap))
	}
	if coverage > candidate.InputCoverageRatio {
		candidate.InputCoverageRatio = coverage
	}

	evidenceCoverage := 0.0
	if hasIdentity {
		evidenceCoverage += 0.5
	}
	if hasDamage {
		evidenceCoverage += 0.5
	}

	candidate.CompatibleSymptoms = compatible
	candidate.CompatibleInputCoverageRatio = coverage
	candidate.HasIdentityEvidence = hasIdentity
	candidate.HasDamageEvidence = hasDamage
	candidate.EvidenceCoverageRatio = evidenceCoverage
	candidate.FinalProbability = s.calculateHybridProbability(
		candidate.MatchRatio,
		candidate.InputCoverageRatio,
		candidate.EvidenceCoverageRatio,
		candidate.BayesianScore,
		candidate.SemanticScore,
		req.CNNConfidence,
	)
	candidate.Confidence = s.calculateConfidence(candidate.FinalProbability)

	if hasIdentity {
		candidate.Reasoning = append(candidate.Reasoning, "Tanda keberadaan hama sesuai dengan hama terdeteksi dan fase tanaman.")
	}
	if hasDamage {
		candidate.Reasoning = append(candidate.Reasoning, "Gejala kerusakan sesuai dengan fase dan tingkat serangan yang dipilih.")
	}
}

func fieldEvidenceKind(symptom domain.Symptom) string {
	role := strings.ToLower(strings.TrimSpace(symptom.RuleRole))
	typeName := strings.ToLower(strings.TrimSpace(symptom.SymptomType))
	if role == "identity" || typeName == "identity" {
		return "identity"
	}
	if role == "damage" || role == "severity_anchor" || typeName == "damage" || typeName == "severity_anchor" {
		return "damage"
	}
	return ""
}

func metadataValueMatches(raw string, expected string, normalizer func(string) string) bool {
	expected = normalizer(expected)
	if expected == "" {
		return true
	}

	values := splitMetadataValues(raw, normalizer)
	if len(values) == 0 {
		return true
	}
	for _, value := range values {
		if value == expected || value == "semua" || value == "all" {
			return true
		}
	}
	return false
}

func splitMetadataValues(raw string, normalizer func(string) string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '|' || r == ',' || r == ';' || r == '/'
	})
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, part := range parts {
		value := normalizer(part)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeGrowthStageValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(value, "generatif") || strings.Contains(value, "reproduktif"):
		return "generatif"
	case strings.Contains(value, "vegetatif"):
		return "vegetatif"
	case value == "semua" || value == "all":
		return value
	default:
		return value
	}
}

func normalizeSeverityValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "berat", "tinggi", "parah", "high":
		return "berat"
	case "sedang", "medium", "moderate":
		return "sedang"
	case "ringan", "rendah", "low":
		return "ringan"
	case "semua", "all":
		return value
	default:
		return value
	}
}

func (s *MatcherService) calculateBayesianProbability(matchRatio float64, cnnConfidence float64, ruleConfidence float64) float64 {
	// Field dipertahankan untuk kompatibilitas respons lama. Nilainya tidak
	// boleh ditingkatkan oleh CNN karena keputusan tetap bergantung pada bukti lapang.
	_ = cnnConfidence
	_ = ruleConfidence
	return clampProbability(matchRatio)
}

func (s *MatcherService) calculateHybridProbability(matchRatio float64, inputCoverageRatio float64, evidenceCoverageRatio float64, bayes float64, semantic float64, cnn float64) float64 {
	// Fase dan severity telah menjadi filter wajib. Skor akhir menggabungkan:
	// 40% kecocokan langsung terhadap rule, 40% validitas seluruh input terhadap
	// hama/fase/severity, dan 20% kelengkapan dua jenis bukti lapangan.
	_ = bayes
	_ = semantic
	_ = cnn
	return clampProbability(
		(matchRatio * 0.40) +
			(inputCoverageRatio * 0.40) +
			(evidenceCoverageRatio * 0.20),
	)
}

func (s *MatcherService) effectiveMinimumMatchScore(ruleMinimum float64, req MatchRequest, candidate MatchCandidate) float64 {
	minimum := ruleMinimum
	if minimum <= 0 {
		minimum = 0.60
	}
	return math.Max(0.55, minimum)
}

func (s *MatcherService) hasSufficientFieldEvidence(_ domain.ExpertRule, candidate MatchCandidate) bool {
	return candidate.HasIdentityEvidence && candidate.HasDamageEvidence
}

func candidateHasStrongSeverityAnchor(candidate MatchCandidate) bool {
	if len(candidate.MatchedSymptoms) == 0 {
		return false
	}

	matched := make(map[uuid.UUID]struct{}, len(candidate.MatchedSymptoms))
	for _, id := range candidate.MatchedSymptoms {
		matched[id] = struct{}{}
	}

	for _, item := range candidate.Rule.Symptoms {
		if _, ok := matched[item.SymptomID]; !ok {
			continue
		}

		if isStrongSeverityAnchorSymptom(item.Symptom) {
			return true
		}
	}

	return false
}

func candidateHasMediumOrStrongFieldEvidence(candidate MatchCandidate) bool {
	if candidateHasStrongSeverityAnchor(candidate) {
		return true
	}

	if len(candidate.MatchedSymptoms) == 0 {
		return false
	}

	matched := make(map[uuid.UUID]struct{}, len(candidate.MatchedSymptoms))
	for _, id := range candidate.MatchedSymptoms {
		matched[id] = struct{}{}
	}

	for _, item := range candidate.Rule.Symptoms {
		if _, ok := matched[item.SymptomID]; !ok {
			continue
		}

		if isMediumOrStrongFieldEvidenceSymptom(item.Symptom) {
			return true
		}
	}

	return false
}

func isMediumOrStrongFieldEvidenceSymptom(symptom domain.Symptom) bool {
	if isStrongSeverityAnchorSymptom(symptom) {
		return true
	}

	role := strings.ToLower(strings.TrimSpace(symptom.RuleRole))
	symptomType := strings.ToLower(strings.TrimSpace(symptom.SymptomType))
	severity := strings.ToLower(strings.TrimSpace(symptom.Severity))

	combined := strings.ToLower(strings.TrimSpace(strings.Join([]string{
		symptom.Name,
		symptom.OriginalName,
		symptom.Description,
		symptom.ExpertNote,
		severity,
		symptomType,
		role,
	}, " ")))

	if combined == "" {
		return false
	}

	// Identity evidence proves pest presence, but does not prove field damage.
	if role == "identity" || symptomType == "identity" {
		return false
	}

	if !(role == "severity_anchor" || role == "damage" || role == "supporting" ||
		symptomType == "severity_anchor" || symptomType == "damage" || symptomType == "supporting") {
		return false
	}

	if strings.Contains(severity, "sedang") ||
		strings.Contains(severity, "medium") ||
		strings.Contains(severity, "moderate") ||
		strings.Contains(severity, "berat") ||
		strings.Contains(severity, "tinggi") ||
		strings.Contains(severity, "high") {
		return true
	}

	mediumKeywords := []string{
		"pertumbuhan anakan melambat",
		"anakan melambat",
		"produksi anakan menurun",
		"anakan menurun",
		"menguning",
		"pangkal batang coklat",
		"pangkal batang cokelat",
		"layu meski air cukup",
		"layu",
		"kerusakan mulai nyata",
		"sebagian rumpun",
		"mengering sebagian",
	}

	for _, keyword := range mediumKeywords {
		if strings.Contains(combined, keyword) {
			return true
		}
	}

	return false
}

func isStrongSeverityAnchorSymptom(symptom domain.Symptom) bool {
	role := strings.ToLower(strings.TrimSpace(symptom.RuleRole))
	symptomType := strings.ToLower(strings.TrimSpace(symptom.SymptomType))
	severity := strings.ToLower(strings.TrimSpace(symptom.Severity))

	combined := strings.ToLower(strings.TrimSpace(strings.Join([]string{
		symptom.Name,
		symptom.OriginalName,
		symptom.Description,
		symptom.ExpertNote,
		severity,
		symptomType,
		role,
	}, " ")))

	if combined == "" {
		return false
	}

	if role == "identity" || symptomType == "identity" {
		return false
	}

	if !(role == "severity_anchor" || role == "damage" || symptomType == "severity_anchor" || symptomType == "damage") {
		return false
	}

	if strings.Contains(severity, "berat") || strings.Contains(severity, "tinggi") || strings.Contains(severity, "high") {
		return true
	}

	heavyKeywords := []string{
		"hopperburn",
		"tanaman seperti terbakar",
		"seperti terbakar",
		"terbakar",
		"gosong",
		"mati serempak",
		"rumpun mati",
		"banyak rumpun mati",
		"gagal panen",
		"hampa tinggi",
		"kehilangan hasil",
		"banyak malai putih",
		"titik tumbuh mati",
	}

	for _, keyword := range heavyKeywords {
		if strings.Contains(combined, keyword) {
			return true
		}
	}

	return false
}

func normalizedRuleSeverity(rule domain.ExpertRule) string {
	name := strings.ToLower(strings.TrimSpace(rule.Severity.Name))
	combined := name

	switch {
	case strings.Contains(combined, "berat") || strings.Contains(combined, "tinggi") || strings.Contains(combined, "high") || strings.Contains(combined, "parah") || strings.Contains(combined, "ber"):
		return "high"
	case strings.Contains(combined, "sedang") || strings.Contains(combined, "medium") || strings.Contains(combined, "moderate") || strings.Contains(combined, "sed"):
		return "medium"
	default:
		return "low"
	}
}

func (s *MatcherService) calculateConfidence(score float64) float64 {

	switch {

	case score >= 0.90:
		return 1.0

	case score >= 0.75:
		return 0.85

	case score >= 0.60:
		return 0.70

	case score >= 0.45:
		return 0.55

	default:
		return 0.30
	}
}

func clampProbability(value float64) float64 {

	return math.Max(
		0,
		math.Min(
			1,
			value,
		),
	)
}
