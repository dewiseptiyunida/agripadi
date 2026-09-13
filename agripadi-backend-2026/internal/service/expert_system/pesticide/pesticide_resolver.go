package pesticide

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gustian305/backend/internal/domain"
)

type PesticideRepository interface {
	FindByPestID(ctx context.Context, pestID uuid.UUID) ([]domain.Pesticide, error)
}

type ResolverService struct {
	repo          PesticideRepository
	safetyService *SafetyService
}

const (
	targetFitWeight     = 0.25
	ingredientFitWeight = 0.20
	doseQualityWeight   = 0.15
	severityFitWeight   = 0.15
	safetyWeight        = 0.15
	growthStageWeight   = 0.07
	dataQualityWeight   = 0.03
)

func NewResolverService(
	repo PesticideRepository,
	safetyServices ...*SafetyService,
) *ResolverService {

	safetyService := NewSafetyService()

	if len(safetyServices) > 0 &&
		safetyServices[0] != nil {

		safetyService = safetyServices[0]
	}

	return &ResolverService{
		repo:          repo,
		safetyService: safetyService,
	}
}

type ResolveRequest struct {
	PestID      uuid.UUID
	PestName    string
	Severity    string
	GrowthStage string
	TopK        int
}

type ScoreBreakdown struct {
	TargetFit     float64
	IngredientFit float64
	DoseQuality   float64
	SeverityFit   float64
	Safety        float64
	GrowthStage   float64
	DataQuality   float64
	WeightedScore float64
}

type Recommendation struct {
	Pesticide         domain.Pesticide
	MatchScore        float64
	Confidence        float64
	RegulationAllowed bool
	ScoreBreakdown    ScoreBreakdown
	Reasoning         []string
}

func (s *ResolverService) Resolve(ctx context.Context, req ResolveRequest) ([]Recommendation, error) {

	items, err :=
		s.repo.FindByPestID(
			ctx,
			req.PestID,
		)

	if err != nil {
		return nil, err
	}

	results := make(
		[]Recommendation,
		0,
	)

	for _, item := range items {

		regulationAllowed :=
			s.checkRegistrationStatus(
				item,
			)

		if !regulationAllowed {
			continue
		}

		// Hard filter: pestisida hanya layak masuk ranking utama jika memiliki
		// dosis yang spesifik untuk OPT/hama target. Ini menjaga prinsip
		// tepat sasaran dan tepat dosis.
		if !hasTargetDosage(item, req.PestID) {
			continue
		}

		// Diagnosis lapang tidak boleh menampilkan perlakuan benih/pratanam
		// sebagai rekomendasi utama. Produk FS tetap boleh tersimpan di database
		// untuk kebutuhan pencegahan, tetapi bukan untuk tanaman yang sudah
		// bergejala di lapangan.
		if !fieldDiagnosisAllowedForProduct(item, req) {
			continue
		}

		breakdown :=
			s.calculateScoreBreakdown(
				item,
				req,
			)

		score := breakdown.WeightedScore

		confidence :=
			s.calculateConfidence(
				score,
			)

		results = append(
			results,
			Recommendation{
				Pesticide:         item,
				MatchScore:        score,
				Confidence:        confidence,
				RegulationAllowed: regulationAllowed,
				ScoreBreakdown:    breakdown,
				Reasoning:         s.buildReasoning(item, req, breakdown),
			},
		)
	}

	sort.Slice(
		results,
		func(i, j int) bool {
			if results[i].MatchScore != results[j].MatchScore {
				return results[i].MatchScore >
					results[j].MatchScore
			}

			return strings.ToLower(results[i].Pesticide.ProductName) <
				strings.ToLower(results[j].Pesticide.ProductName)
		},
	)

	topK := req.TopK

	if topK <= 0 {
		topK = 5
	}

	if len(results) > topK {

		results = results[:topK]
	}

	return results, nil
}

func fieldDiagnosisAllowedForProduct(item domain.Pesticide, req ResolveRequest) bool {
	formulation := strings.ToUpper(strings.TrimSpace(item.Formulation))
	if formulation == "" {
		return true
	}

	if isPrePlantFormulation(formulation) && !isPrePlantGrowthStage(req.GrowthStage) {
		return false
	}

	for _, dosage := range item.Dosages {
		doseText := strings.ToLower(strings.TrimSpace(dosage.DoseRaw + " " + dosage.DoseUnit))
		if strings.Contains(doseText, "kg benih") || strings.Contains(doseText, "benih") {
			if !isPrePlantGrowthStage(req.GrowthStage) {
				return false
			}
		}
	}

	return true
}

func isPrePlantFormulation(formulation string) bool {
	switch strings.ToUpper(strings.TrimSpace(formulation)) {
	case "FS", "WS", "DS":
		return true
	default:
		return false
	}
}

func isPrePlantGrowthStage(stage string) bool {
	normalized := strings.ToLower(strings.TrimSpace(stage))
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.ReplaceAll(normalized, "-", " ")
	normalized = strings.Join(strings.Fields(normalized), " ")

	switch normalized {
	case "pratanam", "pra tanam", "sebelum tanam", "sebelum semai", "persemaian", "semai", "seedling":
		return true
	default:
		return false
	}
}

func (s *ResolverService) calculateScoreBreakdown(item domain.Pesticide, req ResolveRequest) ScoreBreakdown {

	breakdown := ScoreBreakdown{
		TargetFit:     targetFitScore(item, req.PestID),
		IngredientFit: ingredientFitScore(item, req.PestName),
		DoseQuality:   doseQualityScore(item, req.PestID),
		SeverityFit: severityFitScore(
			item,
			req.PestID,
			req.Severity,
		),
		Safety:      s.safetyFitScore(item),
		GrowthStage: growthStageFitScore(item, req.GrowthStage),
		DataQuality: dataQualityScore(item, req.PestID),
	}

	weightedScore := breakdown.TargetFit*targetFitWeight +
		breakdown.IngredientFit*ingredientFitWeight +
		breakdown.DoseQuality*doseQualityWeight +
		breakdown.SeverityFit*severityFitWeight +
		breakdown.Safety*safetyWeight +
		breakdown.GrowthStage*growthStageWeight +
		breakdown.DataQuality*dataQualityWeight

	// Pembatasan hanya diterapkan jika kelak tersedia data label keselamatan
	// terverifikasi yang benar-benar menunjukkan risiko tinggi. Nilai netral
	// karena data belum tersedia tidak boleh dianggap sebagai risiko tinggi.
	if breakdown.Safety <= 0.20 {
		weightedScore = math.Min(weightedScore, 0.50)
	} else if breakdown.Safety <= 0.30 {
		weightedScore = math.Min(weightedScore, 0.70)
	}

	breakdown.WeightedScore = roundScore(weightedScore)

	breakdown.TargetFit = roundScore(breakdown.TargetFit)
	breakdown.IngredientFit = roundScore(breakdown.IngredientFit)
	breakdown.DoseQuality = roundScore(breakdown.DoseQuality)
	breakdown.SeverityFit = roundScore(breakdown.SeverityFit)
	breakdown.Safety = roundScore(breakdown.Safety)
	breakdown.GrowthStage = roundScore(breakdown.GrowthStage)
	breakdown.DataQuality = roundScore(breakdown.DataQuality)

	return breakdown
}

func (s *ResolverService) safetyFitScore(item domain.Pesticide) float64 {
	if s == nil || s.safetyService == nil {
		return 0.80
	}

	return s.safetyService.ScoreProduct(item)
}

func (s *ResolverService) checkRegistrationStatus(item domain.Pesticide) bool {

	now := time.Now()

	if item.RegisteredAt.After(now) {
		return false
	}
	if item.ExpiredAt != nil && item.ExpiredAt.Before(now) {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(item.Commodity), "padi")
}

func (s *ResolverService) calculateConfidence(score float64) float64 {

	switch {

	case score >= 0.90:
		return 1.0

	case score >= 0.80:
		return 0.90

	case score >= 0.70:
		return 0.80

	case score >= 0.60:
		return 0.70

	default:
		return 0.50
	}
}

func (s *ResolverService) buildReasoning(item domain.Pesticide, req ResolveRequest, breakdown ScoreBreakdown) []string {
	_ = item
	_ = req
	_ = breakdown

	return []string{
		"Produk tercatat untuk tanaman padi dan hama sasaran serta memiliki petunjuk dosis pada data yang tersedia.",
		"Urutan pilihan terutama berdasarkan kesesuaian sasaran, kelengkapan dosis, formulasi, dan masa berlaku pendaftaran.",
		"Tingkat serangan tidak digunakan untuk menaikkan dosis. Gunakan pestisida hanya bila hasil pengamatan menunjukkan perlu dan selalu ikuti label resmi produk.",
	}
}

func ingredientFitScore(item domain.Pesticide, pestName string) float64 {
	_ = pestName

	// Produk yang masuk ke fungsi ini sudah lolos filter komoditas padi,
	// hama sasaran, status pendaftaran, dan dosis sasaran. Tanpa tabel
	// efektivitas bahan aktif yang tervalidasi pakar, sistem tidak boleh
	// memberi nilai berdasarkan daftar bahan aktif buatan sendiri.
	if hasNamedIngredient(item.Ingredients) {
		return 1.00
	}

	return 0.00
}

func hasTargetDosage(item domain.Pesticide, pestID uuid.UUID) bool {
	_, ok := targetDosage(item, pestID)
	return ok
}

func targetDosage(item domain.Pesticide, pestID uuid.UUID) (domain.PesticideDosage, bool) {
	for _, dosage := range item.Dosages {
		if dosage.PestID == pestID {
			return dosage, true
		}
	}

	return domain.PesticideDosage{}, false
}

func targetFitScore(item domain.Pesticide, pestID uuid.UUID) float64 {
	if hasTargetDosage(item, pestID) {
		return 1.00
	}

	return 0.00
}

func doseQualityScore(item domain.Pesticide, pestID uuid.UUID) float64 {
	dosage, ok := targetDosage(item, pestID)
	if !ok {
		return 0.00
	}

	hasRaw := strings.TrimSpace(dosage.DoseRaw) != ""
	hasUnit := strings.TrimSpace(dosage.DoseUnit) != ""
	hasRange := dosage.MinDose != nil &&
		dosage.MaxDose != nil &&
		*dosage.MinDose > 0 &&
		*dosage.MaxDose > 0

	if hasRaw && hasUnit && hasRange {
		return 1.00
	}

	if (hasRaw && hasUnit) || hasRange {
		return 0.80
	}

	if hasRaw || hasUnit {
		return 0.50
	}

	return 0.00
}

func severityFitScore(item domain.Pesticide, pestID uuid.UUID, severity string) float64 {
	_ = severity

	// Dataset label tidak memuat aturan dosis atau formulasi khusus berdasarkan
	// tingkat serangan. Beri nilai netral dan jangan mengubah dosis otomatis.
	if hasTargetDosage(item, pestID) {
		return 0.50
	}

	return 0.00
}

func growthStageFitScore(item domain.Pesticide, growthStage string) float64 {
	_ = growthStage

	// Produk pratanam telah disaring terpisah. Untuk produk lapang, dataset
	// belum memuat izin fase spesifik pada label, sehingga nilai dibuat netral.
	if strings.TrimSpace(item.Formulation) != "" {
		return 0.50
	}

	return 0.40
}

func dataQualityScore(item domain.Pesticide, pestID uuid.UUID) float64 {
	missing := 0

	if strings.TrimSpace(item.ProductName) == "" {
		missing++
	}

	if strings.TrimSpace(item.Formulation) == "" {
		missing++
	}

	if !hasNamedIngredient(item.Ingredients) {
		missing++
	}

	dosage, ok := targetDosage(item, pestID)
	if !ok || strings.TrimSpace(dosage.DoseRaw) == "" || strings.TrimSpace(dosage.DoseUnit) == "" {
		missing++
	}

	if item.RegisteredAt.IsZero() || item.ExpiredAt == nil {
		missing++
	}

	switch missing {
	case 0:
		return 1.00
	case 1:
		return 0.70
	default:
		return 0.40
	}
}

func hasNamedIngredient(ingredients []domain.PesticideIngredient) bool {
	for _, ingredient := range ingredients {
		if strings.TrimSpace(ingredient.Name) != "" {
			return true
		}
	}

	return false
}

func inStringSet(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}

	return false
}

func roundScore(value float64) float64 {
	if value > 0 {
		value = math.Round(value*100) / 100
	}

	if value > 1 {
		return 1
	}

	if value < 0 {
		return 0
	}

	return value
}

func severityLabel(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "high", "berat", "tinggi":
		return "berat"
	case "medium", "sedang":
		return "sedang"
	case "low", "ringan", "rendah":
		return "ringan"
	default:
		return severity
	}
}
