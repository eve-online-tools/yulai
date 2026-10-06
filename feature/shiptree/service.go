package shiptree

import (
	"context"

	"github.com/eve-online-tools/yulai/core/sde"
)

// Service is the Wails-facing side of the ship tree data.
type Service struct {
	data *sde.Derived[*Data]
}

func NewService(store *sde.Store) *Service {
	return &Service{data: sde.NewDerived(store, build)}
}

func (s *Service) ServiceName() string { return "ShipTreeService" }

// Data returns the tree's static data, built once per installed SDE. It fails
// before the first install; the frontend refetches on sde:changed.
func (s *Service) Data(ctx context.Context) (*Data, error) {
	return s.data.Get(ctx)
}
