package presence

import "context"

// Service is the Wails-facing side of the feature.
type Service struct {
	f *Feature
}

func NewService(f *Feature) *Service { return &Service{f: f} }

func (s *Service) ServiceName() string { return "PresenceService" }

// List returns the last known location of every character that has one. The
// frontend filters by scopes, so a character needing login keeps its last spot.
func (s *Service) List(ctx context.Context) ([]ListPresenceRow, error) {
	rows, err := s.f.q.ListPresence(ctx)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []ListPresenceRow{}
	}
	return rows, nil
}
