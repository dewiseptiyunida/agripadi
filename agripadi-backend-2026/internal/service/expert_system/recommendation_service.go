package expertSystem

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gustian305/backend/internal/domain"
	"github.com/gustian305/backend/internal/dto"
	"github.com/gustian305/backend/internal/service/expert_system/diagnose"
	"github.com/gustian305/backend/internal/service/expert_system/llm"
	"github.com/gustian305/backend/internal/service/expert_system/pesticide"
	"github.com/gustian305/backend/internal/service/expert_system/rule"
	"github.com/gustian305/backend/logger"
)

var llmDoseMentionPattern = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*(?:ml|mili?liter|l|liter|g|gram|kg|kilogram|ha|hektare)\b`)

type PestLookupRepository interface {
	FindPestByLabelName(ctx context.Context, labelName string) (*domain.Pest, error)
}

type RuleReferenceRepository interface {
	FindSeverityByName(ctx context.Context, name string) (*domain.Severity, error)
	FindGrowthStageByName(ctx context.Context, name string) (*domain.GrowthStage, error)
}

type ApplicationTimingLookupRepository interface {
	FindPesticideApplicationTimings(ctx context.Context, pestID uuid.UUID, pestName string, activeIngredients []string, formulation string, growthStage string, severity string) ([]domain.PesticideApplicationTiming, error)
}

type RecommendationService struct {
	pestRepo              PestLookupRepository
	ruleRefRepo           RuleReferenceRepository
	applicationTimingRepo ApplicationTimingLookupRepository
	ruleService           *rule.Service
	pesticideResolver     *pesticide.ResolverService
	safetyService         *pesticide.SafetyService
	llmService            *llm.RecommendationService
}

func NewRecommendationService(
	pestRepo PestLookupRepository,
	ruleRefRepo RuleReferenceRepository,
	applicationTimingRepo ApplicationTimingLookupRepository,
	ruleService *rule.Service,
	pesticideResolver *pesticide.ResolverService,
	safetyService *pesticide.SafetyService,
	llmService *llm.RecommendationService,
) *RecommendationService {

	return &RecommendationService{
		pestRepo:              pestRepo,
		ruleRefRepo:           ruleRefRepo,
		applicationTimingRepo: applicationTimingRepo,
		ruleService:           ruleService,
		pesticideResolver:     pesticideResolver,
		safetyService:         safetyService,
		llmService:            llmService,
	}
}

type DiagnosisRecommendationResult struct {
	Pest *domain.Pest

	DetectionConfidence float64
	RuleConfidence      float64
	RuleCode            string

	RuleResult      *rule.ExecuteResponse
	Recommendations []dto.PesticideRecommendationResponse

	Symptoms    []string
	Severity    string
	GrowthStage string

	LLMSummary         string
	LLMSelectionReason string
	LLMSeverityAction  string
	LLMMessage         string
	LLMModel           string
	LLMFallback        bool
}

func (s *RecommendationService) ResolveForSession(ctx context.Context, session *domain.ExpertSession) (*DiagnosisRecommendationResult, error) {
	startedAt := time.Now()
	operation := "ResolveForSession"

	if session != nil {
		logger.Request(
			"expert_system.recommendation",
			operation,
			slog.String("session_id", session.ID.String()),
			slog.String("conversation_id", session.ConversationID.String()),
			slog.String("detected_label", session.DetectedLabel),
			slog.Float64("detected_confidence", session.DetectedConfidence),
			slog.String("severity", session.Severity),
			slog.String("growth_stage", session.GrowthStage),
		)
	}

	if s == nil ||
		s.pestRepo == nil ||
		s.pesticideResolver == nil ||
		session == nil {

		logger.Response(
			"expert_system.recommendation",
			operation,
			startedAt,
			slog.Bool("configured", false),
		)
		return &DiagnosisRecommendationResult{}, nil
	}

	pest, err := s.pestRepo.FindPestByLabelName(
		ctx,
		session.DetectedLabel,
	)

	if err != nil {
		logger.Failure("expert_system.recommendation", operation, startedAt, err)
		return nil, err
	}

	if pest == nil {
		logger.Response(
			"expert_system.recommendation",
			operation,
			startedAt,
			slog.Bool("pest_found", false),
			slog.String("detected_label", session.DetectedLabel),
		)
		return &DiagnosisRecommendationResult{}, nil
	}

	ruleResult, err := s.resolveRule(
		ctx,
		session,
		pest.ID,
	)

	if err != nil {
		logger.Failure("expert_system.recommendation", operation, startedAt, err)
		return nil, err
	}

	recommendations := []pesticide.Recommendation{}

	if hasStrongRuleResult(ruleResult) {
		var err error
		recommendations, err = s.pesticideResolver.Resolve(
			ctx,
			pesticide.ResolveRequest{
				PestID:      pest.ID,
				PestName:    pest.Name,
				Severity:    toEngineSeverity(session.Severity),
				GrowthStage: session.GrowthStage,
				TopK:        10,
			},
		)

		if err != nil {
			logger.Failure("expert_system.recommendation", operation, startedAt, err)
			return nil, err
		}
	}

	symptoms := extractSymptomSummary(
		[]byte(session.Symptoms),
	)

	ruleCode := ""

	if ruleResult != nil &&
		ruleResult.BestMatch != nil {

		ruleCode = extractRuleCode(ruleResult)
	}

	result := &DiagnosisRecommendationResult{
		Pest: pest,

		DetectionConfidence: session.DetectedConfidence,
		RuleConfidence:      ruleConfidence(ruleResult),
		RuleCode:            ruleCode,

		RuleResult: ruleResult,

		Recommendations: s.toDTORecommendations(
			ctx,
			pest.ID,
			pest.Name,
			recommendations,
			toEngineSeverity(session.Severity),
			session.GrowthStage,
		),

		Symptoms: symptoms,

		Severity:    session.Severity,
		GrowthStage: session.GrowthStage,
	}

	s.applyLLMNarrative(
		ctx,
		session,
		result,
	)

	bestRule := extractRuleCode(
		ruleResult,
	)

	fallbackRequired := false

	if ruleResult != nil {
		fallbackRequired =
			ruleResult.FallbackRequired
	}

	logger.Response(
		"expert_system.recommendation",
		operation,
		startedAt,
		slog.String("pest_id", pest.ID.String()),
		slog.String("pest_name", pest.Name),
		slog.String("best_rule_code", bestRule),
		slog.Bool("fallback_required", fallbackRequired),
		slog.Int("recommendation_count", len(result.Recommendations)),
		slog.Bool("llm_message_generated", strings.TrimSpace(result.LLMMessage) != ""),
	)

	return result, nil
}

func (s *RecommendationService) applyLLMNarrative(ctx context.Context, session *domain.ExpertSession, result *DiagnosisRecommendationResult) {
	startedAt := time.Now()
	operation := "GenerateLLMRecommendationMessage"

	if session == nil || result == nil {
		return
	}

	// Selalu siapkan narasi aman agar tampilan rekomendasi tetap mudah dipahami
	// ketika penyedia LLM sedang gagal. Ketika LLM berhasil, setiap bagian akan
	// ditimpa dengan hasil Qwen yang sudah disanitasi dan divalidasi.
	result.LLMSummary = sanitizeSeverityTermNarrative(buildDiagnosisNarrative(result))
	result.LLMSelectionReason = sanitizeSeverityTermNarrative(buildSelectionReasonNarrative(result))
	result.LLMSeverityAction = sanitizeSeverityTermNarrative(buildSeverityActionNarrative(result))
	result.LLMMessage = strings.TrimSpace(strings.Join([]string{
		"2. Ringkasan Diagnosis\n" + result.LLMSummary,
		"4. Alasan Pemilihan Pestisida\n" + result.LLMSelectionReason,
		"5. Tindakan Pengendalian Berdasarkan Severity\n" + result.LLMSeverityAction,
	}, "\n\n"))
	result.LLMModel = "deterministic_fallback"
	result.LLMFallback = true

	if s == nil || s.llmService == nil {
		logger.Response(
			"expert_system.recommendation",
			operation,
			startedAt,
			slog.Bool("configured", false),
			slog.Bool("fallback_used", true),
		)
		return
	}

	pesticides := make([]llm.PesticideLLMData, 0, len(result.Recommendations))
	for index, item := range result.Recommendations {
		if index >= 5 {
			break
		}

		ingredientNames := make([]string, 0, len(item.Ingredients))
		for _, ingredient := range item.Ingredients {
			name := strings.TrimSpace(ingredient.Name)
			if name != "" {
				ingredientNames = append(ingredientNames, name)
			}
		}

		pesticides = append(pesticides, llm.PesticideLLMData{
			ProductName:       safeRecommendationName(item),
			Dose:              emptyFallback(item.Dosage.DoseRaw, "Ikuti label produk"),
			ActiveIngredient:  strings.Join(ingredientNames, ", "),
			Formulation:       item.Formulation,
			ApplicationTiming: llmApplicationTimingText(item.ApplicationTiming),
		})
	}

	// Narasi LLM hanya dipanggil ketika sistem sudah memiliki rekomendasi yang
	// valid. Jika rekomendasi ditahan oleh rule engine, fallback deterministik di
	// atas menjelaskan alasan tanpa membuat produk atau dosis baru.
	if len(result.Recommendations) == 0 {
		return
	}

	pestName := session.DetectedLabel
	if result.Pest != nil {
		pestName = result.Pest.Name
	}

	var growthStagePtr *string
	if strings.TrimSpace(session.GrowthStage) != "" {
		growthStage := session.GrowthStage
		growthStagePtr = &growthStage
	}

	response, err := s.llmService.Generate(
		ctx,
		llm.RecommendationRequest{
			PestName:            pestName,
			DetectionLabel:      session.DetectedLabel,
			DetectionConfidence: session.DetectedConfidence,
			Symptoms:            result.Symptoms,
			RuleCode:            result.RuleCode,
			RuleConfidence:      result.RuleConfidence,
			Pesticides:          pesticides,
			Severity:            session.Severity,
			Language:            llm.RecommendationLanguageID,
			FarmerExperience:    llm.FarmerExperienceBeginner,
			GrowthStage:         growthStagePtr,
		},
	)
	if err != nil {
		logger.Failure(
			"expert_system.recommendation",
			operation,
			startedAt,
			err,
			slog.Bool("fallback_used", true),
		)
		return
	}

	message := sanitizeSeverityTermNarrative(strings.TrimSpace(response.Message))
	policyResult := llm.ValidateRecommendationNarrative(
		message,
		buildRecommendationPolicyContext(session, result),
	)
	if !policyResult.Accepted {
		logger.Response(
			"expert_system.recommendation",
			operation,
			startedAt,
			slog.Bool("llm_output_accepted", false),
			slog.Bool("fallback_used", true),
			slog.Int("policy_violation_count", len(policyResult.Violations)),
		)
		return
	}

	sections := parseLLMNarrativeSections(message)

	acceptedSummary := false
	if summary := sanitizeLLMNarrativeForResult(result, sections.summary); summary != "" {
		result.LLMSummary = summary
		acceptedSummary = true
	}
	if reason := sanitizeLLMNarrativeForResult(result, sections.selectionReason); reason != "" {
		result.LLMSelectionReason = reason
	}
	if action := sanitizeLLMNarrativeForResult(result, sections.severityAction); action != "" {
		result.LLMSeverityAction = action
	}

	// Ringkasan adalah bagian yang tampil paling utama pada aplikasi. Jika
	// ringkasan Qwen tidak lolos validasi, pertahankan fallback deterministik dan
	// tandai sumbernya dengan jujur untuk kebutuhan audit.
	if !acceptedSummary {
		logger.Response(
			"expert_system.recommendation",
			operation,
			startedAt,
			slog.Bool("llm_output_accepted", false),
			slog.Bool("fallback_used", true),
		)
		return
	}

	result.LLMMessage = message
	result.LLMModel = s.llmService.ModelName()
	if strings.TrimSpace(result.LLMModel) == "" {
		result.LLMModel = "qwen/qwen3.6-27b"
	}
	result.LLMFallback = false

	logger.Response(
		"expert_system.recommendation",
		operation,
		startedAt,
		slog.Int("message_length", len(result.LLMMessage)),
		slog.Bool("llm_output_accepted", true),
		slog.Bool("fallback_used", false),
	)
}

func buildRecommendationPolicyContext(session *domain.ExpertSession, result *DiagnosisRecommendationResult) llm.RecommendationPolicyContext {
	ctx := llm.RecommendationPolicyContext{
		MaxWords:          220,
		RequirePHT:        true,
		RequireMonitoring: true,
		RequireRotation:   true,
		RequireTopProduct: true,
	}

	if session != nil {
		ctx.PestName = session.DetectedLabel
		ctx.Severity = session.Severity
		ctx.GrowthStage = session.GrowthStage
	}
	if result == nil {
		return ctx
	}
	if result.Pest != nil && strings.TrimSpace(result.Pest.Name) != "" {
		ctx.PestName = result.Pest.Name
	}

	for _, item := range result.Recommendations {
		if productName := strings.TrimSpace(safeRecommendationName(item)); productName != "" {
			ctx.AllowedProducts = append(ctx.AllowedProducts, productName)
		}
		for _, ingredient := range item.Ingredients {
			if name := strings.TrimSpace(ingredient.Name); name != "" {
				ctx.AllowedIngredients = append(ctx.AllowedIngredients, name)
			}
		}
	}

	ctx.KnownProducts = append([]string(nil), ctx.AllowedProducts...)
	ctx.KnownIngredients = append([]string(nil), ctx.AllowedIngredients...)
	return ctx
}

func sanitizeLLMNarrativeForResult(result *DiagnosisRecommendationResult, value string) string {
	value = sanitizeSeverityTermNarrative(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	currentPest := ""
	if result != nil && result.Pest != nil {
		currentPest = canonicalPestName(result.Pest.Name)
	}
	lower := strings.ToLower(value)
	for _, pestName := range []string{"penggerek batang", "walang sangit", "wereng batang cokelat"} {
		if pestName != currentPest && strings.Contains(lower, pestName) {
			return ""
		}
	}

	// Angka dosis hanya ditampilkan oleh komponen deterministik dari data label.
	// Bagian narasi LLM dibuang bila memuat takaran agar model tidak dapat
	// menyisipkan atau mengubah dosis secara bebas.
	if llmDoseMentionPattern.MatchString(value) {
		return ""
	}

	return value
}

func llmApplicationTimingText(timing *dto.PesticideApplicationTimingResponse) string {
	if timing == nil {
		return ""
	}

	return formatApplicationTiming(*timing)
}

func (s *RecommendationService) resolveApplicationTiming(
	ctx context.Context,
	pestID uuid.UUID,
	pestName string,
	ingredients []dto.PesticideIngredientResponse,
	formulation string,
	growthStage string,
	severity string,
) *dto.PesticideApplicationTimingResponse {
	if s == nil || s.applicationTimingRepo == nil {
		return nil
	}

	activeIngredients := make([]string, 0, len(ingredients))
	for _, ingredient := range ingredients {
		name := strings.TrimSpace(ingredient.Name)
		if name == "" {
			continue
		}
		activeIngredients = append(activeIngredients, name)
	}

	timings, err := s.applicationTimingRepo.FindPesticideApplicationTimings(
		ctx,
		pestID,
		pestName,
		activeIngredients,
		formulation,
		growthStage,
		severity,
	)
	if err != nil {
		logger.Failure("expert_system.recommendation", "FindPesticideApplicationTimings", time.Now(), err)
		return nil
	}

	if len(timings) == 0 {
		return nil
	}

	return toApplicationTimingDTO(timings[0])
}

func toApplicationTimingDTO(timing domain.PesticideApplicationTiming) *dto.PesticideApplicationTimingResponse {
	return &dto.PesticideApplicationTimingResponse{
		ApplicationContext:     strings.TrimSpace(timing.ApplicationContext),
		FieldDiagnosisAllowed:  timing.FieldDiagnosisAllowed,
		ApplicationMethod:      strings.TrimSpace(timing.ApplicationMethod),
		ApplicationTarget:      strings.TrimSpace(timing.ApplicationTarget),
		ApplicationTrigger:     strings.TrimSpace(timing.ApplicationTrigger),
		TimingWindow:           strings.TrimSpace(timing.TimingWindow),
		TimingInstruction:      strings.TrimSpace(timing.TimingInstruction),
		WaterManagement:        strings.TrimSpace(timing.WaterManagement),
		WeatherCondition:       strings.TrimSpace(timing.WeatherCondition),
		PreharvestIntervalNote: strings.TrimSpace(timing.PreharvestIntervalNote),
		DisplayWarning:         strings.TrimSpace(timing.DisplayWarning),
		ReferenceTitle:         strings.TrimSpace(timing.ReferenceTitle),
		ReferenceInstitution:   strings.TrimSpace(timing.ReferenceInstitution),
		ReferenceYear:          strings.TrimSpace(timing.ReferenceYear),
		ReferenceURL:           strings.TrimSpace(timing.ReferenceURL),
		ReferenceNote:          strings.TrimSpace(timing.ReferenceNote),
	}
}

type llmNarrativeSections struct {
	summary         string
	selectionReason string
	severityAction  string
}

func parseLLMNarrativeSections(message string) llmNarrativeSections {
	lines := strings.Split(
		strings.TrimSpace(message),
		"\n",
	)

	var sections llmNarrativeSections
	current := ""
	buffers := map[string][]string{
		"summary":          {},
		"selection_reason": {},
		"severity_action":  {},
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		switch {
		case strings.HasPrefix(lower, "2.") ||
			strings.Contains(lower, "ringkasan diagnosis"):
			current = "summary"
			continue

		case strings.HasPrefix(lower, "4.") ||
			strings.Contains(lower, "alasan pemilihan"):
			current = "selection_reason"
			continue

		case strings.HasPrefix(lower, "5.") ||
			strings.Contains(lower, "tindakan pengendalian"):
			current = "severity_action"
			continue
		}

		if current == "" || trimmed == "" {
			continue
		}

		buffers[current] = append(buffers[current], trimmed)
	}

	sections.summary = cleanNarrativeLines(buffers["summary"])
	sections.selectionReason = cleanNarrativeLines(buffers["selection_reason"])
	sections.severityAction = cleanNarrativeLines(buffers["severity_action"])

	if sections.summary == "" &&
		sections.selectionReason == "" &&
		sections.severityAction == "" {

		sections.summary = strings.TrimSpace(message)
	}

	return sections
}

func cleanNarrativeLines(lines []string) string {
	cleaned := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "- ")
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		cleaned = append(cleaned, line)
	}

	return strings.Join(cleaned, "\n")
}

func (s *RecommendationService) resolveRule(ctx context.Context, session *domain.ExpertSession, pestID uuid.UUID) (*rule.ExecuteResponse, error) {

	if s.ruleService == nil {
		return nil, nil
	}

	severityID := uuid.Nil
	growthStageID := uuid.Nil

	if s.ruleRefRepo != nil {
		severity, err := s.ruleRefRepo.FindSeverityByName(
			ctx,
			session.Severity,
		)

		if err != nil {
			return nil, err
		}

		if severity != nil {
			severityID = severity.ID
		}

		growthStage, err := s.ruleRefRepo.FindGrowthStageByName(
			ctx,
			session.GrowthStage,
		)

		if err != nil {
			return nil, err
		}

		if growthStage != nil {
			growthStageID = growthStage.ID
		}
	}

	return s.ruleService.Execute(
		ctx,
		rule.ExecuteRequest{
			PestID:            pestID,
			SeverityID:        severityID,
			GrowthStageID:     growthStageID,
			SymptomIDs:        extractMatchedSymptomIDs(json.RawMessage(session.Symptoms)),
			CNNConfidence:     session.DetectedConfidence,
			MinimumBayesScore: 0,
			SeverityLevel:     toEngineSeverity(session.Severity),
		},
	)
}

func (s *RecommendationService) toDTORecommendations(
	ctx context.Context,
	pestID uuid.UUID,
	pestName string,
	recommendations []pesticide.Recommendation,
	severity string,
	growthStage string,
) []dto.PesticideRecommendationResponse {

	results := make(
		[]dto.PesticideRecommendationResponse,
		0,
		len(recommendations),
	)

	seenIngredientFamilies := make(map[string]struct{})
	seenRecommendationIDs := make(map[uuid.UUID]struct{})
	seenExactBasis := make(map[string]struct{})

	appendRecommendation := func(
		recommendation pesticide.Recommendation,
		allowDuplicateIngredientFamily bool,
	) bool {

		if len(results) >= 5 {
			return false
		}

		item := recommendation.Pesticide

		if _, exists := seenRecommendationIDs[item.ID]; exists {
			return false
		}

		ingredientFamily := ingredientFamilyKey(item.Ingredients)

		if ingredientFamily != "" && !allowDuplicateIngredientFamily {
			if _, exists := seenIngredientFamilies[ingredientFamily]; exists {
				return false
			}
		}

		ingredients := toIngredientDTOs(item.Ingredients)
		activeIngredient := formatIngredients(ingredients)
		productName := strings.TrimSpace(item.ProductName)
		if productName == "" {
			productName = buildPesticideDisplayName(activeIngredient, item.Formulation, item.PesticideType)
		}
		displayName := buildPesticideProductDisplayName(productName, activeIngredient, item.Formulation, item.PesticideType)
		recommendationBasis := buildRecommendationBasis(activeIngredient, item.Formulation)
		exactBasisKey := strings.ToLower(strings.TrimSpace(recommendationBasis))
		applicationTiming := s.resolveApplicationTiming(
			ctx,
			pestID,
			pestName,
			ingredients,
			item.Formulation,
			growthStage,
			severity,
		)

		// Tetap cegah duplikasi persis bahan aktif + formulasi. Namun jangan jadikan
		// konsentrasi berbeda sebagai alasan untuk memenuhi Top 5 jika bahan aktifnya
		// sama; diversifikasi bahan aktif ditangani oleh ingredientFamilyKey.
		if exactBasisKey != "" {
			if _, exists := seenExactBasis[exactBasisKey]; exists {
				return false
			}
		}

		dosage := selectDosage(
			pestID,
			item.Dosages,
			severity,
		)

		safety := dto.PesticideSafetyResponse{}

		if s.safetyService != nil {
			safetyResult := s.safetyService.Analyze(
				recommendation,
			)

			safety = dto.PesticideSafetyResponse{
				ToxicityClass:      stringPtr(safetyResult.ToxicityClass),
				ReentryInterval:    stringPtr(safetyResult.ReentryInterval),
				PreHarvestInterval: stringPtr(safetyResult.PreHarvestInterval),
				MaxApplication:     stringPtr(safetyResult.MaxApplication),
				MixingWarning:      safetyResult.Warnings,
			}
		}

		results = append(
			results,
			dto.PesticideRecommendationResponse{
				PesticideID:         item.ID,
				ProductName:         productName,
				DisplayName:         displayName,
				RecommendationBasis: recommendationBasis,
				TradeNameHidden:     false,
				TradeNamePolicy:     "Nama produk ditampilkan sebagai contoh produk terdaftar, bukan sebagai promosi. Penggunaan tetap harus mengikuti label resmi dan arahan petugas lapangan bila diperlukan.",
				PesticideType:       item.PesticideType,
				Formulation:         item.Formulation,
				Ingredients:         ingredients,
				Dosage: dto.PesticideDosageResponse{
					DoseRaw:  dosage.DoseRaw,
					MinDose:  dosage.MinDose,
					MaxDose:  dosage.MaxDose,
					DoseUnit: dosage.DoseUnit,
				},
				ApplicationTiming: applicationTiming,
				Safety:            safety,
				ScoreBreakdown: dto.PesticideScoreBreakdownResponse{
					TargetFit:     recommendation.ScoreBreakdown.TargetFit,
					IngredientFit: recommendation.ScoreBreakdown.IngredientFit,
					DoseQuality:   recommendation.ScoreBreakdown.DoseQuality,
					SeverityFit:   recommendation.ScoreBreakdown.SeverityFit,
					Safety:        recommendation.ScoreBreakdown.Safety,
					GrowthStage:   recommendation.ScoreBreakdown.GrowthStage,
					DataQuality:   recommendation.ScoreBreakdown.DataQuality,
					WeightedScore: recommendation.ScoreBreakdown.WeightedScore,
				},
				MatchScore:               recommendation.MatchScore,
				RecommendationConfidence: recommendation.Confidence,
				Reason:                   buildRecommendationReason(productName, ingredients, pestName, severity, growthStage),
			},
		)

		seenRecommendationIDs[item.ID] = struct{}{}

		if ingredientFamily != "" {
			seenIngredientFamilies[ingredientFamily] = struct{}{}
		}

		if exactBasisKey != "" {
			seenExactBasis[exactBasisKey] = struct{}{}
		}

		return true
	}

	// Tahap pertama: isi Top 5 dengan keluarga bahan aktif yang berbeda agar
	// output tidak menampilkan dua produk berbahan aktif sama secara berurutan
	// seperti Dimehipo 525 dan Dimehipo 500.
	for _, recommendation := range recommendations {
		if len(results) >= 5 {
			break
		}

		appendRecommendation(recommendation, false)
	}

	// Tahap kedua: jika database tidak menyediakan cukup bahan aktif berbeda,
	// isi sisa slot dengan kandidat terbaik berikutnya. Ini menjaga output tetap
	// berisi sampai 5 rekomendasi tanpa mengorbankan ranking utama.
	for _, recommendation := range recommendations {
		if len(results) >= 5 {
			break
		}

		appendRecommendation(recommendation, true)
	}

	return results
}

func selectDosage(
	pestID uuid.UUID,
	dosages []domain.PesticideDosage,
	severity string,
) domain.PesticideDosage {

	for _, dosage := range dosages {
		if dosage.PestID == pestID {
			return adjustDosageBySeverity(dosage, severity)
		}
	}

	if len(dosages) > 0 {
		return adjustDosageBySeverity(dosages[0], severity)
	}

	return domain.PesticideDosage{
		DoseRaw:  "Ikuti dosis pada label produk",
		DoseUnit: "label",
	}
}

func adjustDosageBySeverity(
	dosage domain.PesticideDosage,
	severity string,
) domain.PesticideDosage {

	// Tingkat serangan tidak boleh dipakai untuk menaikkan atau menurunkan
	// dosis secara otomatis. Dosis tetap mengikuti rentang pada label produk.
	_ = severity

	if strings.TrimSpace(dosage.DoseRaw) == "" &&
		dosage.MinDose != nil &&
		dosage.MaxDose != nil &&
		*dosage.MinDose > 0 &&
		*dosage.MaxDose > 0 {

		if *dosage.MinDose == *dosage.MaxDose {
			dosage.DoseRaw = formatDoseValue(*dosage.MinDose)
		} else {
			dosage.DoseRaw = fmt.Sprintf(
				"%s-%s",
				formatDoseValue(*dosage.MinDose),
				formatDoseValue(*dosage.MaxDose),
			)
		}
	}

	return dosage
}

func formatDoseValue(value float64) string {
	text := fmt.Sprintf("%.4f", value)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")

	if text == "" {
		return "0"
	}

	return text
}

func toIngredientDTOs(
	ingredients []domain.PesticideIngredient,
) []dto.PesticideIngredientResponse {

	results := make(
		[]dto.PesticideIngredientResponse,
		0,
		len(ingredients),
	)

	for _, ingredient := range ingredients {
		results = append(
			results,
			dto.PesticideIngredientResponse{
				Name:             ingredient.Name,
				ConcentrationRaw: ingredient.ConcentrationRaw,
			},
		)
	}

	return results
}

func FormatRecommendationMessage(
	result *DiagnosisRecommendationResult,
) string {

	if result == nil {
		return strings.TrimSpace(`Hasil pemeriksaan: Data belum tersedia
Tingkat kecocokan: Belum tersedia
Kondisi serangan: Belum tersedia
Fase padi: Belum tersedia
Ringkasan: Data pemeriksaan belum tersedia.
Pilihan bahan aktif: Belum tersedia.
Waktu penggunaan: Belum tersedia.
Cara penggunaan: Belum tersedia.
Langkah berikutnya: Ulangi pemeriksaan dengan foto dan gejala yang lebih jelas.
Catatan keamanan: Ikuti arahan penyuluh atau petugas lapangan sebelum menggunakan pestisida.`)
	}

	pestName := "hama terdeteksi"
	if result.Pest != nil && strings.TrimSpace(result.Pest.Name) != "" {
		pestName = result.Pest.Name
	}

	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Hasil pemeriksaan: %s\n", pestName))
	builder.WriteString(fmt.Sprintf("Tingkat kecocokan: %s\n", formatDiagnosisConfidenceForUser(result)))
	builder.WriteString(fmt.Sprintf("Kondisi serangan: %s\n", displayFallback(result.Severity, "tidak tercatat")))
	builder.WriteString(fmt.Sprintf("Fase padi: %s\n\n", displayFallback(result.GrowthStage, "tidak tercatat")))

	builder.WriteString("Penjelasan sederhana:\n")
	builder.WriteString(compactText(preferredFarmerSummary(result), 420, 3))
	builder.WriteString("\n\n")

	builder.WriteString("Pilihan bahan aktif:\n")
	builder.WriteString(formatActiveIngredientRecommendationForUser(result))
	builder.WriteString("\n\n")

	builder.WriteString("Waktu penggunaan:\n")
	builder.WriteString(formatApplicationTimeForUser(result.Recommendations))
	builder.WriteString("\n\n")

	builder.WriteString("Cara penggunaan:\n")
	builder.WriteString(formatApplicationMethodSectionForUser(result.Recommendations))
	builder.WriteString("\n\n")

	builder.WriteString("Langkah berikutnya:\n")
	builder.WriteString(formatFollowUpActionForUser(result))
	builder.WriteString("\n\n")

	builder.WriteString("Catatan keamanan:\n")
	builder.WriteString(formatSafetyNoteForUser(result))

	return strings.TrimSpace(builder.String())
}

func formatDiagnosisConfidenceForUser(result *DiagnosisRecommendationResult) string {
	if result == nil {
		return "Belum tersedia"
	}

	// Jangan memakai keyakinan CNN sebagai pengganti kecocokan rule. Foto dapat
	// memiliki confidence tinggi, tetapi hasil lapangan tetap belum kuat apabila
	// bukti identitas, kerusakan, fase, dan severity tidak membentuk rule valid.
	if !hasStrongRuleResult(result.RuleResult) {
		if result.DetectionConfidence > 0 {
			return "Perlu verifikasi gejala"
		}
		return "Belum tersedia"
	}

	score := result.RuleConfidence
	switch {
	case score >= 0.80:
		return "Kuat"
	case score >= 0.60:
		return "Cukup"
	case score > 0:
		return "Perlu diperiksa ulang"
	default:
		return "Belum tersedia"
	}
}

func formatActiveIngredientRecommendationForUser(result *DiagnosisRecommendationResult) string {
	if result == nil || len(result.Recommendations) == 0 {
		return "Belum ditampilkan karena gejala lapangan belum cukup kuat. Lengkapi tanda keberadaan hama, kerusakan tanaman, tingkat serangan, dan fase tanaman terlebih dahulu."
	}

	lines := make([]string, 0, 4)
	seen := map[string]struct{}{}

	for _, item := range result.Recommendations {
		ingredient := strings.TrimSpace(formatIngredients(item.Ingredients))
		if ingredient == "" {
			continue
		}

		key := strings.ToLower(ingredient)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		line := "- " + ingredient
		if dosage := strings.TrimSpace(formatDosage(item.Dosage)); dosage != "" {
			line += "; dosis: " + dosage
		}
		lines = append(lines, line)

		if len(lines) >= 2 {
			break
		}
	}

	if len(lines) == 0 {
		return "- Bahan aktif mengikuti rekomendasi pada label produk dan arahan petugas lapangan."
	}

	return strings.Join(lines, "\n")
}

func formatApplicationTimeForUser(recommendations []dto.PesticideRecommendationResponse) string {
	timing := firstApplicationTiming(recommendations)
	if timing == nil {
		return "Waktu aplikasi khusus belum tersedia untuk kondisi ini. Gunakan petunjuk pada label resmi produk atau arahan petugas lapangan."
	}

	lines := make([]string, 0, 5)

	if value := strings.TrimSpace(timing.TimingWindow); value != "" {
		lines = append(lines, "- Waktu: "+ensureTrailingPeriod(value))
	}

	if value := strings.TrimSpace(timing.WeatherCondition); value != "" {
		lines = append(lines, "- Kondisi cuaca: "+ensureTrailingPeriod(value))
	}

	if value := strings.TrimSpace(timing.TimingInstruction); value != "" {
		instruction := formatTimingInstructionForUser(value)
		if instruction != "" {
			lines = append(lines, "- Instruksi waktu: "+instruction)
		}
	}

	if warning := strings.TrimSpace(timing.DisplayWarning); warning != "" {
		lines = append(lines, "- Peringatan: "+ensureTrailingPeriod(warning))
	}

	if len(lines) == 0 {
		return "Waktu aplikasi belum tercatat lengkap. Ikuti label resmi produk atau arahan petugas lapangan."
	}

	return strings.Join(lines, "\n")
}

func formatTimingInstructionForUser(value string) string {
	parts := make([]string, 0, 2)
	seen := make(map[string]struct{})

	for _, sentence := range splitGuidanceSentences(value) {
		sentence = sanitizeGuidanceSentence(sentence)
		if sentence == "" || isReferenceLikeGuidance(sentence) || isGlobalPHTGuidance(sentence) {
			continue
		}

		key := strings.ToLower(sentence)
		if _, exists := seen[key]; exists {
			continue
		}

		parts = append(parts, ensureTrailingPeriod(sentence))
		seen[key] = struct{}{}

		if len(parts) >= 2 {
			break
		}
	}

	return strings.Join(parts, " ")
}

func formatApplicationMethodSectionForUser(recommendations []dto.PesticideRecommendationResponse) string {
	timing := firstApplicationTiming(recommendations)
	if timing == nil {
		return "Cara aplikasi belum tercatat lengkap. Ikuti label resmi produk dan arahan petugas lapangan."
	}

	parts := make([]string, 0, 4)

	if method := formatApplicationMethodForUser(timing.ApplicationMethod); method != "" {
		parts = append(parts, ensureTrailingPeriod(method))
	}

	if target := strings.TrimSpace(timing.ApplicationTarget); target != "" {
		parts = append(parts, "Sasaran: "+ensureTrailingPeriod(target))
	}

	if instruction := strings.TrimSpace(timing.TimingInstruction); instruction != "" {
		for _, sentence := range splitGuidanceSentences(instruction) {
			sentence = sanitizeGuidanceSentence(sentence)
			if sentence == "" || isReferenceLikeGuidance(sentence) || isGlobalPHTGuidance(sentence) {
				continue
			}
			parts = append(parts, ensureTrailingPeriod(sentence))
			break
		}
	}

	if len(parts) == 0 {
		return "Aplikasikan sesuai formulasi, dosis, sasaran, dan petunjuk pada label resmi produk."
	}

	return compactText(strings.Join(parts, " "), 280, 0)
}

func formatFollowUpActionForUser(result *DiagnosisRecommendationResult) string {
	if result == nil {
		return "Ulangi pemantauan dan konsultasikan ke petugas lapangan bila gejala belum jelas."
	}

	base := "Amati ulang 3-5 hari setelah tindakan."

	switch toEngineSeverity(result.Severity) {
	case "high":
		base += " Jika gejala meluas, segera konsultasikan ke penyuluh atau petugas lapangan."
	case "medium":
		base += " Pantau rumpun sekitar dan lakukan evaluasi ulang bila populasi hama meningkat."
	default:
		base += " Lanjutkan pemantauan rutin dan utamakan sanitasi serta pengendalian selektif."
	}

	base += " Terapkan PHT dan rotasi bahan aktif pada aplikasi berikutnya."
	return base
}

func formatSafetyNoteForUser(result *DiagnosisRecommendationResult) string {
	items := []string{
		"Gunakan APD saat mencampur dan mengaplikasikan pestisida.",
		"Ikuti dosis, interval aplikasi, dan larangan pada label resmi produk.",
		"Jangan mencampur pestisida sembarangan tanpa arahan petugas.",
	}

	if result != nil && len(result.Recommendations) > 0 {
		top := result.Recommendations[0]
		if top.Safety.ReentryInterval != nil && strings.TrimSpace(*top.Safety.ReentryInterval) != "" {
			items = append(items, "Interval masuk kembali: "+ensureTrailingPeriod(*top.Safety.ReentryInterval))
		}
		if top.Safety.PreHarvestInterval != nil && strings.TrimSpace(*top.Safety.PreHarvestInterval) != "" {
			items = append(items, "Interval pra-panen: "+ensureTrailingPeriod(*top.Safety.PreHarvestInterval))
		}
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		lines = append(lines, "- "+ensureTrailingPeriod(item))
	}

	return strings.Join(limitStrings(lines, 4), "\n")
}

func BuildExpertValidationResponse(result *DiagnosisRecommendationResult) *dto.ExpertValidationResponse {
	status, label, note := expertValidationReadiness(result)

	return &dto.ExpertValidationResponse{
		ReadinessStatus:  status,
		ReadinessLabel:   label,
		ValidationTarget: buildExpertValidationTarget(result),
		OverallNote:      note,
		Evidence:         buildExpertValidationEvidence(result),
		Checklist:        buildExpertValidationChecklist(result),
		Warnings:         buildExpertValidationWarnings(result),
		ExpertDecisionOptions: []string{
			"valid",
			"valid_dengan_catatan",
			"perlu_perbaikan",
			"tidak_valid",
		},
		RatingScale: []string{
			"1 = tidak sesuai",
			"2 = kurang sesuai",
			"3 = cukup sesuai",
			"4 = sesuai",
			"5 = sangat sesuai",
		},
		FieldsToReview: []string{
			"nama_hama",
			"gejala_kunci",
			"tingkat_keparahan",
			"fase_pertumbuhan",
			"rule_terpilih",
			"rekomendasi_pestisida",
			"waktu_dan_cara_aplikasi",
			"keamanan_penggunaan",
		},
	}
}

func FormatExpertValidationSection(result *DiagnosisRecommendationResult) string {
	validation := BuildExpertValidationResponse(result)
	if validation == nil {
		return "- Status validasi: perlu data tambahan dari hasil diagnosis."
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("- Status validasi awal: %s.\n", validation.ReadinessLabel))
	builder.WriteString(fmt.Sprintf("- Sasaran validasi: %s.\n", validation.ValidationTarget))
	builder.WriteString(fmt.Sprintf("- Catatan sistem: %s\n", validation.OverallNote))

	if len(validation.Evidence) > 0 {
		builder.WriteString("- Bukti yang perlu dicek pakar:\n")
		for _, item := range validation.Evidence {
			builder.WriteString(fmt.Sprintf("  %s: %s", item.Component, item.SystemValue))
			if strings.TrimSpace(item.Source) != "" {
				builder.WriteString(fmt.Sprintf(" (sumber: %s)", item.Source))
			}
			if strings.TrimSpace(item.Note) != "" {
				builder.WriteString(fmt.Sprintf("; %s", item.Note))
			}
			builder.WriteString(".\n")
		}
	}

	if len(validation.Checklist) > 0 {
		builder.WriteString("- Checklist validasi pakar:\n")
		for idx, item := range validation.Checklist {
			builder.WriteString(fmt.Sprintf("  %d) %s Jawaban sistem: %s.\n", idx+1, item.Question, displayFallback(item.SystemAnswer, "belum tersedia")))
		}
	}

	if len(validation.Warnings) > 0 {
		builder.WriteString("- Peringatan yang harus dikonfirmasi pakar:\n")
		for _, warning := range validation.Warnings {
			warning = strings.TrimSpace(warning)
			if warning == "" {
				continue
			}
			builder.WriteString("  - " + ensureTrailingPeriod(warning) + "\n")
		}
	}

	builder.WriteString("- Keputusan pakar yang disarankan pada form validasi: valid, valid dengan catatan, perlu perbaikan, atau tidak valid.")
	return strings.TrimSpace(builder.String())
}

func expertValidationReadiness(result *DiagnosisRecommendationResult) (string, string, string) {
	if result == nil {
		return "perlu_data", "Perlu data", "Hasil diagnosis belum tersedia untuk divalidasi."
	}

	if result.RuleResult == nil || result.RuleResult.FallbackRequired || result.RuleResult.BestMatch == nil {
		return "ditahan_perlu_verifikasi", "Ditahan untuk verifikasi", "Gejala yang dipilih belum cukup kuat, sehingga tanda keberadaan hama dan kerusakan tanaman perlu diperiksa kembali sebelum pestisida digunakan."
	}

	if hasEvaluationOnlyTiming(result.Recommendations) {
		return "perlu_verifikasi_lapang", "Perlu verifikasi lapang", "Hasil dapat ditinjau oleh pakar, tetapi waktu dan cara aplikasi perlu dikonfirmasi langsung di lapangan."
	}

	return "siap_divalidasi_pakar", "Siap divalidasi pakar", "Hasil diagnosis memiliki data hama, gejala, tingkat serangan, fase tanaman, rekomendasi, dan waktu aplikasi yang dapat diperiksa oleh pakar pertanian."
}

func buildExpertValidationTarget(result *DiagnosisRecommendationResult) string {
	if result == nil {
		return "hasil diagnosis belum tersedia"
	}

	pestName := "hama belum tercatat"
	if result.Pest != nil && strings.TrimSpace(result.Pest.Name) != "" {
		pestName = result.Pest.Name
	}

	return fmt.Sprintf(
		"%s tingkat %s pada fase %s",
		pestName,
		displayFallback(result.Severity, "tidak tercatat"),
		displayFallback(result.GrowthStage, "tidak tercatat"),
	)
}

func buildExpertValidationEvidence(result *DiagnosisRecommendationResult) []dto.ExpertValidationEvidenceResponse {
	items := make([]dto.ExpertValidationEvidenceResponse, 0, 8)
	if result == nil {
		return items
	}

	pestName := "tidak tercatat"
	if result.Pest != nil && strings.TrimSpace(result.Pest.Name) != "" {
		pestName = result.Pest.Name
	}

	items = append(items, dto.ExpertValidationEvidenceResponse{
		Component:   "Deteksi gambar",
		SystemValue: fmt.Sprintf("%s (%.2f%%)", pestName, result.DetectionConfidence*100),
		Source:      "hasil pemeriksaan gambar",
		Note:        "Pakar dapat membandingkan dengan citra tanaman/hama yang diunggah.",
	})

	if len(result.Symptoms) > 0 {
		items = append(items, dto.ExpertValidationEvidenceResponse{
			Component:   "Gejala terpilih",
			SystemValue: strings.Join(result.Symptoms, "; "),
			Source:      "gejala yang dipilih pengguna",
			Note:        "Pastikan gejala benar-benar terlihat di lapangan, bukan hanya dugaan.",
		})
	}

	if result.RuleCode != "" {
		items = append(items, dto.ExpertValidationEvidenceResponse{
			Component:   "Pola gejala terpilih",
			SystemValue: fmt.Sprintf("%s (kecocokan %.2f%%)", result.RuleCode, result.RuleConfidence*100),
			Source:      "pencocokan gejala lapangan",
			Note:        "Pola harus sesuai dengan jenis hama, tingkat serangan, fase tanaman, dan gejala kunci.",
		})
	}

	items = append(items, dto.ExpertValidationEvidenceResponse{
		Component:   "Tingkat keparahan",
		SystemValue: displayFallback(result.Severity, "tidak tercatat"),
		Source:      "pilihan pengguna dan pemeriksaan gejala",
		Note:        "Pakar perlu memeriksa apakah ringan/sedang/berat sesuai kondisi serangan.",
	})

	items = append(items, dto.ExpertValidationEvidenceResponse{
		Component:   "Fase pertumbuhan",
		SystemValue: displayFallback(result.GrowthStage, "tidak tercatat"),
		Source:      "pilihan pengguna dan pemeriksaan gejala",
		Note:        "Fase harus cocok dengan gejala, misalnya sundep vegetatif atau beluk/gabah generatif.",
	})

	if len(result.Recommendations) > 0 {
		top := result.Recommendations[0]
		items = append(items, dto.ExpertValidationEvidenceResponse{
			Component:   "Rekomendasi utama",
			SystemValue: joinNonEmpty("; ", top.ProductName, formatIngredients(top.Ingredients), strings.ToUpper(strings.TrimSpace(top.Formulation))),
			Source:      "data produk terdaftar dan hasil pemeriksaan",
			Note:        "Pakar menilai kesesuaian bahan aktif, formulasi, dosis label, dan kebutuhan rotasi.",
		})

		if top.ApplicationTiming != nil {
			items = append(items, dto.ExpertValidationEvidenceResponse{
				Component:   "Waktu dan cara aplikasi",
				SystemValue: joinNonEmpty("; ", top.ApplicationTiming.TimingWindow, top.ApplicationTiming.ApplicationMethod, top.ApplicationTiming.ApplicationTarget),
				Source:      "panduan waktu aplikasi yang tersedia",
				Note:        "Pakar perlu memastikan timing sesuai kondisi cuaca, fase tanaman, dan label produk.",
			})
		}
	}

	return items
}

func buildExpertValidationChecklist(result *DiagnosisRecommendationResult) []dto.ExpertValidationChecklistResponse {
	pestName := "belum tersedia"
	severity := "belum tersedia"
	growthStage := "belum tersedia"
	symptoms := "belum tersedia"
	recommendation := "belum tersedia"
	timing := "belum tersedia"

	if result != nil {
		if result.Pest != nil && strings.TrimSpace(result.Pest.Name) != "" {
			pestName = result.Pest.Name
		}
		severity = displayFallback(result.Severity, severity)
		growthStage = displayFallback(result.GrowthStage, growthStage)
		if len(result.Symptoms) > 0 {
			symptoms = strings.Join(result.Symptoms, "; ")
		}
		if len(result.Recommendations) > 0 {
			top := result.Recommendations[0]
			recommendation = joinNonEmpty("; ", top.ProductName, formatIngredients(top.Ingredients), strings.ToUpper(strings.TrimSpace(top.Formulation)))
			if top.ApplicationTiming != nil {
				timing = joinNonEmpty("; ", top.ApplicationTiming.TimingWindow, top.ApplicationTiming.TimingInstruction, top.ApplicationTiming.DisplayWarning)
			}
		}
	}

	options := []string{"sesuai", "kurang sesuai", "tidak sesuai", "perlu data tambahan"}
	return []dto.ExpertValidationChecklistResponse{
		{Aspect: "hama", Question: "Apakah hama hasil diagnosis sesuai dengan citra/gejala lapangan?", SystemAnswer: pestName, ExpectedExpertInput: options},
		{Aspect: "gejala", Question: "Apakah gejala yang dipakai sistem merupakan gejala kunci hama tersebut?", SystemAnswer: symptoms, ExpectedExpertInput: options},
		{Aspect: "keparahan", Question: "Apakah tingkat keparahan yang dipilih sesuai luas dan intensitas serangan?", SystemAnswer: severity, ExpectedExpertInput: options},
		{Aspect: "fase", Question: "Apakah fase pertumbuhan sesuai dengan bagian tanaman yang terserang?", SystemAnswer: growthStage, ExpectedExpertInput: options},
		{Aspect: "rekomendasi", Question: "Apakah rekomendasi pestisida/bahan aktif sesuai hama, fase, dan prinsip PHT?", SystemAnswer: recommendation, ExpectedExpertInput: options},
		{Aspect: "timing", Question: "Apakah waktu, sasaran, dan cara aplikasi sesuai kondisi lapang dan label produk?", SystemAnswer: timing, ExpectedExpertInput: options},
		{Aspect: "keamanan", Question: "Apakah catatan APD, rotasi bahan aktif, masa tunggu, dan keamanan lingkungan sudah memadai?", SystemAnswer: "APD, label, PHT, rotasi bahan aktif, dan masa tunggu dicantumkan bila tersedia", ExpectedExpertInput: options},
	}
}

func buildExpertValidationWarnings(result *DiagnosisRecommendationResult) []string {
	warnings := make([]string, 0, 6)
	if result == nil {
		return warnings
	}

	if result.RuleResult == nil || result.RuleResult.FallbackRequired || result.RuleResult.BestMatch == nil {
		warnings = append(warnings, "Gejala belum cukup kuat; jangan gunakan pestisida sebelum tanda keberadaan hama dan kerusakan tanaman diperiksa kembali.")
	}

	if hasEvaluationOnlyTiming(result.Recommendations) {
		warnings = append(warnings, "Waktu aplikasi masih perlu diperiksa di lapangan dan tidak boleh dianggap sebagai perintah otomatis.")
	}

	for _, item := range result.Recommendations {
		if item.ApplicationTiming == nil {
			continue
		}
		if warning := strings.TrimSpace(item.ApplicationTiming.DisplayWarning); warning != "" {
			warnings = appendUniqueValidationWarning(warnings, warning)
		}
		if note := strings.TrimSpace(item.ApplicationTiming.PreharvestIntervalNote); note != "" {
			warnings = appendUniqueValidationWarning(warnings, note)
		}
	}

	warnings = appendUniqueValidationWarning(warnings, "Nama produk hanya menjadi contoh produk terdaftar, bukan promosi. Ikuti label resmi dan arahan petugas lapangan.")
	warnings = appendUniqueValidationWarning(warnings, "Utamakan pengamatan, kebersihan lahan, dan cara pengendalian nonkimia. Bila pestisida diperlukan, gunakan bahan aktif secara bergantian sesuai arahan petugas agar hama tidak cepat kebal.")
	return warnings
}

func hasEvaluationOnlyTiming(recommendations []dto.PesticideRecommendationResponse) bool {
	for _, item := range recommendations {
		if item.ApplicationTiming == nil {
			continue
		}
		context := strings.ToLower(strings.TrimSpace(item.ApplicationTiming.ApplicationContext))
		if strings.Contains(context, "evaluasi") || !item.ApplicationTiming.FieldDiagnosisAllowed {
			return true
		}
	}
	return false
}

func appendUniqueValidationWarning(warnings []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return warnings
	}
	key := strings.ToLower(value)
	for _, existing := range warnings {
		if strings.ToLower(strings.TrimSpace(existing)) == key {
			return warnings
		}
	}
	return append(warnings, value)
}

func preferredFarmerSummary(result *DiagnosisRecommendationResult) string {
	if result == nil {
		return buildDiagnosisNarrative(result)
	}
	if summary := strings.TrimSpace(result.LLMSummary); summary != "" {
		return summary
	}
	return buildDiagnosisNarrative(result)
}

func buildDiagnosisNarrative(result *DiagnosisRecommendationResult) string {
	if result == nil {
		return "Pemeriksaan selesai berdasarkan informasi yang tersedia."
	}

	if result.RuleResult == nil || result.RuleResult.FallbackRequired || result.RuleResult.BestMatch == nil {
		pestName := "hama terdeteksi"
		if result.Pest != nil && strings.TrimSpace(result.Pest.Name) != "" {
			pestName = result.Pest.Name
		}

		return fmt.Sprintf(
			"Gambar mengarah pada kemungkinan %s, tetapi gejala yang dipilih belum cukup kuat. Periksa kembali tanda keberadaan hama dan kerusakan tanaman di lahan sebelum menentukan tindakan.",
			pestName,
		)
	}

	// Final diagnosis untuk tampilan aplikasi dibuat deterministik dari domain/database:
	// - nama hama dari tabel pests,
	// - severity dan fase dari sesi diagnosis yang divalidasi ke tabel severity/growth_stages,
	// - rule dari expert_rules,
	// - gejala dari relasi rule/symptoms.
	// LLM tidak dipakai pada ringkasan akhir agar hasil tidak keluar dari data domain.
	pestName := "hama terdeteksi"
	if result.Pest != nil && strings.TrimSpace(result.Pest.Name) != "" {
		pestName = result.Pest.Name
	}

	return fmt.Sprintf(
		"Hasil pemeriksaan mengarah pada serangan %s tingkat %s pada fase %s berdasarkan gambar dan gejala yang dipilih. Cocokkan kembali dengan kondisi lahan sebelum melakukan pengendalian.",
		pestName,
		displayFallback(result.Severity, "tidak tercatat"),
		displayFallback(result.GrowthStage, "tidak tercatat"),
	)
}

func buildSelectionReasonNarrative(result *DiagnosisRecommendationResult) string {
	if result == nil {
		return "Belum ada rekomendasi pestisida yang dapat dijelaskan."
	}

	if len(result.Recommendations) == 0 {
		return "Rekomendasi pestisida belum ditampilkan karena gejala lapangan belum cukup kuat. Tambahkan tanda keberadaan hama dan tanda kerusakan tanaman yang lebih jelas."
	}

	top := result.Recommendations[0]
	activeIngredient := formatIngredients(top.Ingredients)
	if strings.TrimSpace(activeIngredient) == "" {
		activeIngredient = "bahan aktif yang tercatat pada data resmi yang tersedia"
	}

	productName := strings.TrimSpace(top.ProductName)
	if productName == "" {
		productName = "produk pestisida terpilih"
	}

	return fmt.Sprintf(
		"%s ditampilkan karena tercatat untuk hama sasaran pada tanaman padi dan memiliki informasi dosis serta formulasi. Tingkat serangan %s dan fase %s digunakan untuk memeriksa kecocokan kondisi lapangan, bukan untuk mengubah dosis. Produk ini memiliki bahan aktif %s. Penyebutan nama produk hanya sebagai identitas data, bukan promosi; penggunaan tetap mengikuti label resmi dan arahan petugas lapangan bila diperlukan.",
		productName,
		displayFallback(result.Severity, "tidak tercatat"),
		displayFallback(result.GrowthStage, "tidak tercatat"),
		displayFallback(activeIngredient, "bahan aktif terdaftar"),
	)
}

func buildSeverityActionNarrative(result *DiagnosisRecommendationResult) string {
	if result == nil {
		return "Lakukan pemantauan ulang dan ikuti rekomendasi pengendalian sesuai kondisi lahan."
	}

	if result.RuleResult == nil || result.RuleResult.FallbackRequired || result.RuleResult.BestMatch == nil {
		return "Lakukan verifikasi lapangan: periksa keberadaan hama target, gejala khas pada batang/malai/bulir, serta luas serangan pada beberapa titik sampel. Hindari aplikasi pestisida sampai diagnosis lebih kuat."
	}

	narrative := ""

	// Tindakan lapangan dibuat deterministik. Narasi dari model bahasa tidak
	// boleh mengubah keputusan, dosis, waktu, atau cara aplikasi.
	switch toEngineSeverity(result.Severity) {
	case "high":
		narrative = "Serangan tergolong berat. Batasi tindakan pada area terdampak, periksa kembali setelah 3-5 hari, dan hubungi penyuluh atau petugas lapangan bila gejala terus meluas."
	case "medium":
		narrative = "Serangan tergolong sedang. Tangani rumpun atau petakan yang menunjukkan gejala, pantau jumlah hama, dan periksa kembali beberapa hari kemudian."
	default:
		narrative = "Serangan tergolong ringan. Utamakan pemantauan rutin, kebersihan lahan, dan tindakan selektif bila gejala mulai bertambah."
	}

	narrative = sanitizeApplicationNarrative(
		narrative,
		result.Recommendations,
	)

	narrative = ensurePHTRotationNote(
		narrative,
		result.Recommendations,
	)

	return narrative
}

func FormatSafetyTemplate(result *DiagnosisRecommendationResult) string {
	if result == nil || len(result.Recommendations) == 0 {
		return "- Ikuti label resmi produk sebelum aplikasi.\n- Gunakan alat pelindung diri saat menangani pestisida."
	}

	top := result.Recommendations[0]
	items := make([]string, 0, 8)

	items = append(
		items,
		"Gunakan alat pelindung diri lengkap saat pencampuran dan aplikasi.",
		"Ikuti dosis, waktu aplikasi, dan larangan pada label resmi produk.",
		"Jangan makan, minum, atau merokok saat menangani pestisida.",
		"Simpan pestisida di tempat terkunci dan jauh dari anak-anak serta sumber air.",
	)

	if top.Safety.ReentryInterval != nil {
		items = append(
			items,
			"Interval masuk kembali: "+*top.Safety.ReentryInterval+".",
		)
	}

	if top.Safety.PreHarvestInterval != nil {
		items = append(
			items,
			"Interval pra-panen: "+*top.Safety.PreHarvestInterval+".",
		)
	}

	if top.Safety.MaxApplication != nil {
		items = append(
			items,
			"Batas aplikasi: "+*top.Safety.MaxApplication+".",
		)
	}

	items = append(items, top.Safety.MixingWarning...)

	lines := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		lines = append(lines, "- "+item)
	}

	return strings.Join(lines, "\n")
}

func ingredientFamilyKey(ingredients []domain.PesticideIngredient) string {
	families := make([]string, 0, len(ingredients))

	for _, ingredient := range ingredients {
		name := normalizeIngredientFamilyName(ingredient.Name)
		if name == "" {
			continue
		}

		families = append(families, name)
	}

	if len(families) == 0 {
		return ""
	}

	sort.Strings(families)

	unique := make([]string, 0, len(families))
	last := ""
	for _, family := range families {
		if family == last {
			continue
		}
		unique = append(unique, family)
		last = family
	}

	return strings.Join(unique, "|")
}

func normalizeIngredientFamilyName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}

	// Nama bahan aktif pada database sering memuat angka konsentrasi, misalnya
	// "Dimehipo 525" atau "Chlorantraniliprole 100". Normalisasi harus memakai
	// pencocokan substring agar dua produk dengan bahan aktif sama tidak lolos
	// sebagai keluarga berbeda hanya karena angka/formulasi berbeda.
	replacer := strings.NewReplacer(
		"_", " ",
		"-", " ",
		",", " ",
		"/", " ",
		".", " ",
	)
	cleaned := replacer.Replace(name)
	for strings.Contains(cleaned, "  ") {
		cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	}

	aliases := []struct {
		Needle string
		Family string
	}{
		{"clothianidin", "klotianidin"},
		{"klotianidin", "klotianidin"},
		{"thiamethoxam", "tiametoksam"},
		{"tiametoksam", "tiametoksam"},
		{"imidacloprid", "imidakloprid"},
		{"imidakloprid", "imidakloprid"},
		{"nitenpyram", "nitenpiram"},
		{"nitenpiram", "nitenpiram"},
		{"pymetrozine", "pimetrozin"},
		{"pimetrozin", "pimetrozin"},
		{"chlorantraniliprole", "klorantraniliprol"},
		{"klorantraniliprol", "klorantraniliprol"},
		{"indoxacarb", "indoksakarb"},
		{"indoksakarb", "indoksakarb"},
		{"emamectin", "emamectin"},
		{"emamektin", "emamectin"},
		{"flubendiamide", "flubendiamide"},
		{"flubendiamida", "flubendiamide"},
		{"monosultap", "monosultap"},
		{"bisultap", "bisultap"},
		{"cartap", "cartap"},
		{"kartap", "cartap"},
		{"abamectin", "abamectin"},
		{"abamektin", "abamectin"},
		{"dimehipo", "dimehipo"},
		{"dimehypo", "dimehipo"},
		{"buprofezin", "buprofezin"},
		{"fipronil", "fipronil"},
		{"bpmc", "bpmc"},
		{"dinotefuran", "dinotefuran"},
		{"spinetoram", "spinetoram"},
	}

	for _, alias := range aliases {
		if strings.Contains(cleaned, alias.Needle) {
			return alias.Family
		}
	}

	parts := strings.Fields(cleaned)
	for _, part := range parts {
		if regexp.MustCompile(`^[a-z]+$`).MatchString(part) {
			return part
		}
	}

	return cleaned
}

func sanitizeApplicationNarrative(text string, recommendations []dto.PesticideRecommendationResponse) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}

	if !hasGranuleRecommendation(recommendations) {
		return text
	}

	replacer := strings.NewReplacer(
		"cakupan semprot", "cakupan aplikasi",
		"Cakupan semprot", "Cakupan aplikasi",
		"penyemprotan", "aplikasi",
		"Penyemprotan", "Aplikasi",
		"semprot", "aplikasi",
		"Semprot", "Aplikasi",
	)

	text = replacer.Replace(text)

	formulationNote := "Perhatikan bentuk formulasi: produk cair/larutan digunakan sesuai petunjuk label, sedangkan formulasi GR atau granul mengikuti cara aplikasi granul pada label."

	if !containsAnyFold(text, "formulasi gr", "granul", "aplikasi granul") {
		text = strings.TrimSpace(text) + "\n" + formulationNote
	}

	return strings.TrimSpace(text)
}

func hasGranuleRecommendation(recommendations []dto.PesticideRecommendationResponse) bool {
	for _, recommendation := range recommendations {
		formulation := strings.ToUpper(strings.TrimSpace(recommendation.Formulation))
		if formulation == "GR" || strings.Contains(formulation, "GRANUL") {
			return true
		}
	}

	return false
}

func ensurePHTRotationNote(text string, recommendations []dto.PesticideRecommendationResponse) string {
	text = strings.TrimSpace(text)
	if text == "" || len(recommendations) == 0 {
		return text
	}

	hasPHT := containsAnyFold(text, "pht", "pengendalian hama terpadu")
	hasRotation := containsAnyFold(text, "rotasi", "resistensi")

	if hasPHT && hasRotation {
		return text
	}

	note := "Terapkan pengendalian hama terpadu (PHT), pantau populasi hama setelah tindakan, dan lakukan rotasi bahan aktif pada aplikasi berikutnya untuk menekan risiko resistensi."

	return strings.TrimSpace(text) + "\n" + note
}

func containsAnyFold(text string, needles ...string) bool {
	lower := strings.ToLower(text)

	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle == "" {
			continue
		}

		if strings.Contains(lower, needle) {
			return true
		}
	}

	return false
}

func sanitizeSeverityTermNarrative(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}

	replacer := strings.NewReplacer(
		"keparahan yang tinggi", "keparahan berat",
		"Keparahan yang tinggi", "Keparahan berat",
		"tingkat keparahan tinggi", "tingkat keparahan berat",
		"Tingkat keparahan tinggi", "Tingkat keparahan berat",
		"tingkat serangan tinggi", "tingkat serangan berat",
		"Tingkat serangan tinggi", "Tingkat serangan berat",
		"keparahan tinggi", "keparahan berat",
		"Keparahan tinggi", "Keparahan berat",
		"severity tinggi", "severity berat",
		"Severity tinggi", "Severity berat",
	)

	return strings.TrimSpace(
		replacer.Replace(text),
	)
}

func displayFallback(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}

	return value
}

func limitStrings(values []string, maxItems int) []string {
	if maxItems <= 0 || len(values) <= maxItems {
		return values
	}
	return values[:maxItems]
}

func compactText(value string, maxChars int, maxSentences int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Lakukan pemantauan ulang dan ikuti rekomendasi pengendalian sesuai kondisi lahan."
	}

	value = strings.ReplaceAll(value, "\r", "\n")
	value = regexp.MustCompile(`\s+`).ReplaceAllString(value, " ")
	value = strings.TrimSpace(strings.Trim(value, "-• "))

	if maxSentences > 0 {
		parts := regexp.MustCompile(`(?m)([^.!?]+[.!?])`).FindAllString(value, -1)
		if len(parts) > 0 {
			selected := make([]string, 0, maxSentences)
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				selected = append(selected, part)
				if len(selected) >= maxSentences {
					break
				}
			}
			if len(selected) > 0 {
				value = strings.Join(selected, " ")
			}
		}
	}

	if maxChars > 0 && len([]rune(value)) > maxChars {
		runes := []rune(value)
		value = strings.TrimSpace(string(runes[:maxChars]))
		value = strings.TrimRight(value, ",;:- ") + "..."
	}

	return value
}

func briefApplicationTiming(item dto.PesticideRecommendationResponse) string {
	if item.ApplicationTiming == nil {
		return ""
	}

	candidates := []string{
		item.ApplicationTiming.TimingWindow,
		item.ApplicationTiming.ApplicationTrigger,
		item.ApplicationTiming.TimingInstruction,
	}

	for _, candidate := range candidates {
		candidate = compactText(candidate, 0, 1)
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}

	return ""
}

func FormatPesticideRecommendations(
	result *DiagnosisRecommendationResult,
) string {

	if result == nil || len(result.Recommendations) == 0 {
		return "Rekomendasi pestisida belum ditampilkan karena gejala yang dipilih belum cukup kuat."
	}

	item := result.Recommendations[0]
	productName := strings.TrimSpace(item.ProductName)
	if productName == "" {
		productName = safeRecommendationName(item)
	}
	if productName == "" {
		productName = "Produk pestisida terpilih"
	}

	ingredientStr := formatIngredients(item.Ingredients)
	if strings.TrimSpace(ingredientStr) == "" {
		ingredientStr = "bahan aktif yang tercatat pada data produk terdaftar"
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("- Produk utama: %s\n", productName))
	builder.WriteString(fmt.Sprintf("- Bahan aktif: %s\n", ingredientStr))
	builder.WriteString(fmt.Sprintf("- Dosis: %s sesuai label produk\n", displayFallback(formatDosage(item.Dosage), "ikuti label produk")))

	if timing := briefApplicationTiming(item); timing != "" {
		builder.WriteString(fmt.Sprintf("- Waktu: %s\n", timing))
	}

	builder.WriteString("- Catatan: gunakan alat pelindung diri, ikuti label resmi, dan tanyakan kepada penyuluh bila kondisi lahan meragukan.")

	return strings.TrimSpace(builder.String())
}

func pesticidePriorityLabel(index int) string {
	switch index {
	case 0:
		return "Utama"
	case 1:
		return "Alternatif rotasi 1"
	case 2:
		return "Alternatif rotasi 2"
	default:
		return "Alternatif"
	}
}

func buildPesticideItemReason(item dto.PesticideRecommendationResponse, result *DiagnosisRecommendationResult) string {
	pestName := "hama target"
	severity := "tingkat keparahan yang terdiagnosis"
	growthStage := "fase pertumbuhan tanaman"

	if result != nil {
		if result.Pest != nil && strings.TrimSpace(result.Pest.Name) != "" {
			pestName = result.Pest.Name
		}
		if strings.TrimSpace(result.Severity) != "" {
			severity = "tingkat keparahan " + strings.ToLower(strings.TrimSpace(result.Severity))
		}
		if strings.TrimSpace(result.GrowthStage) != "" {
			growthStage = "fase " + strings.ToLower(strings.TrimSpace(result.GrowthStage))
		}
	}

	productName := strings.TrimSpace(item.ProductName)
	if productName == "" {
		productName = "Produk ini"
	}

	activeIngredient := formatIngredients(item.Ingredients)
	if strings.TrimSpace(activeIngredient) == "" {
		activeIngredient = "bahan aktif pada produk ini"
	}

	return fmt.Sprintf(
		"%s direkomendasikan karena bahan aktif %s sesuai dengan target pengendalian %s pada %s dan %s. Penggunaan tetap mengikuti dosis, cara aplikasi, dan ketentuan pada label resmi produk.",
		productName,
		activeIngredient,
		pestName,
		severity,
		growthStage,
	)
}

func formatApplicationGuidanceForUser(recommendations []dto.PesticideRecommendationResponse) string {
	timing := firstApplicationTiming(recommendations)

	window := "Ikuti waktu aplikasi yang tercantum pada label resmi produk."
	method := "Cara aplikasi mengikuti formulasi dan petunjuk pada label masing-masing produk."
	target := "Sasaran aplikasi mengikuti hama target dan petunjuk pada label produk."
	note := "Aplikasi disarankan pada kondisi suhu tidak terlalu terik dan lahan memungkinkan untuk perlakuan yang aman."
	warning := ""
	preharvest := ""

	if timing != nil {
		if value := strings.TrimSpace(timing.TimingWindow); value != "" {
			window = ensureTrailingPeriod(value)
		}

		if value := formatApplicationMethodForUser(timing.ApplicationMethod); value != "" {
			method = ensureTrailingPeriod(value)
		}

		if value := strings.TrimSpace(timing.ApplicationTarget); value != "" {
			target = ensureTrailingPeriod(value)
		}

		if value := formatApplicationNoteForUser(*timing); value != "" {
			note = ensureTrailingPeriod(value)
		}

		if value := strings.TrimSpace(timing.DisplayWarning); value != "" {
			warning = ensureTrailingPeriod(value)
		}

		if value := strings.TrimSpace(timing.PreharvestIntervalNote); value != "" {
			preharvest = ensureTrailingPeriod(value)
		}
	}

	parts := []string{
		"- Waktu: " + window,
		"- Cara: " + method,
		"- Sasaran aplikasi: " + target,
		"- Kondisi: " + note,
	}

	if preharvest != "" {
		parts = append(parts, "- Masa tunggu: "+preharvest)
	}

	if warning != "" {
		parts = append(parts, "- Peringatan lapang: "+warning)
	}

	return strings.Join(parts, "\n")
}

func firstApplicationTiming(recommendations []dto.PesticideRecommendationResponse) *dto.PesticideApplicationTimingResponse {
	for _, item := range recommendations {
		if item.ApplicationTiming != nil {
			return item.ApplicationTiming
		}
	}

	return nil
}

func formatApplicationMethodForUser(method string) string {
	method = strings.TrimSpace(method)
	if method == "" {
		return ""
	}

	lower := strings.ToLower(method)
	if strings.Contains(lower, "sesuai label") {
		return "Metode aplikasi mengikuti formulasi dan label produk"
	}

	return "Metode aplikasi: " + method
}

func formatApplicationNoteForUser(timing dto.PesticideApplicationTimingResponse) string {
	candidates := []string{
		strings.TrimSpace(timing.ApplicationTrigger),
		strings.TrimSpace(timing.TimingInstruction),
		strings.TrimSpace(timing.WaterManagement),
		strings.TrimSpace(timing.WeatherCondition),
	}

	parts := make([]string, 0, 3)
	seen := make(map[string]struct{})

	for _, candidate := range candidates {
		for _, sentence := range splitGuidanceSentences(candidate) {
			sentence = sanitizeGuidanceSentence(sentence)
			if sentence == "" || isReferenceLikeGuidance(sentence) || isGlobalPHTGuidance(sentence) {
				continue
			}

			key := strings.ToLower(sentence)
			if _, exists := seen[key]; exists {
				continue
			}

			parts = append(parts, sentence)
			seen[key] = struct{}{}

			if len(parts) >= 2 {
				break
			}
		}

		if len(parts) >= 2 {
			break
		}
	}

	return strings.Join(parts, " ")
}

func splitGuidanceSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{}
	}

	replacer := strings.NewReplacer(";", ".", "\n", ".")
	text = replacer.Replace(text)

	rawParts := strings.Split(text, ".")
	parts := make([]string, 0, len(rawParts))

	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}

	return parts
}

func sanitizeGuidanceSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	text = strings.TrimSuffix(text, ".")
	text = strings.TrimSpace(text)

	return text
}

func isReferenceLikeGuidance(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return containsAnyFold(lower,
		"pemerintah",
		"dinas",
		"kabupaten",
		"lugosobo",
		"http://",
		"https://",
		"tahun",
		"2020",
		"2021",
		"2022",
		"2023",
		"2024",
		"2025",
	)
}

func isGlobalPHTGuidance(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return containsAnyFold(lower,
		"prinsip 6 tepat",
		"6 tepat",
		"rotasi bahan aktif",
		"resistensi",
		"pemantauan",
		"pengamatan lapangan",
	)
}

func ensureTrailingPeriod(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}

	if strings.HasSuffix(text, ".") || strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?") {
		return text
	}

	return text + "."
}

func joinNonEmpty(separator string, values ...string) string {
	parts := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parts = append(parts, value)
	}

	return strings.Join(parts, separator)
}

func formatPesticideTypeForUser(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	upper := strings.ToUpper(value)
	switch upper {
	case "INSECTICIDE", "INSEKTISIDA":
		return "Insektisida"
	case "FUNGICIDE", "FUNGISIDA":
		return "Fungisida"
	case "HERBICIDE", "HERBISIDA":
		return "Herbisida"
	default:
		return strings.Title(strings.ToLower(value))
	}
}

func formatApplicationTiming(timing dto.PesticideApplicationTimingResponse) string {
	parts := make([]string, 0, 5)

	if window := strings.TrimSpace(timing.TimingWindow); window != "" {
		parts = append(parts, "Waktu: "+window)
	}

	if method := formatApplicationMethodForUser(timing.ApplicationMethod); method != "" {
		parts = append(parts, "Cara: "+method)
	}

	if target := strings.TrimSpace(timing.ApplicationTarget); target != "" {
		parts = append(parts, "Sasaran: "+target)
	}

	if note := formatApplicationNoteForUser(timing); note != "" {
		parts = append(parts, "Catatan: "+note)
	}

	if warning := strings.TrimSpace(timing.DisplayWarning); warning != "" {
		parts = append(parts, "Peringatan: "+warning)
	}

	return strings.Join(parts, " | ")
}

func formatApplicationTimingReference(timing dto.PesticideApplicationTimingResponse) string {
	parts := make([]string, 0, 4)

	if strings.TrimSpace(timing.ReferenceInstitution) != "" {
		parts = append(parts, strings.TrimSpace(timing.ReferenceInstitution))
	}

	if strings.TrimSpace(timing.ReferenceYear) != "" {
		parts = append(parts, strings.TrimSpace(timing.ReferenceYear))
	}

	if strings.TrimSpace(timing.ReferenceTitle) != "" {
		parts = append(parts, strings.TrimSpace(timing.ReferenceTitle))
	}

	return strings.Join(parts, " - ")
}

func extractMatchedSymptomIDs(
	raw json.RawMessage,
) []uuid.UUID {

	if len(raw) == 0 {
		return []uuid.UUID{}
	}

	var sessionData diagnose.SymptomSessionData

	if err := json.Unmarshal(
		raw,
		&sessionData,
	); err != nil {

		return []uuid.UUID{}
	}

	seen := make(
		map[uuid.UUID]struct{},
	)

	results := make(
		[]uuid.UUID,
		0,
		len(sessionData.Normalized),
	)

	for _, item := range sessionData.Normalized {
		if item.MatchedSymptomID == nil ||
			*item.MatchedSymptomID == uuid.Nil {

			continue
		}

		if _, exists := seen[*item.MatchedSymptomID]; exists {
			continue
		}

		seen[*item.MatchedSymptomID] = struct{}{}

		results = append(
			results,
			*item.MatchedSymptomID,
		)
	}

	return results
}

func buildPesticideProductDisplayName(productName string, activeIngredient string, formulation string, pesticideType string) string {
	productName = strings.TrimSpace(productName)
	activeIngredient = strings.TrimSpace(activeIngredient)
	formulation = strings.ToUpper(strings.TrimSpace(formulation))
	pesticideType = formatPesticideTypeForUser(pesticideType)

	parts := make([]string, 0, 4)

	if productName != "" {
		parts = append(parts, productName)
	}

	if activeIngredient != "" {
		parts = append(parts, "bahan aktif "+activeIngredient)
	}

	if formulation != "" {
		parts = append(parts, "formulasi "+formulation)
	}

	if pesticideType != "" {
		parts = append(parts, pesticideType)
	}

	if len(parts) == 0 {
		return "Produk pestisida terdaftar"
	}

	return strings.Join(parts, " - ")
}

func buildPesticideDisplayName(activeIngredient string, formulation string, pesticideType string) string {
	activeIngredient = strings.TrimSpace(activeIngredient)
	formulation = strings.ToUpper(strings.TrimSpace(formulation))
	pesticideType = formatPesticideTypeForUser(pesticideType)

	if activeIngredient == "" {
		activeIngredient = "bahan aktif terdaftar"
	}

	parts := make([]string, 0, 3)
	parts = append(parts, "Pestisida berbahan aktif "+activeIngredient)

	if formulation != "" {
		parts = append(parts, "formulasi "+formulation)
	}

	if pesticideType != "" {
		parts = append(parts, pesticideType)
	}

	return strings.Join(parts, " - ")
}

func buildRecommendationReason(productName string, ingredients []dto.PesticideIngredientResponse, pestName string, severity string, growthStage string) string {
	productName = strings.TrimSpace(productName)
	if productName == "" {
		productName = "Produk ini"
	}

	activeIngredient := strings.TrimSpace(formatIngredients(ingredients))
	if activeIngredient == "" {
		activeIngredient = "bahan aktif terdaftar"
	}

	pestName = strings.TrimSpace(pestName)
	if pestName == "" {
		pestName = "hama sasaran"
	}

	severity = strings.TrimSpace(severity)
	if severity == "" {
		severity = "tingkat serangan yang terdiagnosis"
	}

	growthStage = strings.TrimSpace(growthStage)
	if growthStage == "" {
		growthStage = "fase pertumbuhan tanaman"
	}

	return fmt.Sprintf(
		"%s ditampilkan karena bahan aktif %s tercatat untuk mengendalikan %s. Urutan rekomendasi mempertimbangkan kelengkapan data sasaran, dosis, waktu aplikasi, dan masa berlaku pendaftaran. Tingkat serangan %s dan fase %s digunakan untuk menjelaskan kondisi lahan, bukan untuk mengubah dosis. Ikuti label resmi produk serta arahan petugas lapangan bila diperlukan.",
		productName,
		activeIngredient,
		pestName,
		severity,
		growthStage,
	)
}

func buildRecommendationBasis(activeIngredient string, formulation string) string {
	activeIngredient = strings.TrimSpace(activeIngredient)
	formulation = strings.ToUpper(strings.TrimSpace(formulation))

	if activeIngredient == "" {
		activeIngredient = "bahan aktif terdaftar"
	}

	if formulation == "" {
		return activeIngredient
	}

	return activeIngredient + " | " + formulation
}

func safeRecommendationName(item dto.PesticideRecommendationResponse) string {
	if strings.TrimSpace(item.ProductName) != "" {
		return strings.TrimSpace(item.ProductName)
	}

	if strings.TrimSpace(item.DisplayName) != "" {
		return strings.TrimSpace(item.DisplayName)
	}

	return buildPesticideDisplayName(
		formatIngredients(item.Ingredients),
		item.Formulation,
		item.PesticideType,
	)
}

func formatIngredients(
	ingredients []dto.PesticideIngredientResponse,
) string {

	items := make(
		[]string,
		0,
		len(ingredients),
	)

	for _, ingredient := range ingredients {
		name :=
			strings.TrimSpace(
				ingredient.Name,
			)

		if name == "" {
			continue
		}

		value := name

		if strings.TrimSpace(
			ingredient.ConcentrationRaw,
		) != "" {

			value +=
				" " +
					ingredient.ConcentrationRaw
		}

		items = append(
			items,
			value,
		)
	}

	return strings.Join(items, ", ")
}

func formatDosage(
	dosage dto.PesticideDosageResponse,
) string {
	raw := strings.TrimSpace(dosage.DoseRaw)
	unit := strings.TrimSpace(dosage.DoseUnit)

	if raw == "" {
		raw = "Ikuti dosis pada label produk"
	}

	if unit == "" || strings.EqualFold(unit, "label") {
		return raw
	}

	markers := []struct {
		Marker string
		Label  string
	}{
		{"; batas atas rentang label ", "batas atas"},
		{"; batas bawah rentang label ", "batas bawah"},
		{"; nilai tengah rentang label ", "nilai tengah"},
	}

	for _, marker := range markers {
		if idx := strings.Index(raw, marker.Marker); idx >= 0 {
			selected := strings.TrimSpace(raw[:idx])
			rangeText := strings.TrimSpace(raw[idx+len(marker.Marker):])
			if selected != "" && rangeText != "" {
				return fmt.Sprintf(
					"%s %s (%s rentang label: %s %s)",
					selected,
					unit,
					marker.Label,
					rangeText,
					unit,
				)
			}
		}
	}

	return fmt.Sprintf("%s (%s)", raw, unit)
}

func toEngineSeverity(
	severity string,
) string {

	switch strings.ToLower(
		strings.TrimSpace(severity),
	) {
	case "tinggi", "berat", "parah", "high":
		return "high"

	case "sedang", "medium", "moderate":
		return "medium"

	default:
		return "low"
	}
}

func hasStrongRuleResult(result *rule.ExecuteResponse) bool {
	return result != nil &&
		!result.FallbackRequired &&
		result.BestMatch != nil &&
		result.BestMatch.Candidate.Rule.ID != uuid.Nil &&
		result.BestMatch.Score.FinalScore >= 0.60
}

func ruleConfidence(
	result *rule.ExecuteResponse,
) float64 {

	if result == nil ||
		result.BestMatch == nil {

		return 0
	}

	confidence := result.BestMatch.Score.FinalScore

	// Output confidence tidak ditampilkan sebagai 100% agar tidak terlihat
	// absolut. Diagnosis lapangan tetap mengandung ketidakpastian karena
	// bergantung pada kualitas foto, jumlah gejala, dan kondisi tanaman.
	if confidence >= 0.995 {
		return 0.96
	}

	if confidence > 0.98 {
		return 0.98
	}

	return confidence
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	return &value
}

func extractRuleCode(
	result *rule.ExecuteResponse,
) string {

	if result == nil {
		return ""
	}

	if result.BestMatch == nil {
		return ""
	}

	return strings.TrimSpace(
		result.BestMatch.
			Candidate.
			Rule.
			Code,
	)
}
