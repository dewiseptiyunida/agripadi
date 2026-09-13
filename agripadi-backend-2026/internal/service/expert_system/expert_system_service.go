package expertSystem

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/gustian305/backend/internal/service/expert_system/session"
	"github.com/gustian305/backend/internal/service/expert_system/workflow"
	"github.com/gustian305/backend/logger"
)

var (
	ErrInvalidConversationID = errors.New(
		"invalid conversation id",
	)

	ErrInvalidSessionID = errors.New(
		"invalid session id",
	)

	ErrEmptyDetectionCandidates = errors.New(
		"detection candidates are required",
	)

	ErrEmptyUserInput = errors.New(
		"user input is required",
	)
)

var symptomSelectionSplitter = regexp.MustCompile(`(?i)\s*(?:,|;|\||\r?\n|\.|\s+dan\s+|\s+serta\s+|\s+lalu\s+|\s+kemudian\s+)\s*`)

type ExpertSystemService struct {
	workflowRouter *workflow.RouterService
	catalogRepo    interface {
		FindPestByLabelName(ctx context.Context, labelName string) (*domain.Pest, error)
		FindSymptomsByPestID(ctx context.Context, pestID uuid.UUID) ([]domain.Symptom, error)
	}

	sessionService *session.SessionService
	sessionLoader  *session.SessionLoader
	sessionUpdater *session.SessionUpdater

	diagnoseService       *diagnose.DiagnoseService
	recommendationService *RecommendationService
}

func NewExpertSystemService(
	workflowRouter *workflow.RouterService,
	catalogRepo interface {
		FindPestByLabelName(ctx context.Context, labelName string) (*domain.Pest, error)
		FindSymptomsByPestID(ctx context.Context, pestID uuid.UUID) ([]domain.Symptom, error)
	},

	sessionService *session.SessionService,
	sessionLoader *session.SessionLoader,
	sessionUpdater *session.SessionUpdater,

	diagnoseService *diagnose.DiagnoseService,
	recommendationService *RecommendationService,
) *ExpertSystemService {

	return &ExpertSystemService{
		workflowRouter: workflowRouter,
		catalogRepo:    catalogRepo,

		sessionService: sessionService,
		sessionLoader:  sessionLoader,
		sessionUpdater: sessionUpdater,

		diagnoseService:       diagnoseService,
		recommendationService: recommendationService,
	}
}

func (s *ExpertSystemService) StartDiagnosis(ctx context.Context, conversationID uuid.UUID, detections []dto.DetectionCandidateRequest) (*dto.OrchestratorResponse, error) {

	startedAt := time.Now()
	operation := "StartDiagnosis"

	logger.Request(
		"expert_system",
		operation,
		slog.String(
			"conversation_id",
			conversationID.String(),
		),
		slog.Int(
			"detection_count",
			len(detections),
		),
	)

	if conversationID == uuid.Nil {

		logger.Failure(
			"expert_system",
			operation,
			startedAt,
			ErrInvalidConversationID,
		)

		return nil,
			ErrInvalidConversationID
	}

	if len(detections) == 0 {

		logger.Failure(
			"expert_system",
			operation,
			startedAt,
			ErrEmptyDetectionCandidates,
		)

		return nil,
			ErrEmptyDetectionCandidates
	}

	err :=
		s.diagnoseService.Start(
			ctx,
			conversationID.String(),
			detections,
		)

	if err != nil {

		logger.Failure(
			"expert_system",
			operation,
			startedAt,
			err,
		)

		return nil,
			err
	}

	diagnoseSession, err :=
		s.sessionLoader.GetActiveByConversationID(
			ctx,
			conversationID.String(),
		)

	if err != nil {

		logger.Failure(
			"expert_system",
			operation,
			startedAt,
			err,
		)

		return nil,
			err
	}

	if diagnoseSession == nil {

		err :=
			errors.New(
				"diagnosis session not found",
			)

		logger.Failure(
			"expert_system",
			operation,
			startedAt,
			err,
		)

		return nil,
			err
	}

	// =====================================================
	// NO PEST FLOW
	// =====================================================

	if isNoPestDetectedLabel(
		diagnoseSession.DetectedLabel,
	) {

		if err := s.sessionService.Complete(
			ctx,
			diagnoseSession,
		); err != nil {

			logger.Failure(
				"expert_system",
				operation,
				startedAt,
				err,
			)

			return nil,
				err
		}

		response := &dto.OrchestratorResponse{
			SessionID: diagnoseSession.ID,

			Mode: dto.ConversationModeConsultation,

			State: dto.DiagnoseFlowStateCompleted,

			Message: strings.TrimSpace(`
Foto belum menunjukkan tanda yang cukup kuat dari tiga hama yang diperiksa.

Jangan gunakan pestisida hanya berdasarkan hasil foto ini. Periksa langsung bagian pangkal batang, batang, malai, dan bulir.

Lakukan pemantauan rutin. Ambil foto baru dari jarak dekat apabila terlihat hama atau gejala kerusakan.
`),

			Actions: []dto.ChatAction{
				{
					Type:  dto.ActionContinueConsult,
					Label: "Tanya AgriPadi",
					Value: "consultation",
				},
				{
					Type:  dto.ActionUploadImage,
					Label: "Periksa Foto Lain",
					Value: "upload_image",
				},
			},

			CreatedAt: time.Now(),
		}

		logger.Response(
			"expert_system",
			operation,
			startedAt,
			slog.String(
				"session_id",
				response.SessionID.String(),
			),
			slog.String(
				"state",
				string(response.State),
			),
			slog.String(
				"detected_label",
				diagnoseSession.DetectedLabel,
			),
			slog.Float64(
				"detected_confidence",
				diagnoseSession.DetectedConfidence,
			),
			slog.Bool(
				"no_pest_flow",
				true,
			),
		)

		return response,
			nil
	}

	// =====================================================
	// LOW CONFIDENCE / UNSUPPORTED PEST FLOW
	// =====================================================

	if diagnoseSession.DetectedConfidence > 0 && diagnoseSession.DetectedConfidence < 0.50 {
		if err := s.sessionService.Complete(ctx, diagnoseSession); err != nil {
			logger.Failure("expert_system", operation, startedAt, err)
			return nil, err
		}

		response := buildUnsupportedDetectionResponse(
			diagnoseSession,
			"gambar belum cukup jelas untuk dilanjutkan secara aman",
		)

		logger.Response(
			"expert_system",
			operation,
			startedAt,
			slog.String("session_id", response.SessionID.String()),
			slog.String("state", string(response.State)),
			slog.String("detected_label", diagnoseSession.DetectedLabel),
			slog.Float64("detected_confidence", diagnoseSession.DetectedConfidence),
			slog.Bool("low_confidence_flow", true),
		)

		return response, nil
	}

	if s.catalogRepo != nil {
		pest, err := s.catalogRepo.FindPestByLabelName(ctx, diagnoseSession.DetectedLabel)
		if err != nil {
			logger.Failure("expert_system", operation, startedAt, err)
			return nil, err
		}

		if pest == nil {
			if err := s.sessionService.Complete(ctx, diagnoseSession); err != nil {
				logger.Failure("expert_system", operation, startedAt, err)
				return nil, err
			}

			response := buildUnsupportedDetectionResponse(
				diagnoseSession,
				"jenis hama tersebut belum tersedia pada pemeriksaan AgriPadi",
			)

			logger.Response(
				"expert_system",
				operation,
				startedAt,
				slog.String("session_id", response.SessionID.String()),
				slog.String("state", string(response.State)),
				slog.String("detected_label", diagnoseSession.DetectedLabel),
				slog.Float64("detected_confidence", diagnoseSession.DetectedConfidence),
				slog.Bool("unsupported_pest_flow", true),
			)

			return response, nil
		}
	}

	// =====================================================
	// NORMAL FLOW
	// =====================================================

	response, err :=
		s.workflowRouter.Handle(
			ctx,
			workflow.RouteRequest{
				Session: diagnoseSession,
				Event:   workflow.EventDiagnosisStarted,
			},
		)

	if err != nil {

		logger.Failure(
			"expert_system",
			operation,
			startedAt,
			err,
		)

		return nil,
			err
	}

	response.SessionID =
		diagnoseSession.ID

	response.Mode =
		dto.ConversationModeDiagnose

	response.Message =
		buildSymptomQuestionMessage(
			ctx,
			s.catalogRepo,
			diagnoseSession,
		)

	response.Actions =
		buildSymptomQuestionActions(
			ctx,
			s.catalogRepo,
			diagnoseSession,
		)

	response.CreatedAt =
		time.Now()

	logger.Response(
		"expert_system",
		operation,
		startedAt,
		slog.String(
			"session_id",
			response.SessionID.String(),
		),
		slog.String(
			"state",
			string(response.State),
		),
		slog.String(
			"detected_label",
			diagnoseSession.DetectedLabel,
		),
		slog.Float64(
			"detected_confidence",
			diagnoseSession.DetectedConfidence,
		),
	)

	return response,
		nil
}

func (s *ExpertSystemService) ContinueActiveDiagnosis(ctx context.Context, conversationID uuid.UUID, userInput string) (*dto.OrchestratorResponse, error) {

	startedAt := time.Now()
	operation := "ContinueActiveDiagnosis"

	logger.Request(
		"expert_system",
		operation,
		slog.String(
			"conversation_id",
			conversationID.String(),
		),
		slog.Int(
			"input_length",
			len(userInput),
		),
	)

	if conversationID == uuid.Nil {

		logger.Failure(
			"expert_system",
			operation,
			startedAt,
			ErrInvalidConversationID,
		)

		return nil,
			ErrInvalidConversationID
	}

	userInput =
		strings.TrimSpace(
			userInput,
		)

	if userInput == "" {

		logger.Failure(
			"expert_system",
			operation,
			startedAt,
			ErrEmptyUserInput,
		)

		return nil,
			ErrEmptyUserInput
	}

	diagnoseSession, err :=
		s.sessionService.GetActiveByConversationID(
			ctx,
			conversationID,
		)

	if err != nil {

		logger.Failure(
			"expert_system",
			operation,
			startedAt,
			err,
		)

		return nil,
			err
	}

	if diagnoseSession == nil {

		logger.Response(
			"expert_system",
			operation,
			startedAt,
			slog.Bool(
				"active_session_found",
				false,
			),
		)

		return nil,
			nil
	}

	// =====================================================
	// NO PEST SAFETY GUARD
	// =====================================================

	if isNoPestDetectedLabel(
		diagnoseSession.DetectedLabel,
	) {

		if err := s.sessionService.Complete(
			ctx,
			diagnoseSession,
		); err != nil {

			logger.Failure(
				"expert_system",
				operation,
				startedAt,
				err,
			)

			return nil,
				err
		}

		response := &dto.OrchestratorResponse{
			SessionID: diagnoseSession.ID,

			Mode: dto.ConversationModeConsultation,

			State: dto.DiagnoseFlowStateCompleted,

			Message: `
Pemeriksaan foto selesai.

Foto belum menunjukkan tanda yang cukup kuat dari tiga hama yang diperiksa.

Periksa tanaman langsung dan ambil foto lain apabila terlihat hama atau kerusakan.
`,

			Actions: []dto.ChatAction{
				{
					Type:  dto.ActionUploadImage,
					Label: "Periksa Foto Lain",
					Value: "upload_image",
				},
			},

			CreatedAt: time.Now(),
		}

		logExpertResponse(
			operation,
			startedAt,
			dto.DiagnoseFlowStateCompleted,
			response,
			nil,
		)

		return response,
			nil
	}

	state :=
		dto.DiagnoseFlowState(
			diagnoseSession.State,
		)

	switch state {

	case dto.DiagnoseFlowStateImageAnalyzed,
		dto.DiagnoseFlowStateCollectSymptoms:

		response, err := s.continueSymptoms(
			ctx,
			diagnoseSession.ID,
			conversationID,
			userInput,
		)

		logExpertResponse(
			operation,
			startedAt,
			state,
			response,
			err,
		)

		return response,
			err

	case dto.DiagnoseFlowStateNormalizeSymptoms,
		dto.DiagnoseFlowStateCollectSeverity:

		response, err := s.continueSeverity(
			ctx,
			diagnoseSession.ID,
			conversationID,
			userInput,
		)

		logExpertResponse(
			operation,
			startedAt,
			state,
			response,
			err,
		)

		return response,
			err

	case dto.DiagnoseFlowStateCollectGrowthStage:

		response, err := s.continueGrowthStage(
			ctx,
			diagnoseSession.ID,
			conversationID,
			userInput,
		)

		logExpertResponse(
			operation,
			startedAt,
			state,
			response,
			err,
		)

		return response,
			err

	case dto.DiagnoseFlowStateGenerateResult:

		response, err := s.completeDiagnosis(
			ctx,
			diagnoseSession.ID,
			conversationID,
		)

		logExpertResponse(
			operation,
			startedAt,
			state,
			response,
			err,
		)

		return response,
			err
	}

	logger.Response(
		"expert_system",
		operation,
		startedAt,
		slog.String(
			"current_state",
			string(state),
		),
		slog.Bool(
			"handled",
			false,
		),
	)

	return nil,
		nil
}

func (s *ExpertSystemService) ContinueDiagnosis(ctx context.Context, sessionID uuid.UUID, userInput string) (*dto.OrchestratorResponse, error) {

	if sessionID == uuid.Nil {

		return nil,
			ErrInvalidSessionID
	}

	if userInput == "" {

		return nil,
			ErrEmptyUserInput
	}

	diagnoseSession, err :=
		s.sessionLoader.GetByID(
			ctx,
			sessionID,
		)

	if err != nil {
		return nil, err
	}

	response, err :=
		s.workflowRouter.Handle(
			ctx,
			workflow.RouteRequest{
				Session: diagnoseSession,

				UserInput: userInput,

				Event: workflow.EventUserResponseReceived,
			},
		)

	if err != nil {
		return nil, err
	}

	response.SessionID =
		diagnoseSession.ID

	response.Mode =
		dto.ConversationModeDiagnose

	return response, nil
}

func (s *ExpertSystemService) SubmitSymptoms(ctx context.Context, conversationID uuid.UUID, symptoms []string) error {

	if conversationID == uuid.Nil {

		return ErrInvalidConversationID
	}

	return s.diagnoseService.SubmitSymptoms(
		ctx,
		conversationID.String(),
		symptoms,
	)
}

func (s *ExpertSystemService) SubmitSeverity(ctx context.Context, conversationID uuid.UUID, severity string) error {

	if conversationID == uuid.Nil {

		return ErrInvalidConversationID
	}

	return s.diagnoseService.SubmitSeverity(
		ctx,
		conversationID.String(),
		severity,
	)
}

func (s *ExpertSystemService) SubmitGrowthStage(ctx context.Context, conversationID uuid.UUID, stage string) error {

	if conversationID == uuid.Nil {

		return ErrInvalidConversationID
	}

	return s.diagnoseService.SubmitGrowthStage(
		ctx,
		conversationID.String(),
		stage,
	)
}

func (s *ExpertSystemService) FinalizeDiagnosis(ctx context.Context, conversationID uuid.UUID) error {

	if conversationID == uuid.Nil {

		return ErrInvalidConversationID
	}

	return s.diagnoseService.Finalize(
		ctx,
		conversationID.String(),
	)
}

func (s *ExpertSystemService) CancelDiagnosis(ctx context.Context, sessionID uuid.UUID) error {

	if sessionID == uuid.Nil {

		return ErrInvalidSessionID
	}

	sessionData, err :=
		s.sessionLoader.GetByID(
			ctx,
			sessionID,
		)

	if err != nil {
		return err
	}

	return s.sessionUpdater.Reset(
		ctx,
		sessionData,
	)
}

func (s *ExpertSystemService) continueSymptoms(ctx context.Context, sessionID uuid.UUID, conversationID uuid.UUID, userInput string) (*dto.OrchestratorResponse, error) {

	symptoms := splitUserSelections(
		userInput,
	)

	if len(symptoms) == 0 {
		symptoms = []string{userInput}
	}

	if err := s.SubmitSymptoms(
		ctx,
		conversationID,
		symptoms,
	); err != nil {
		return nil, err
	}

	updatedSession, err := s.sessionLoader.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if dto.DiagnoseFlowState(updatedSession.State) == dto.DiagnoseFlowStateCollectSymptoms {
		return buildAdditionalSymptomQuestionResponse(
			ctx,
			s.catalogRepo,
			updatedSession,
		), nil
	}

	return buildSeverityQuestionResponse(
		sessionID,
		updatedSession,
	), nil
}

func (s *ExpertSystemService) continueSeverity(ctx context.Context, sessionID uuid.UUID, conversationID uuid.UUID, userInput string) (*dto.OrchestratorResponse, error) {

	severity := normalizeSeverityInput(
		userInput,
	)

	currentSession, err := s.sessionLoader.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if response := buildSeverityEvidenceConflictResponse(sessionID, currentSession, severity); response != nil {
		return response, nil
	}

	if err := s.SubmitSeverity(
		ctx,
		conversationID,
		severity,
	); err != nil {
		return nil, err
	}

	return buildGrowthStageQuestionResponse(
		sessionID,
	), nil
}

func (s *ExpertSystemService) continueGrowthStage(ctx context.Context, sessionID uuid.UUID, conversationID uuid.UUID, userInput string) (*dto.OrchestratorResponse, error) {

	growthStage := normalizeGrowthStageInput(
		userInput,
	)

	currentSession, err := s.sessionLoader.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if response := buildGrowthStageEvidenceConflictResponse(sessionID, currentSession, growthStage); response != nil {
		return response, nil
	}

	if err := s.SubmitGrowthStage(
		ctx,
		conversationID,
		growthStage,
	); err != nil {
		return nil, err
	}

	return s.completeDiagnosis(
		ctx,
		sessionID,
		conversationID,
	)
}

func (s *ExpertSystemService) completeDiagnosis(ctx context.Context, sessionID uuid.UUID, conversationID uuid.UUID) (*dto.OrchestratorResponse, error) {
	startedAt := time.Now()
	operation := "completeDiagnosis"

	logger.Request(
		"expert_system",
		operation,
		slog.String("session_id", sessionID.String()),
		slog.String("conversation_id", conversationID.String()),
	)

	// Susun hasil terlebih dahulu sebelum sesi ditandai selesai. Jika database,
	// rule engine, atau layanan pendukung mengalami kegagalan sementara, sesi
	// tetap berada pada state generate_result sehingga pengguna dapat mencoba
	// kembali tanpa kehilangan alur diagnosis.
	pendingSession, err := s.sessionLoader.GetByID(
		ctx,
		sessionID,
	)

	if err != nil {
		logger.Failure("expert_system", operation, startedAt, err)
		return nil, err
	}

	recommendationResult, err := s.resolveRecommendations(
		ctx,
		pendingSession,
	)

	if err != nil {
		logger.Failure("expert_system", operation, startedAt, err)
		return nil, err
	}

	if err := s.FinalizeDiagnosis(
		ctx,
		conversationID,
	); err != nil {
		logger.Failure("expert_system", operation, startedAt, err)
		return nil, err
	}

	completedSession, err := s.sessionLoader.GetByID(
		ctx,
		sessionID,
	)

	if err != nil {
		logger.Failure("expert_system", operation, startedAt, err)
		return nil, err
	}

	response := buildCompletedResponse(
		sessionID,
		completedSession,
		recommendationResult,
	)

	recommendationCount := 0
	if recommendationResult != nil {
		recommendationCount = len(recommendationResult.Recommendations)
	}

	logger.Response(
		"expert_system",
		operation,
		startedAt,
		slog.String("session_id", sessionID.String()),
		slog.String("detected_label", completedSession.DetectedLabel),
		slog.String("severity", completedSession.Severity),
		slog.String("growth_stage", completedSession.GrowthStage),
		slog.Int("recommendation_count", recommendationCount),
	)

	return response, nil
}

func (s *ExpertSystemService) resolveRecommendations(ctx context.Context, session *domain.ExpertSession) (*DiagnosisRecommendationResult, error) {

	if s.recommendationService == nil {
		return &DiagnosisRecommendationResult{}, nil
	}

	return s.recommendationService.ResolveForSession(
		ctx,
		session,
	)
}

func buildSymptomQuestionMessage(
	ctx context.Context,
	catalogRepo interface {
		FindPestByLabelName(ctx context.Context, labelName string) (*domain.Pest, error)
		FindSymptomsByPestID(ctx context.Context, pestID uuid.UUID) ([]domain.Symptom, error)
	},
	session *domain.ExpertSession,
) string {

	label := ""
	confidence := 0.0

	if session != nil {
		label = session.DetectedLabel
		confidence = session.DetectedConfidence
	}

	selectedSymptoms := []domain.Symptom{}

	if catalogRepo != nil && strings.TrimSpace(label) != "" {
		if pest, err := catalogRepo.FindPestByLabelName(ctx, label); err == nil && pest != nil {
			if symptoms, err := catalogRepo.FindSymptomsByPestID(ctx, pest.ID); err == nil && len(symptoms) > 0 {
				selectedSymptoms = selectStateFlowSymptoms(symptoms, 12)
			}
		}
	}

	builder := strings.Builder{}
	builder.WriteString(
		fmt.Sprintf(
			"Gambar berhasil dianalisis. Deteksi utama: %s (tingkat keyakinan %.0f%%).\n\n",
			emptyFallback(
				FormatPestDisplayName(label),
				"hama padi",
			),
			confidence*100,
		),
	)

	if confidence > 0 && confidence < 0.7 {
		builder.WriteString("Catatan: hasil gambar belum terlalu meyakinkan, sehingga gejala lapangan perlu dipilih dengan teliti sebelum menentukan hasil.\n\n")
	}

	builder.WriteString("Silakan pilih atau tulis gejala yang benar-benar terlihat di sawah. Agar pemeriksaan dapat dilanjutkan, pilih minimal dua gejala: satu tanda keberadaan hama dan satu gejala kerusakan tanaman.\n")

	if len(selectedSymptoms) > 0 {
		builder.WriteString("\nGejala yang dapat diperiksa di sawah:\n")
		for i, symptom := range selectedSymptoms {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, strings.TrimSpace(symptom.Name)))
		}
	}

	examples := buildSymptomExamplesByPest(label)
	if examples != "" {
		builder.WriteString("\nContoh jawaban: ")
		builder.WriteString(examples)
		builder.WriteString(".")
	} else {
		builder.WriteString("\nContoh jawaban: gejala yang benar-benar terlihat pada tanaman padi di lapangan.")
	}

	return strings.TrimSpace(builder.String())
}

func buildSymptomQuestionActions(
	ctx context.Context,
	catalogRepo interface {
		FindPestByLabelName(ctx context.Context, labelName string) (*domain.Pest, error)
		FindSymptomsByPestID(ctx context.Context, pestID uuid.UUID) ([]domain.Symptom, error)
	},
	session *domain.ExpertSession,
) []dto.ChatAction {
	if catalogRepo == nil || session == nil || strings.TrimSpace(session.DetectedLabel) == "" {
		return nil
	}

	pest, err := catalogRepo.FindPestByLabelName(ctx, session.DetectedLabel)
	if err != nil || pest == nil {
		return nil
	}

	symptoms, err := catalogRepo.FindSymptomsByPestID(ctx, pest.ID)
	if err != nil || len(symptoms) == 0 {
		return nil
	}

	selectedSymptoms := selectStateFlowSymptoms(symptoms, 12)
	actions := make([]dto.ChatAction, 0, len(selectedSymptoms))
	for _, symptom := range selectedSymptoms {
		name := strings.TrimSpace(symptom.Name)
		if name == "" {
			continue
		}

		actions = append(actions, dto.ChatAction{
			Type:  dto.ActionSelectSymptom,
			Label: name,
			Value: name,
		})
	}

	return actions
}

func selectStateFlowSymptoms(symptoms []domain.Symptom, limit int) []domain.Symptom {
	if limit <= 0 {
		limit = 12
	}

	strict := make([]domain.Symptom, 0, len(symptoms))
	fallback := make([]domain.Symptom, 0, len(symptoms))

	for _, item := range symptoms {
		if !isStateFlowObservableSymptom(item) {
			continue
		}

		if isStateFlowRecommendedSymptom(item) {
			strict = append(strict, item)
			continue
		}

		if isStateFlowFallbackSymptom(item) {
			fallback = append(fallback, item)
		}
	}

	selected := strict
	if len(selected) == 0 {
		selected = fallback
	}

	identity := make([]domain.Symptom, 0, len(selected))
	damage := make([]domain.Symptom, 0, len(selected))
	other := make([]domain.Symptom, 0, len(selected))

	for _, item := range selected {
		switch symptomEvidenceKind(item) {
		case "identity":
			identity = append(identity, item)
		case "damage":
			damage = append(damage, item)
		default:
			other = append(other, item)
		}
	}

	sortSymptomsByStateFlowScore(identity)
	sortSymptomsByStateFlowScore(damage)
	sortSymptomsByStateFlowScore(other)

	balanced := appendBalancedSymptoms(nil, identity, damage, limit)
	if len(balanced) < limit {
		balanced = append(balanced, other...)
	}

	balanced = deduplicateStateFlowSymptoms(balanced)
	if len(balanced) > limit {
		balanced = balanced[:limit]
	}

	return balanced
}

func isStateFlowObservableSymptom(symptom domain.Symptom) bool {
	name := strings.TrimSpace(symptom.Name)
	if name == "" {
		return false
	}

	if !symptom.UserObservable {
		return false
	}

	text := strings.ToLower(strings.TrimSpace(strings.Join([]string{
		symptom.Name,
		symptom.OriginalName,
		symptom.Description,
		symptom.SymptomType,
		symptom.RuleRole,
		symptom.ExpertNote,
	}, " ")))

	blockedTypes := map[string]struct{}{
		"risk_factor":         {},
		"risk factor":         {},
		"natural_enemy":       {},
		"natural enemy":       {},
		"behavior":            {},
		"behaviour":           {},
		"post_harvest_impact": {},
		"post harvest impact": {},
		"post_harvest":        {},
		"post harvest":        {},
		"vector_disease":      {},
		"vector disease":      {},
		"environment":         {},
		"excluded_from_rule":  {},
		"excluded":            {},
	}

	if _, blocked := blockedTypes[strings.ToLower(strings.TrimSpace(symptom.SymptomType))]; blocked {
		return false
	}

	if _, blocked := blockedTypes[strings.ToLower(strings.TrimSpace(symptom.RuleRole))]; blocked {
		return false
	}

	blockedKeywords := []string{
		"migrasi",
		"populasi naik saat panas",
		"cuaca panas",
		"panas/kering",
		"rumput sekitar",
		"rasa/kualitas nasi",
		"rasa nasi",
		"mudah pecah saat digiling",
		"saat digiling",
		"leher malai rusak sekunder",
		"rusak sekunder",
		"musuh alami",
		"laba-laba",
		"predator",
		"parasitoid",
	}

	for _, keyword := range blockedKeywords {
		if strings.Contains(text, keyword) {
			return false
		}
	}

	return true
}

func isStateFlowRecommendedSymptom(symptom domain.Symptom) bool {
	if !symptom.RecommendedForRule {
		return false
	}

	role := strings.ToLower(strings.TrimSpace(symptom.RuleRole))
	if role == "" {
		return true
	}

	switch role {
	case "identity", "damage", "severity_anchor", "supporting":
		return true
	default:
		return false
	}
}

func isStateFlowFallbackSymptom(symptom domain.Symptom) bool {
	role := strings.ToLower(strings.TrimSpace(symptom.RuleRole))
	symptomType := strings.ToLower(strings.TrimSpace(symptom.SymptomType))

	if role != "" {
		switch role {
		case "identity", "damage", "severity_anchor", "supporting":
			return true
		default:
			return false
		}
	}

	if symptomType != "" {
		switch symptomType {
		case "identity", "damage", "severity_anchor", "supporting", "symptom", "field_symptom":
			return true
		default:
			return false
		}
	}

	return true
}

func scoreStateFlowSymptom(symptom domain.Symptom) float64 {
	score := 0.0

	if symptom.RecommendedForRule {
		score += 100
	}

	if symptom.IsCoreSymptom {
		score += 25
	}

	switch strings.ToLower(strings.TrimSpace(symptom.RuleRole)) {
	case "identity":
		score += 35
	case "damage":
		score += 30
	case "severity_anchor":
		score += 28
	case "supporting":
		score += 12
	}

	switch strings.ToLower(strings.TrimSpace(symptom.FieldReliability)) {
	case "high":
		score += 12
	case "medium":
		score += 6
	}

	switch strings.ToLower(strings.TrimSpace(symptom.DiagnosticSpecificity)) {
	case "high":
		score += 12
	case "medium":
		score += 6
	}

	if symptom.DefaultWeight > 0 {
		score += symptom.DefaultWeight * 10
	}

	return score
}

func deduplicateStateFlowSymptoms(symptoms []domain.Symptom) []domain.Symptom {
	seen := map[string]struct{}{}
	result := make([]domain.Symptom, 0, len(symptoms))

	for _, symptom := range symptoms {
		key := strings.ToLower(strings.TrimSpace(symptom.Name))
		if key == "" {
			continue
		}

		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		result = append(result, symptom)
	}

	return result
}

func buildSymptomExamplesByPest(label string) string {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "walang_sangit", "walang sangit":
		return "walang sangit pada malai, aroma khas walang sangit, bercak coklat pada bulir, bulir hampa sebagian"
	case "wereng_batang_cokelat", "wereng batang cokelat":
		return "wereng di pangkal batang, daun menguning bagian bawah, embun madu pada batang, tanaman hopperburn"
	case "penggerek_batang", "penggerek batang":
		return "lubang kecil pada batang, larva di dalam batang, gejala sundep, malai putih atau beluk"
	default:
		return "gejala utama yang benar-benar terlihat pada tanaman padi"
	}
}

type symptomEvidenceStatus struct {
	MatchedCount int
	HasIdentity  bool
	HasDamage    bool
	Selected     []string
	Unknown      []string
}

func buildAdditionalSymptomQuestionResponse(
	ctx context.Context,
	catalogRepo interface {
		FindPestByLabelName(ctx context.Context, labelName string) (*domain.Pest, error)
		FindSymptomsByPestID(ctx context.Context, pestID uuid.UUID) ([]domain.Symptom, error)
	},
	session *domain.ExpertSession,
) *dto.OrchestratorResponse {
	evidence := symptomEvidenceFromSession(session)
	message := strings.Builder{}

	switch {
	case !evidence.HasIdentity && !evidence.HasDamage:
		message.WriteString("Gejala yang dipilih sudah dicatat, tetapi belum membentuk bukti lapangan yang cukup.\n\n")
		message.WriteString("Agar pemeriksaan dapat dilanjutkan, pilih minimal satu tanda keberadaan hama dan satu gejala kerusakan tanaman yang benar-benar terlihat di sawah.\n")
	case !evidence.HasIdentity:
		message.WriteString("Gejala kerusakan tanaman sudah dicatat.\n\n")
		message.WriteString("Sekarang tambahkan satu tanda keberadaan hama, misalnya hama/larva terlihat, lubang gerekan, serbuk gerekan, embun madu, atau tanda langsung lain yang sesuai.\n")
	case !evidence.HasDamage:
		message.WriteString("Tanda keberadaan hama sudah dicatat.\n\n")
		message.WriteString("Sekarang tambahkan satu gejala kerusakan atau tanda tingkat serangan, misalnya bagian tanaman menguning, anakan mati, malai putih, bulir hampa, hopperburn, atau kerusakan lain yang sesuai.\n")
	default:
		message.WriteString("Gejala sudah dicatat, tetapi sistem masih memerlukan satu gejala lapangan lain agar hasil tidak ditentukan dari satu tanda saja.\n")
	}

	if len(evidence.Selected) > 0 {
		message.WriteString("\nGejala yang sudah tersimpan:\n")
		for i, name := range evidence.Selected {
			if i >= 5 {
				message.WriteString(fmt.Sprintf("- dan %d gejala lainnya\n", len(evidence.Selected)-i))
				break
			}
			message.WriteString("- " + name + "\n")
		}
	}

	candidates := selectMissingEvidenceSymptoms(ctx, catalogRepo, session, evidence, 8)
	actions := make([]dto.ChatAction, 0, len(candidates))

	if len(candidates) > 0 {
		message.WriteString("\nPilihan gejala berikutnya:\n")
		for i, symptom := range candidates {
			name := strings.TrimSpace(symptom.Name)
			message.WriteString(fmt.Sprintf("%d. %s (%s)\n", i+1, name, symptomEvidenceLabel(symptom)))
			actions = append(actions, dto.ChatAction{
				Type:  dto.ActionSelectSymptom,
				Label: name,
				Value: name,
			})
		}
	} else {
		message.WriteString("\nPilihan otomatis belum tersedia. Tulis gejala yang terlihat dengan kalimat singkat, misalnya hama terlihat di pangkal batang atau daun bagian bawah menguning.")
	}

	if len(evidence.Unknown) > 0 {
		message.WriteString("\nCatatan: ada pilihan sebelumnya yang belum dikenali. Pilih salah satu gejala yang tersedia atau tulis gejala dengan kalimat singkat dan jelas.")
	}

	return &dto.OrchestratorResponse{
		SessionID: session.ID,
		Mode:      dto.ConversationModeDiagnose,
		State:     dto.DiagnoseFlowStateCollectSymptoms,
		Message:   strings.TrimSpace(message.String()),
		Actions:   actions,
		CreatedAt: time.Now(),
	}
}

func symptomEvidenceFromSession(session *domain.ExpertSession) symptomEvidenceStatus {
	status := symptomEvidenceStatus{}
	if session == nil || len(session.Symptoms) == 0 {
		return status
	}

	var data diagnose.SymptomSessionData
	if err := json.Unmarshal(session.Symptoms, &data); err != nil {
		return status
	}

	seenSelected := map[string]struct{}{}
	for _, item := range data.Normalized {
		name := strings.TrimSpace(item.MatchedSymptomName)
		if name == "" {
			name = strings.TrimSpace(item.InputText)
		}
		if name != "" {
			key := strings.ToLower(name)
			if _, exists := seenSelected[key]; !exists {
				seenSelected[key] = struct{}{}
				status.Selected = append(status.Selected, name)
			}
		}

		if item.MatchedSymptomID == nil || item.Confidence < 0.65 {
			continue
		}

		status.MatchedCount++
		role := strings.ToLower(strings.TrimSpace(item.RuleRole))
		symptomType := strings.ToLower(strings.TrimSpace(item.SymptomType))

		if role == "identity" || symptomType == "identity" {
			status.HasIdentity = true
		}
		if role == "damage" || role == "severity_anchor" || symptomType == "damage" || symptomType == "severity_anchor" {
			status.HasDamage = true
		}
	}

	status.Unknown = append(status.Unknown, data.Unknown...)
	return status
}

func selectMissingEvidenceSymptoms(
	ctx context.Context,
	catalogRepo interface {
		FindPestByLabelName(ctx context.Context, labelName string) (*domain.Pest, error)
		FindSymptomsByPestID(ctx context.Context, pestID uuid.UUID) ([]domain.Symptom, error)
	},
	session *domain.ExpertSession,
	evidence symptomEvidenceStatus,
	limit int,
) []domain.Symptom {
	if catalogRepo == nil || session == nil || strings.TrimSpace(session.DetectedLabel) == "" {
		return nil
	}
	if limit <= 0 {
		limit = 8
	}

	pest, err := catalogRepo.FindPestByLabelName(ctx, session.DetectedLabel)
	if err != nil || pest == nil {
		return nil
	}

	symptoms, err := catalogRepo.FindSymptomsByPestID(ctx, pest.ID)
	if err != nil || len(symptoms) == 0 {
		return nil
	}

	alreadySelected := selectedSymptomNamesFromSession(session)
	identityCandidates := make([]domain.Symptom, 0, len(symptoms))
	damageCandidates := make([]domain.Symptom, 0, len(symptoms))

	for _, symptom := range symptoms {
		if !isStateFlowObservableSymptom(symptom) || !isStateFlowRecommendedSymptom(symptom) {
			continue
		}
		if _, exists := alreadySelected[strings.ToLower(strings.TrimSpace(symptom.Name))]; exists {
			continue
		}

		switch symptomEvidenceKind(symptom) {
		case "identity":
			identityCandidates = append(identityCandidates, symptom)
		case "damage":
			damageCandidates = append(damageCandidates, symptom)
		}
	}

	sortSymptomsByStateFlowScore(identityCandidates)
	sortSymptomsByStateFlowScore(damageCandidates)

	result := make([]domain.Symptom, 0, limit)
	switch {
	case !evidence.HasIdentity && !evidence.HasDamage:
		result = appendBalancedSymptoms(result, identityCandidates, damageCandidates, limit)
	case !evidence.HasIdentity:
		result = append(result, identityCandidates...)
	case !evidence.HasDamage:
		result = append(result, damageCandidates...)
	default:
		result = appendBalancedSymptoms(result, identityCandidates, damageCandidates, limit)
	}

	result = deduplicateStateFlowSymptoms(result)
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

func appendBalancedSymptoms(result []domain.Symptom, identity []domain.Symptom, damage []domain.Symptom, limit int) []domain.Symptom {
	identityIndex := 0
	damageIndex := 0
	for len(result) < limit && (identityIndex < len(identity) || damageIndex < len(damage)) {
		if identityIndex < len(identity) {
			result = append(result, identity[identityIndex])
			identityIndex++
		}
		if len(result) >= limit {
			break
		}
		if damageIndex < len(damage) {
			result = append(result, damage[damageIndex])
			damageIndex++
		}
	}
	return result
}

func sortSymptomsByStateFlowScore(symptoms []domain.Symptom) {
	sort.SliceStable(symptoms, func(i, j int) bool {
		left := scoreStateFlowSymptom(symptoms[i])
		right := scoreStateFlowSymptom(symptoms[j])
		if left == right {
			return strings.ToLower(symptoms[i].Name) < strings.ToLower(symptoms[j].Name)
		}
		return left > right
	})
}

func symptomEvidenceKind(symptom domain.Symptom) string {
	role := strings.ToLower(strings.TrimSpace(symptom.RuleRole))
	symptomType := strings.ToLower(strings.TrimSpace(symptom.SymptomType))
	if role == "identity" || symptomType == "identity" {
		return "identity"
	}
	if role == "damage" || role == "severity_anchor" || symptomType == "damage" || symptomType == "severity_anchor" {
		return "damage"
	}
	return ""
}

func symptomEvidenceLabel(symptom domain.Symptom) string {
	if symptomEvidenceKind(symptom) == "identity" {
		return "tanda keberadaan hama"
	}
	return "gejala kerusakan"
}

func selectedSymptomNamesFromSession(session *domain.ExpertSession) map[string]struct{} {
	selected := map[string]struct{}{}
	if session == nil || len(session.Symptoms) == 0 {
		return selected
	}

	var data diagnose.SymptomSessionData
	if err := json.Unmarshal(session.Symptoms, &data); err != nil {
		return selected
	}

	for _, item := range data.Normalized {
		for _, value := range []string{item.InputText, item.MatchedSymptomName, item.MatchedText} {
			value = strings.ToLower(strings.TrimSpace(value))
			if value == "" {
				continue
			}
			selected[value] = struct{}{}
		}
	}

	return selected
}

type severityEvidenceResult struct {
	Rank           int
	Label          string
	AnchorSymptoms []string
}

func buildSeverityEvidenceConflictResponse(sessionID uuid.UUID, session *domain.ExpertSession, selectedSeverity string) *dto.OrchestratorResponse {
	evidence := inferSeverityEvidenceFromSession(session)
	selectedRank := severityRank(selectedSeverity)

	if evidence.Rank <= selectedRank || evidence.Rank <= 0 {
		return nil
	}

	if evidence.Rank < severityRank("berat") {
		return nil
	}

	anchors := strings.Join(evidence.AnchorSymptoms, ", ")
	if strings.TrimSpace(anchors) == "" {
		anchors = "gejala yang dipilih"
	}

	message := fmt.Sprintf(`Tingkat keparahan yang dipilih belum sesuai dengan gejala lapangan yang sudah dicatat.

Gejala %s termasuk indikator serangan berat. Pada kasus wereng batang cokelat, hopperburn/tanaman seperti terbakar biasanya menunjukkan kerusakan serius dan tidak tepat jika dikategorikan sebagai sedang atau ringan.

Jika gejala tersebut benar-benar terlihat di sawah, pilih "berat". Jika yang terlihat belum sampai hopperburn, kembali tambahkan atau koreksi gejala kerusakan yang lebih sesuai, misalnya daun mulai menguning atau pertumbuhan anakan melambat.`, anchors)

	return &dto.OrchestratorResponse{
		SessionID: sessionID,
		Mode:      dto.ConversationModeDiagnose,
		State:     dto.DiagnoseFlowStateCollectSeverity,
		Message:   strings.TrimSpace(message),
		Actions: []dto.ChatAction{
			{Type: dto.ActionSelectSeverity, Label: "Berat", Value: "berat"},
			{Type: dto.ActionSelectSeverity, Label: "Sedang", Value: "sedang"},
			{Type: dto.ActionSelectSeverity, Label: "Ringan", Value: "ringan"},
		},
		CreatedAt: time.Now(),
	}
}

func inferSeverityEvidenceFromSession(session *domain.ExpertSession) severityEvidenceResult {
	result := severityEvidenceResult{}

	if session == nil || len(session.Symptoms) == 0 {
		return result
	}

	var data diagnose.SymptomSessionData
	if err := json.Unmarshal(session.Symptoms, &data); err != nil {
		return result
	}

	seenAnchors := map[string]struct{}{}

	for _, item := range data.Normalized {
		if item.MatchedSymptomID == nil || item.Confidence < 0.65 {
			continue
		}

		rank := severityRankFromSymptomEvidence(item)
		if rank <= 0 {
			continue
		}

		if rank > result.Rank {
			result.Rank = rank
			result.AnchorSymptoms = result.AnchorSymptoms[:0]
		}

		if rank == result.Rank {
			name := strings.TrimSpace(item.MatchedSymptomName)
			if name == "" {
				name = strings.TrimSpace(item.InputText)
			}
			key := strings.ToLower(name)
			if name != "" {
				if _, exists := seenAnchors[key]; !exists {
					seenAnchors[key] = struct{}{}
					result.AnchorSymptoms = append(result.AnchorSymptoms, name)
				}
			}
		}
	}

	switch result.Rank {
	case 3:
		result.Label = "berat"
	case 2:
		result.Label = "sedang"
	case 1:
		result.Label = "ringan"
	}

	return result
}

func severityRankFromSymptomEvidence(item diagnose.NormalizedSymptomData) int {
	role := strings.ToLower(strings.TrimSpace(item.RuleRole))
	symptomType := strings.ToLower(strings.TrimSpace(item.SymptomType))

	// Gejala identitas hama, misalnya "wereng di pangkal batang", tidak boleh
	// menaikkan severity. Severity hanya ditentukan oleh gejala kerusakan
	// atau anchor tingkat serangan.
	if role == "identity" || symptomType == "identity" {
		return 0
	}

	severityMetadata := ""
	if role == "damage" || role == "severity_anchor" || symptomType == "damage" || symptomType == "severity_anchor" {
		severityMetadata = item.Severity
	}

	combined := strings.ToLower(strings.TrimSpace(strings.Join([]string{
		severityMetadata,
		item.SymptomType,
		item.RuleRole,
		item.MatchedSymptomName,
		item.MatchedText,
		item.InputText,
	}, " ")))

	if combined == "" {
		return 0
	}

	heavyKeywords := []string{
		"berat",
		"tinggi",
		"high",
		"parah",
		"hopperburn",
		"tanaman seperti terbakar",
		"seperti terbakar",
		"terbakar",
		"gosong",
		"mati serempak",
		"rumpun mati",
		"gagal panen",
		"hampa tinggi",
		"kehilangan hasil",
	}

	for _, keyword := range heavyKeywords {
		if strings.Contains(combined, keyword) {
			return 3
		}
	}

	mediumKeywords := []string{
		"sedang",
		"medium",
		"kerusakan mulai nyata",
		"melambat",
		"menurun",
		"menguning",
		"layu",
		"coklat tua",
		"embun madu",
		"jamur jelaga",
	}

	for _, keyword := range mediumKeywords {
		if strings.Contains(combined, keyword) {
			return 2
		}
	}

	lowKeywords := []string{
		"ringan",
		"rendah",
		"low",
		"awal",
		"mulai",
		"bekas kulit",
		"menggulung",
	}

	for _, keyword := range lowKeywords {
		if strings.Contains(combined, keyword) {
			return 1
		}
	}

	return 0
}

func severityRank(severity string) int {
	severity = strings.ToLower(strings.TrimSpace(severity))

	switch severity {
	case "berat", "tinggi", "parah", "high":
		return 3
	case "sedang", "medium", "moderate":
		return 2
	case "ringan", "rendah", "low":
		return 1
	default:
		return 0
	}
}

func buildSeverityQuestionResponse(sessionID uuid.UUID, session *domain.ExpertSession) *dto.OrchestratorResponse {

	message := "Gejala sudah dicatat. Sekarang pilih tingkat keparahan serangan (ringan, sedang, atau berat)."
	severityEvidence := inferSeverityEvidenceFromSession(session)

	if severityEvidence.Rank >= severityRank("berat") {
		message = fmt.Sprintf(
			"Gejala sudah dicatat. Catatan: %s termasuk indikator serangan berat di lapangan. Jika gejala tersebut benar-benar terlihat, pilih tingkat keparahan: berat.",
			strings.Join(severityEvidence.AnchorSymptoms, ", "),
		)
	} else if severityEvidence.Rank >= severityRank("sedang") {
		message = fmt.Sprintf(
			"Gejala sudah dicatat. Catatan: %s menunjukkan kerusakan yang mulai nyata. Pilih tingkat keparahan yang paling sesuai: sedang atau berat jika kerusakan sudah meluas.",
			strings.Join(severityEvidence.AnchorSymptoms, ", "),
		)
	}

	actions := []dto.ChatAction{
		{Type: dto.ActionSelectSeverity, Label: "Ringan", Value: "ringan"},
		{Type: dto.ActionSelectSeverity, Label: "Sedang", Value: "sedang"},
		{Type: dto.ActionSelectSeverity, Label: "Berat", Value: "berat"},
	}

	if severityEvidence.Rank >= severityRank("berat") {
		actions = []dto.ChatAction{
			{Type: dto.ActionSelectSeverity, Label: "Berat", Value: "berat"},
			{Type: dto.ActionSelectSeverity, Label: "Sedang", Value: "sedang"},
			{Type: dto.ActionSelectSeverity, Label: "Ringan", Value: "ringan"},
		}
	}

	return &dto.OrchestratorResponse{
		SessionID: sessionID,
		Mode:      dto.ConversationModeDiagnose,
		State:     dto.DiagnoseFlowStateCollectSeverity,
		Message:   message,
		Actions:   actions,
		CreatedAt: time.Now(),
	}
}

type growthStageEvidenceResult struct {
	RequiredStage string
	Reason        string
	Symptoms      []string
}

func buildGrowthStageEvidenceConflictResponse(sessionID uuid.UUID, session *domain.ExpertSession, selectedStage string) *dto.OrchestratorResponse {
	evidence := inferGrowthStageEvidenceFromSession(session)
	if evidence.RequiredStage == "" || selectedStage == "" || evidence.RequiredStage == selectedStage {
		return nil
	}

	symptoms := strings.Join(evidence.Symptoms, ", ")
	if strings.TrimSpace(symptoms) == "" {
		symptoms = "gejala yang dipilih"
	}

	message := fmt.Sprintf(`Fase pertumbuhan yang dipilih belum sesuai dengan gejala lapangan yang sudah dicatat.

Gejala %s lebih sesuai dengan fase %s. %s

Silakan pilih fase pertumbuhan yang sesuai agar rule diagnosis tidak salah mengunci hasil.`, symptoms, evidence.RequiredStage, evidence.Reason)

	actions := []dto.ChatAction{
		{Type: dto.ActionSelectGrowth, Label: "Generatif", Value: "generatif"},
		{Type: dto.ActionSelectGrowth, Label: "Vegetatif", Value: "vegetatif"},
	}
	if evidence.RequiredStage == "vegetatif" {
		actions = []dto.ChatAction{
			{Type: dto.ActionSelectGrowth, Label: "Vegetatif", Value: "vegetatif"},
			{Type: dto.ActionSelectGrowth, Label: "Generatif", Value: "generatif"},
		}
	}

	return &dto.OrchestratorResponse{
		SessionID: sessionID,
		Mode:      dto.ConversationModeDiagnose,
		State:     dto.DiagnoseFlowStateCollectGrowthStage,
		Message:   strings.TrimSpace(message),
		Actions:   actions,
		CreatedAt: time.Now(),
	}
}

func inferGrowthStageEvidenceFromSession(session *domain.ExpertSession) growthStageEvidenceResult {
	result := growthStageEvidenceResult{}
	if session == nil || len(session.Symptoms) == 0 {
		return result
	}

	var data diagnose.SymptomSessionData
	if err := json.Unmarshal(session.Symptoms, &data); err != nil {
		return result
	}

	// Prioritaskan metadata fase dari gejala kerusakan yang sudah cocok dengan
	// katalog. Bukti identitas seperti larva/wereng dapat muncul pada dua fase
	// dan tidak boleh mengalahkan gejala spesifik seperti anakan (vegetatif)
	// atau malai/gabah (generatif).
	if stage, names := inferExclusiveDamageGrowthStage(data.Normalized); stage != "" {
		reason := "Gejala kerusakan yang dipilih hanya tercatat untuk fase " + stage + " pada basis pengetahuan."
		return growthStageEvidenceResult{
			RequiredStage: stage,
			Reason:        reason,
			Symptoms:      names,
		}
	}

	label := strings.ToLower(session.DetectedLabel)
	combinedParts := []string{session.DetectedLabel}
	for _, item := range data.Normalized {
		combinedParts = append(combinedParts,
			item.GrowthStage,
			item.MatchedSymptomName,
			item.MatchedText,
			item.InputText,
			item.NormalizedText,
		)
	}
	combined := strings.ToLower(strings.Join(combinedParts, " "))

	// Walang sangit adalah hama fase generatif karena gejala kuncinya terkait bulir/gabah.
	// Validasi ini dibuat paling awal agar user tidak mengunci fase vegetatif.
	if strings.Contains(label, "walang") || containsAnyText(combined, "walang sangit") {
		return growthStageEvidenceResult{
			RequiredStage: "generatif",
			Reason:        "Walang sangit umumnya divalidasi pada fase generatif karena menyerang bulir/gabah yang sedang berkembang.",
			Symptoms:      collectGrowthStageSymptomNames(data.Normalized),
		}
	}

	strongGenerative := false
	strongVegetative := false
	metadataGenerative := false
	metadataVegetative := false
	generativeSymptoms := make([]string, 0)
	vegetativeSymptoms := make([]string, 0)

	for _, item := range data.Normalized {
		name := strings.TrimSpace(item.MatchedSymptomName)
		if name == "" {
			name = strings.TrimSpace(item.InputText)
		}

		// Prioritaskan isi gejala yang terlihat user, bukan metadata growth_stage.
		// Gejala identitas seperti larva dapat muncul pada lebih dari satu fase, sedangkan
		// malai/bulir/beluk adalah anchor generatif yang harus mengalahkan metadata campuran.
		contentText := strings.ToLower(strings.Join([]string{
			item.MatchedSymptomName,
			item.MatchedText,
			item.InputText,
			item.NormalizedText,
		}, " "))
		stageMeta := strings.ToLower(item.GrowthStage)

		if containsAnyText(contentText,
			"malai", "beluk", "whitehead", "bulir", "gabah", "hampa", "masak susu", "berbunga", "berisi susu",
		) {
			strongGenerative = true
			if name != "" {
				generativeSymptoms = appendUniqueString(generativeSymptoms, name)
			}
		}

		if containsAnyText(contentText,
			"sundep", "deadheart", "anakan", "tunas", "pertumbuhan anakan",
		) {
			strongVegetative = true
			if name != "" {
				vegetativeSymptoms = appendUniqueString(vegetativeSymptoms, name)
			}
		}

		// Metadata growth_stage hanya dipakai jika tidak campuran.
		// Contoh "vegetatif,generatif" tidak boleh membatalkan anchor generatif seperti malai kosong.
		metaHasGenerative := containsAnyText(stageMeta, "generatif", "reproduktif")
		metaHasVegetative := containsAnyText(stageMeta, "vegetatif")
		if metaHasGenerative && !metaHasVegetative {
			metadataGenerative = true
			if name != "" {
				generativeSymptoms = appendUniqueString(generativeSymptoms, name)
			}
		}
		if metaHasVegetative && !metaHasGenerative {
			metadataVegetative = true
			if name != "" {
				vegetativeSymptoms = appendUniqueString(vegetativeSymptoms, name)
			}
		}
	}

	if strongGenerative && !strongVegetative {
		return growthStageEvidenceResult{
			RequiredStage: "generatif",
			Reason:        "Gejala seperti malai, bulir/gabah hampa, beluk, atau masak susu merupakan indikator fase generatif.",
			Symptoms:      generativeSymptoms,
		}
	}

	if strongVegetative && !strongGenerative {
		return growthStageEvidenceResult{
			RequiredStage: "vegetatif",
			Reason:        "Gejala seperti sundep, anakan terganggu, atau kerusakan tunas lebih sesuai dengan fase vegetatif.",
			Symptoms:      vegetativeSymptoms,
		}
	}

	// Jika ada anchor generatif dan vegetatif sekaligus, jangan memaksa salah satu fase.
	// Biarkan rule engine/pertanyaan lanjutan menangani kombinasi gejala yang benar-benar campuran.
	if strongGenerative && strongVegetative {
		return result
	}

	if metadataGenerative && !metadataVegetative {
		return growthStageEvidenceResult{
			RequiredStage: "generatif",
			Reason:        "Metadata gejala yang tercatat lebih sesuai dengan fase generatif.",
			Symptoms:      generativeSymptoms,
		}
	}

	if metadataVegetative && !metadataGenerative {
		return growthStageEvidenceResult{
			RequiredStage: "vegetatif",
			Reason:        "Metadata gejala yang tercatat lebih sesuai dengan fase vegetatif.",
			Symptoms:      vegetativeSymptoms,
		}
	}

	return result
}

func inferExclusiveDamageGrowthStage(items []diagnose.NormalizedSymptomData) (string, []string) {
	hasVegetativeOnly := false
	hasGenerativeOnly := false
	vegetativeNames := make([]string, 0)
	generativeNames := make([]string, 0)

	for _, item := range items {
		if item.MatchedSymptomID == nil || item.Confidence < 0.65 {
			continue
		}

		role := strings.ToLower(strings.TrimSpace(item.RuleRole))
		symptomType := strings.ToLower(strings.TrimSpace(item.SymptomType))
		if role == "identity" || symptomType == "identity" || role == "supporting" || symptomType == "supporting" {
			continue
		}
		if !(role == "damage" || role == "severity_anchor" || symptomType == "damage" || symptomType == "severity_anchor") {
			continue
		}

		stageMeta := strings.ToLower(strings.TrimSpace(item.GrowthStage))
		hasVegetative := metadataContainsStage(stageMeta, "vegetatif")
		hasGenerative := metadataContainsStage(stageMeta, "generatif")
		if hasVegetative == hasGenerative {
			// metadata kosong atau mendukung kedua fase tidak mengunci pilihan.
			continue
		}

		name := strings.TrimSpace(item.MatchedSymptomName)
		if name == "" {
			name = strings.TrimSpace(item.InputText)
		}

		if hasVegetative {
			hasVegetativeOnly = true
			vegetativeNames = appendUniqueString(vegetativeNames, name)
		}
		if hasGenerative {
			hasGenerativeOnly = true
			generativeNames = appendUniqueString(generativeNames, name)
		}
	}

	switch {
	case hasVegetativeOnly && !hasGenerativeOnly:
		return "vegetatif", vegetativeNames
	case hasGenerativeOnly && !hasVegetativeOnly:
		return "generatif", generativeNames
	default:
		return "", nil
	}
}

func metadataContainsStage(raw string, expected string) bool {
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == '|' || r == ',' || r == ';' || r == '/'
	}) {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == expected {
			return true
		}
		if expected == "generatif" && strings.Contains(part, "reproduktif") {
			return true
		}
	}
	return false
}

func collectGrowthStageSymptomNames(items []diagnose.NormalizedSymptomData) []string {
	names := make([]string, 0)
	for _, item := range items {
		name := strings.TrimSpace(item.MatchedSymptomName)
		if name == "" {
			name = strings.TrimSpace(item.InputText)
		}
		if name != "" {
			names = appendUniqueString(names, name)
		}
	}
	return names
}

func containsAnyText(value string, keywords ...string) bool {
	value = strings.ToLower(value)
	for _, keyword := range keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword != "" && strings.Contains(value, keyword) {
			return true
		}
	}
	return false
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	key := strings.ToLower(value)
	for _, existing := range values {
		if strings.ToLower(strings.TrimSpace(existing)) == key {
			return values
		}
	}
	return append(values, value)
}

func buildGrowthStageQuestionResponse(sessionID uuid.UUID) *dto.OrchestratorResponse {

	return &dto.OrchestratorResponse{
		SessionID: sessionID,
		Mode:      dto.ConversationModeDiagnose,
		State:     dto.DiagnoseFlowStateCollectGrowthStage,
		Message:   "Baik. Sekarang pilih fase pertumbuhan padi saat gejala terlihat (vegetatif atau generatif).",
		Actions: []dto.ChatAction{
			{Type: dto.ActionSelectGrowth, Label: "Vegetatif", Value: "vegetatif"},
			{Type: dto.ActionSelectGrowth, Label: "Generatif", Value: "generatif"},
		},
		CreatedAt: time.Now(),
	}
}

func buildCompletedResponse(sessionID uuid.UUID, session *domain.ExpertSession, recommendationResult *DiagnosisRecommendationResult) *dto.OrchestratorResponse {

	return &dto.OrchestratorResponse{
		SessionID: sessionID,
		Mode:      dto.ConversationModeConsultation,
		State:     dto.DiagnoseFlowStateCompleted,
		Message: buildDiagnosisSummary(
			session,
			recommendationResult,
		),
		Payload: buildDiagnoseResultPayload(
			session,
			recommendationResult,
		),
		Actions: []dto.ChatAction{
			{
				Type:  dto.ActionContinueConsult,
				Label: "Tanya AgriPadi",
				Value: "consultation",
			},
			{
				Type:  dto.ActionUploadImage,
				Label: "Diagnosa Gambar Baru",
				Value: "upload_image",
			},
		},
		CreatedAt: time.Now(),
	}
}

func buildDiagnoseResultPayload(session *domain.ExpertSession, recommendationResult *DiagnosisRecommendationResult) *dto.OrchestratorPayload {
	if session == nil {
		return nil
	}

	payload := &dto.OrchestratorPayload{
		Diagnose: &dto.DiagnosePayload{
			Session: dto.DiagnoseSessionResponse{
				SessionID:      session.ID,
				ConversationID: session.ConversationID,
				Mode:           dto.ConversationModeDiagnose,
				State:          dto.DiagnoseFlowStateCompleted,
				Metadata: dto.DiagnoseSessionMetadata{
					IsCompleted:   session.IsCompleted,
					IsFallbackLLM: false,
				},
				CreatedAt: session.CreatedAt,
				UpdatedAt: session.UpdatedAt,
			},
		},
	}

	if recommendationResult != nil {
		payload.Diagnose.Result = buildDiagnosisResultResponse(recommendationResult)
	}

	return payload
}

func buildDiagnosisResultResponse(result *DiagnosisRecommendationResult) *dto.DiagnosisResultResponse {
	if result == nil {
		return nil
	}

	pestRef := dto.PestReference{}
	if result.Pest != nil {
		pestRef = dto.PestReference{
			ID:        result.Pest.ID,
			Name:      result.Pest.Name,
			LabelName: result.Pest.LabelName,
		}
	}

	severityRef := dto.SeverityReference{Name: result.Severity}
	growthRef := dto.GrowthStageReference{Name: result.GrowthStage}
	matchedRuleCode := ""
	var matchedRuleID *uuid.UUID
	reasoning := []string{}

	if result.RuleResult != nil {
		reasoning = append(reasoning, result.RuleResult.Explainability.Reasoning...)
		if result.RuleResult.BestMatch != nil {
			rule := result.RuleResult.BestMatch.Candidate.Rule
			matchedRuleCode = rule.Code
			ruleID := rule.ID
			matchedRuleID = &ruleID
			if rule.Severity.ID != uuid.Nil {
				severityRef.ID = rule.Severity.ID
			}
			if rule.Severity.Name != "" {
				severityRef.Name = rule.Severity.Name
			}
			if rule.GrowthStage.ID != uuid.Nil {
				growthRef.ID = rule.GrowthStage.ID
			}
			if rule.GrowthStage.Name != "" {
				growthRef.Name = rule.GrowthStage.Name
			}
		}
	}

	if matchedRuleCode == "" {
		matchedRuleCode = result.RuleCode
	}

	var matchedRuleCodePtr *string
	if strings.TrimSpace(matchedRuleCode) != "" {
		matchedRuleCodePtr = &matchedRuleCode
	}

	return &dto.DiagnosisResultResponse{
		MatchedRuleID: matchedRuleID,
		Pest:          pestRef,
		Severity:      severityRef,
		GrowthStage:   growthRef,
		Confidence: dto.DiagnosisConfidence{
			CNN:                  result.DetectionConfidence,
			SymptomNormalization: 1,
			RuleEngine:           result.RuleConfidence,
			FinalScore:           finalDiagnosisScore(result.DetectionConfidence, result.RuleConfidence),
		},
		MatchedSymptoms: buildMatchedSymptomReferences(result.Symptoms),
		Recommendations: result.Recommendations,
		Deterministic: dto.DeterministicDiagnosisResponse{
			Summary:         buildDiagnosisNarrative(result),
			MatchedRuleCode: matchedRuleCodePtr,
			Reasoning:       reasoning,
		},
		LLM:              buildLLMExplanationResponse(result),
		ExpertValidation: BuildExpertValidationResponse(result),
	}
}

func buildMatchedSymptomReferences(symptoms []string) []dto.SymptomReference {
	items := make([]dto.SymptomReference, 0, len(symptoms))
	seen := map[string]struct{}{}
	for _, symptom := range symptoms {
		symptom = strings.TrimSpace(symptom)
		if symptom == "" {
			continue
		}
		key := strings.ToLower(symptom)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, dto.SymptomReference{Name: symptom})
	}
	return items
}

func buildLLMExplanationResponse(result *DiagnosisRecommendationResult) *dto.LLMExplanationResponse {
	if result == nil {
		return nil
	}

	summary := safeLLMSummaryForFarmer(result)
	if summary == "" {
		return nil
	}

	model := strings.TrimSpace(result.LLMModel)
	if model == "" {
		model = "deterministic_fallback"
	}
	source := "groq"
	if result.LLMFallback {
		source = "deterministic_fallback"
	}

	return &dto.LLMExplanationResponse{
		Model:      model,
		Source:     source,
		Fallback:   result.LLMFallback,
		Summary:    summary,
		Reasoning:  limitStrings(splitNonEmptyLines(result.LLMSelectionReason), 3),
		Prevention: limitStrings(splitNonEmptyLines(result.LLMSeverityAction), 4),
	}
}

func safeLLMSummaryForFarmer(result *DiagnosisRecommendationResult) string {
	if result == nil {
		return ""
	}

	text := strings.TrimSpace(result.LLMSummary)
	if text == "" {
		return ""
	}

	lower := strings.ToLower(text)
	currentPest := ""
	if result.Pest != nil {
		currentPest = canonicalPestName(result.Pest.Name)
	}

	for _, pestName := range []string{"penggerek batang", "walang sangit", "wereng batang cokelat"} {
		if pestName != currentPest && strings.Contains(lower, pestName) {
			return ""
		}
	}

	// Ringkasan tidak boleh memuat dosis atau angka aplikasi. Informasi tersebut
	// hanya boleh berasal dari data label yang ditampilkan secara deterministik.
	if llmDoseMentionPattern.MatchString(text) {
		return ""
	}

	replacer := strings.NewReplacer(
		"CNN", "pemeriksaan gambar",
		"cnn", "pemeriksaan gambar",
		"rule engine", "pencocokan gejala",
		"Rule Engine", "pencocokan gejala",
		"backend", "sistem",
		"Backend", "sistem",
		"database", "data yang tersedia",
		"Database", "data yang tersedia",
		"LLM", "penjelasan",
	)
	text = strings.TrimSpace(replacer.Replace(text))

	if len([]rune(text)) > 240 {
		runes := []rune(text)
		text = strings.TrimSpace(string(runes[:240])) + "…"
	}

	return text
}

func canonicalPestName(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(normalized, "penggerek batang"):
		return "penggerek batang"
	case strings.Contains(normalized, "walang sangit"):
		return "walang sangit"
	case strings.Contains(normalized, "wereng batang cokelat"):
		return "wereng batang cokelat"
	default:
		return normalized
	}
}

func splitNonEmptyLines(value string) []string {
	lines := strings.Split(value, "\n")
	results := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimLeft(line, "- "))
		if line != "" {
			results = append(results, line)
		}
	}
	return results
}

func finalDiagnosisScore(cnn float64, rule float64) float64 {
	if cnn <= 0 && rule <= 0 {
		return 0
	}
	if cnn <= 0 {
		return rule
	}
	if rule <= 0 {
		return cnn
	}
	return (cnn * 0.4) + (rule * 0.6)
}

func splitUserSelections(input string) []string {

	parts := symptomSelectionSplitter.Split(input, -1)

	results := make(
		[]string,
		0,
		len(parts),
	)

	for _, part := range parts {
		item := strings.TrimSpace(part)
		item = strings.TrimLeft(item, "-*0123456789. )")
		item = strings.TrimSpace(item)

		if item == "" {
			continue
		}

		results = append(
			results,
			item,
		)
	}

	return results
}

func normalizeSeverityInput(input string) string {

	normalized := strings.ToLower(
		strings.TrimSpace(input),
	)

	switch normalized {
	case "1", "low", "ringan", "rendah":
		return "ringan"

	case "2", "medium", "moderate", "sedang":
		return "sedang"

	case "3", "high", "berat", "parah", "tinggi":
		return "berat"
	}

	if strings.Contains(normalized, "tinggi") ||
		strings.Contains(normalized, "parah") ||
		strings.Contains(normalized, "berat") {

		return "berat"
	}

	if strings.Contains(normalized, "sedang") ||
		strings.Contains(normalized, "medium") {

		return "sedang"
	}

	return "ringan"
}

func normalizeGrowthStageInput(input string) string {

	normalized := strings.ToLower(
		strings.TrimSpace(input),
	)

	switch normalized {

	case "1", "vegetatif", "vegetative", "anakan":
		return "vegetatif"

	case "2", "generatif", "reproduktif", "berbunga", "bunting":
		return "generatif"

	}

	if strings.Contains(normalized, "veget") ||
		strings.Contains(normalized, "anakan") {

		return "vegetatif"
	}

	if strings.Contains(normalized, "gener") ||
		strings.Contains(normalized, "bunga") ||
		strings.Contains(normalized, "bunting") {

		return "generatif"
	}

	return "vegetatif"
}

func buildDiagnosisSummary(session *domain.ExpertSession, recommendationResult *DiagnosisRecommendationResult) string {

	if session == nil {
		return "Pemeriksaan selesai.\nAnda dapat bertanya lebih lanjut atau memeriksa foto lain."
	}

	if isNoPestDetectedLabel(
		session.DetectedLabel,
	) {

		return `Pemeriksaan selesai.

Foto belum menunjukkan tanda yang cukup kuat dari tiga hama yang diperiksa.

Jangan gunakan pestisida hanya berdasarkan foto ini. Lanjutkan pemantauan dan periksa tanaman secara langsung.`
	}

	return formatDiagnosisSummary(
		session.DetectedLabel,
		session.DetectedConfidence,
		session.Severity,
		session.GrowthStage,
		[]byte(session.Symptoms),
		recommendationResult,
	)
}

func formatDiagnosisSummary(label string, confidence float64, severity string, growthStage string, symptomsRaw []byte, recommendationResult *DiagnosisRecommendationResult) string {

	symptoms := make(
		[]string,
		0,
	)

	if len(symptomsRaw) > 0 {
		symptoms = extractSymptomSummary(
			symptomsRaw,
		)
	}

	if len(symptoms) == 0 {
		symptoms = append(
			symptoms,
			"belum ada gejala tersimpan",
		)
	}

	return FormatRecommendationMessage(
		recommendationResult,
	)
}

func extractSymptomSummary(symptomsRaw []byte) []string {

	rawSymptoms := make(
		[]string,
		0,
	)

	if err := json.Unmarshal(
		symptomsRaw,
		&rawSymptoms,
	); err == nil &&
		len(rawSymptoms) > 0 {

		return deduplicateSymptomSummary(rawSymptoms)
	}

	var sessionData diagnose.SymptomSessionData

	if err := json.Unmarshal(
		symptomsRaw,
		&sessionData,
	); err != nil {

		return deduplicateSymptomSummary(rawSymptoms)
	}

	results := make(
		[]string,
		0,
		len(sessionData.Normalized),
	)

	for _, item := range sessionData.Normalized {
		switch {
		case item.MatchedSymptomName != "":
			results = append(
				results,
				item.MatchedSymptomName,
			)

		case item.NormalizedText != "":
			results = append(
				results,
				item.NormalizedText,
			)

		case item.InputText != "":
			results = append(
				results,
				item.InputText,
			)
		}
	}

	if len(results) == 0 {
		results = append(
			results,
			sessionData.UserInputs...,
		)
	}

	return deduplicateSymptomSummary(results)
}

func deduplicateSymptomSummary(symptoms []string) []string {

	seen := make(
		map[string]struct{},
		len(symptoms),
	)

	results := make(
		[]string,
		0,
		len(symptoms),
	)

	for _, symptom := range symptoms {
		symptom = strings.TrimSpace(symptom)
		if symptom == "" {
			continue
		}

		key := strings.ToLower(symptom)
		key = regexp.MustCompile(`\s+`).ReplaceAllString(key, " ")

		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}

		results = append(
			results,
			symptom,
		)
	}

	return results
}

func emptyFallback(
	value string,
	fallback string,
) string {

	if strings.TrimSpace(value) == "" {
		return fallback
	}

	return value
}

func logExpertResponse(operation string, startedAt time.Time, previousState dto.DiagnoseFlowState, response *dto.OrchestratorResponse, err error) {
	if err != nil {
		logger.Failure(
			"expert_system",
			operation,
			startedAt,
			err,
			slog.String("previous_state", string(previousState)),
		)
		return
	}

	if response == nil {
		logger.Response(
			"expert_system",
			operation,
			startedAt,
			slog.String("previous_state", string(previousState)),
			slog.Bool("empty_response", true),
		)
		return
	}

	logger.Response(
		"expert_system",
		operation,
		startedAt,
		slog.String("previous_state", string(previousState)),
		slog.String("session_id", response.SessionID.String()),
		slog.String("next_state", string(response.State)),
		slog.Int("message_length", len(response.Message)),
		slog.Int("action_count", len(response.Actions)),
	)
}

func FormatPestDisplayName(label string) string {

	label = strings.TrimSpace(label)

	if label == "" {
		return ""
	}

	label = strings.ReplaceAll(
		label,
		"_",
		" ",
	)

	words := strings.Fields(label)

	for i := range words {

		if len(words[i]) == 0 {
			continue
		}

		words[i] =
			strings.ToUpper(words[i][:1]) +
				words[i][1:]
	}

	return strings.Join(
		words,
		" ",
	)
}

func isNoPestDetectedLabel(label string) bool {
	normalized := strings.ToLower(
		strings.TrimSpace(label),
	)
	normalized = strings.ReplaceAll(
		normalized,
		" ",
		"_",
	)
	normalized = strings.ReplaceAll(
		normalized,
		"-",
		"_",
	)

	for strings.Contains(normalized, "__") {
		normalized = strings.ReplaceAll(
			normalized,
			"__",
			"_",
		)
	}

	normalized = strings.Trim(
		normalized,
		"_",
	)

	switch normalized {
	case "tidak_ada_hama",
		"tanpa_hama",
		"no_pest",
		"no_pests",
		"none":
		return true
	default:
		return false
	}
}

func buildUnsupportedDetectionResponse(session *domain.ExpertSession, reason string) *dto.OrchestratorResponse {
	_ = reason // alasan teknis dicatat di log, tidak ditampilkan kepada pengguna
	message := `Foto belum dapat dicocokkan dengan tiga hama yang tersedia: wereng batang cokelat, walang sangit, dan penggerek batang.

AgriPadi tidak menampilkan pestisida agar tidak terjadi salah penanganan.

Ambil foto dari jarak dekat, gunakan cahaya yang cukup, dan pastikan hama atau bagian tanaman yang rusak terlihat jelas.`

	return &dto.OrchestratorResponse{
		SessionID: session.ID,
		Mode:      dto.ConversationModeConsultation,
		State:     dto.DiagnoseFlowStateCompleted,
		Message:   strings.TrimSpace(message),
		Actions: []dto.ChatAction{
			{
				Type:  dto.ActionUploadImage,
				Label: "Periksa Foto Lain",
				Value: "upload_image",
			},
			{
				Type:  dto.ActionContinueConsult,
				Label: "Tanya AgriPadi",
				Value: "consultation",
			},
		},
		CreatedAt: time.Now(),
	}
}
