package sde

import "context"

// MapData is the input of @eve-online-tools/eve-map.
type MapData struct {
	Systems []MapSystem `json:"systems"`
	// Keyed by constellation ID.
	Constellations map[int64]MapConstellation `json:"constellations"`
	// Region ID to name.
	Regions map[int64]string `json:"regions"`
}

type MapSystem struct {
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

type MapConstellation struct {
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

// mapData reads known space from the SDE. Systems without a position or
// constellation are skipped, since the map rejects them.
func mapData(ctx context.Context, q *Queries) (*MapData, error) {
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

	out := &MapData{
		Systems:        make([]MapSystem, 0, len(systems)),
		Constellations: make(map[int64]MapConstellation, len(constellations)),
		Regions:        make(map[int64]string, len(regions)),
	}
	for _, r := range regions {
		out.Regions[r.Key] = deref(r.NameEn)
	}
	for _, c := range constellations {
		out.Constellations[c.Key] = MapConstellation{Name: deref(c.NameEn), RegionID: deref(c.RegionID)}
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
		sys := MapSystem{
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
