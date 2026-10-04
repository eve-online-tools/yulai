package sde

import (
	"context"
	"time"

	"github.com/eve-online-tools/yulai/core/task"
)

// Status is what the UI shows about static data.
type Status struct {
	// Build and ReleaseDate are zero before the first install.
	Build       int64     `json:"build"`
	ReleaseDate time.Time `json:"releaseDate"`
	// Latest is the build the last check saw, 0 before it.
	Latest    int64     `json:"latest"`
	LastCheck time.Time `json:"lastCheck"`
	// LastError is the error of the last failed check or update.
	LastError string `json:"lastError"`
}

// Service is the Wails-facing side of the updater.
type Service struct {
	u       *Updater
	list    func() []task.Status
	mapData *Derived[*MapData]
}

// list returns the scheduler's task statuses, e.g. task.Default.List.
func NewService(u *Updater, list func() []task.Status) *Service {
	return &Service{u: u, list: list, mapData: NewDerived(u.store, mapData)}
}

func (s *Service) ServiceName() string { return "SDEService" }

func (s *Service) Status() Status {
	var out Status
	if m := s.u.store.Meta(); m != nil {
		out.Build, out.ReleaseDate = m.Build, m.ReleaseDate
	}
	if b := s.u.Latest(); b != nil {
		out.Latest = b.Number
	}
	for _, st := range s.list() {
		switch st.Task {
		case Check.Name():
			out.LastCheck = st.LastRun
			if st.LastError != "" {
				out.LastError = st.LastError
			}
		case Update.Name():
			if st.LastError != "" {
				out.LastError = st.LastError
			}
		}
	}
	return out
}

// CheckNow runs a check, which queues an update when one is due.
func (s *Service) CheckNow(ctx context.Context) error {
	return Check.Queue(ctx, struct{}{})
}

// Map returns known space for the map. ErrNotInstalled before the first install.
func (s *Service) Map(ctx context.Context) (*MapData, error) {
	return s.mapData.Get(ctx)
}
