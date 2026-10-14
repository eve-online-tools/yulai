// Package starmap serves known space from the SDE to the map page.
package starmap

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/eve-online-tools/yulai/core/sde"
	"github.com/eve-online-tools/yulai/feature"
)

func Feature(d feature.Deps) feature.Feature {
	return feature.Feature{
		ID: "map",
		Services: []application.Service{
			application.NewService(newService(d.SDE)),
		},
	}
}

// Service is the Wails-facing side of the map data.
type Service struct {
	data *sde.Derived[*Data]
}

func newService(store *sde.Store) *Service {
	return &Service{data: sde.NewDerived(store, build)}
}

func (s *Service) ServiceName() string { return "MapService" }

// Data returns known space, built once per installed SDE. It fails with
// sde.ErrNotInstalled before the first install; the frontend refetches on sde:changed.
func (s *Service) Data(ctx context.Context) (*Data, error) {
	return s.data.Get(ctx)
}

// Data is the input of @eve-online-tools/eve-map.
type Data struct {
	Systems []System `json:"systems"`
	// Keyed by constellation ID.
	Constellations map[int64]Constellation `json:"constellations"`
	// Region ID to name.
	Regions map[int64]string `json:"regions"`
}

type System struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	ConstellationID int64   `json:"constellationId"`
	Security        float64 `json:"security"`
	// SDE universe coordinates, meters.
	Position   Vec3  `json:"position"`
	Position2D *Vec2 `json:"position2d,omitempty"`
	// IDs of systems connected by a stargate.
	Gates []int64 `json:"gates"`
}

type Constellation struct {
	Name     string `json:"name"`
	RegionID int64  `json:"regionId"`
}

type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// build reads known space from the SDE. Systems without a position or
// constellation are skipped, since the map rejects them.
func build(ctx context.Context, q *sde.Queries) (*Data, error) {
	regions, err := q.ListMapRegions(ctx)
	if err != nil {
		return nil, err
	}
	constellations, err := q.ListMapConstellations(ctx)
	if err != nil {
		return nil, err
	}
	systems, err := q.ListMapSystems(ctx)
	if err != nil {
		return nil, err
	}
	gates, err := q.ListMapGates(ctx)
	if err != nil {
		return nil, err
	}

	out := &Data{
		Systems:        make([]System, 0, len(systems)),
		Constellations: make(map[int64]Constellation, len(constellations)),
		Regions:        make(map[int64]string, len(regions)),
	}
	for _, r := range regions {
		out.Regions[r.Key] = deref(r.NameEn)
	}
	for _, c := range constellations {
		out.Constellations[c.Key] = Constellation{Name: deref(c.NameEn), RegionID: deref(c.RegionID)}
	}

	links := map[int64][]int64{}
	for _, g := range gates {
		if g.SolarSystemID == nil || g.DestinationSolarSystemID == nil {
			continue
		}
		from := int64(*g.SolarSystemID)
		links[from] = append(links[from], int64(*g.DestinationSolarSystemID))
	}

	for _, s := range systems {
		if s.PositionX == nil || s.PositionY == nil || s.PositionZ == nil || s.ConstellationID == nil {
			continue
		}
		if _, ok := out.Constellations[*s.ConstellationID]; !ok {
			continue
		}
		id := int64(s.Key)
		sys := System{
			ID:              id,
			Name:            deref(s.NameEn),
			ConstellationID: *s.ConstellationID,
			Security:        deref(s.SecurityStatus),
			Position:        Vec3{X: *s.PositionX, Y: *s.PositionY, Z: *s.PositionZ},
			Gates:           links[id],
		}
		if sys.Gates == nil {
			sys.Gates = []int64{}
		}
		if s.Position2dX != nil && s.Position2dY != nil {
			sys.Position2D = &Vec2{X: *s.Position2dX, Y: *s.Position2dY}
		}
		out.Systems = append(out.Systems, sys)
	}
	return out, nil
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
