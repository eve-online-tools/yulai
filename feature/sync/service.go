package sync

import "context"

// Service is the Wails-facing side of the scheduler.
type Service struct {
	s *Scheduler
}

func NewService(s *Scheduler) *Service { return &Service{s: s} }

func (s *Service) ServiceName() string { return "SyncService" }

func (s *Service) List(ctx context.Context) ([]SyncJob, error) {
	return []SyncJob{}, nil
}

func (s *Service) RunNow(ctx context.Context, characterID int64, job string) error {
	return s.s.RunNow(ctx, characterID, job)
}
