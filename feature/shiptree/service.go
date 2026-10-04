package shiptree

import (
	"context"

	"github.com/eve-online-tools/yulai/core/sde"
)

// Service is the Wails-facing side of the ship tree data.
type Service struct {
	store *sde.Store
}

func NewService(store *sde.Store) *Service { return &Service{store: store} }

func (s *Service) ServiceName() string { return "ShipTreeService" }

// Data returns the tree's static data from the installed SDE. It fails before
// the first install; the frontend refetches on sde:changed.
func (s *Service) Data(ctx context.Context) (*Data, error) {
	return Build(ctx, s.store)
}
