package diagnose

import (
	"context"

	"github.com/gustian305/backend/internal/service/expert_system/session"
	"github.com/gustian305/backend/internal/service/expert_system/workflow"
)

type FinalizeService struct {
	sessionLoader  *session.SessionLoader
	sessionUpdater *session.SessionUpdater
}

func NewFinalizeService(
	sessionLoader *session.SessionLoader,
	sessionUpdater *session.SessionUpdater,
) *FinalizeService {

	return &FinalizeService{
		sessionLoader:  sessionLoader,
		sessionUpdater: sessionUpdater,
	}
}

func (s *FinalizeService) Execute(ctx context.Context, conversationID string) error {

	session, err := s.sessionLoader.RequireActiveSession(ctx, conversationID)

	if err != nil {
		return err
	}

	// Penentuan hasil dan rekomendasi sudah diselesaikan oleh alur utama.
	// Tahap ini hanya menutup sesi setelah seluruh hasil berhasil disusun.

	return s.sessionUpdater.UpdateState(
		ctx,
		session,
		workflow.StateCompleted,
	)
}
