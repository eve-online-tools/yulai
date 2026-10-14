package shiptree

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/eve-online-tools/yulai/core/sde"
	"github.com/eve-online-tools/yulai/feature"
)

func Feature(d feature.Deps) feature.Feature {
	return feature.Feature{
		ID: "shiptree",
		Services: []application.Service{
			application.NewService(newService(d.SDE)),
		},
	}
}

// Service is the Wails-facing side of the ship tree data.
type Service struct {
	data *sde.Derived[*Data]
}

func newService(store *sde.Store) *Service {
	return &Service{data: sde.NewDerived(store, build)}
}

func (s *Service) ServiceName() string { return "ShipTreeService" }

// Data returns the tree's static data, built once per installed SDE. It fails
// before the first install; the frontend refetches when the sde topic changes.
func (s *Service) Data(ctx context.Context) (*Data, error) {
	return s.data.Get(ctx)
}
