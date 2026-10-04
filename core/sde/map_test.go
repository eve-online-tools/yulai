package sde

import (
	"testing"
)

func TestMap(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(t.Context(), dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	files := fixture(1)
	files["mapRegions.jsonl"] = `{"_key": 10000002, "name": {"en": "The Forge"}}` + "\n"
	files["mapConstellations.jsonl"] = `{"_key": 20000020, "name": {"en": "Kimotoro"}, "regionID": 10000002}` + "\n"
	files["mapSolarSystems.jsonl"] = `{"_key": 30000142, "name": {"en": "Jita"}, "constellationID": 20000020, "position": {"x": 1, "y": 2, "z": 3}, "position2D": {"x": 4, "y": 5}, "securityStatus": 0.95}` + "\n" +
		`{"_key": 30000144, "name": {"en": "Perimeter"}, "constellationID": 20000020, "position": {"x": 6, "y": 7, "z": 8}, "securityStatus": 0.94}` + "\n" +
		`{"_key": 30000145, "name": {"en": "Nowhere"}, "constellationID": 20000999, "position": {"x": 0, "y": 0, "z": 0}}` + "\n" +
		`{"_key": 31000005, "name": {"en": "Thera"}, "constellationID": 20000020, "position": {"x": 0, "y": 0, "z": 0}}` + "\n"
	files["mapStargates.jsonl"] = `{"_key": 50001248, "solarSystemID": 30000142, "destination": {"solarSystemID": 30000144, "stargateID": 50001249}}` + "\n" +
		`{"_key": 50001249, "solarSystemID": 30000144, "destination": {"solarSystemID": 30000142, "stargateID": 50001248}}` + "\n"
	if err := s.Install(t.Context(), writeZip(t, dir, files)); err != nil {
		t.Fatal(err)
	}

	svc := NewService(NewUpdater(s, nil, nil), nil)
	got, err := svc.Map(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Systems) != 2 {
		t.Fatalf("systems = %+v, want Jita and Perimeter", got.Systems)
	}
	jita, perimeter := got.Systems[0], got.Systems[1]
	if jita.Name != "Jita" || jita.Security != 0.95 || jita.Position != (Vec3{1, 2, 3}) || *jita.Position2D != (Vec2{4, 5}) {
		t.Errorf("jita = %+v", jita)
	}
	if len(jita.Gates) != 1 || jita.Gates[0] != 30000144 {
		t.Errorf("jita gates = %v", jita.Gates)
	}
	if perimeter.Position2D != nil {
		t.Errorf("perimeter position2d = %v, want nil", perimeter.Position2D)
	}
	if got.Constellations[20000020] != (MapConstellation{Name: "Kimotoro", RegionID: 10000002}) || got.Regions[10000002] != "The Forge" {
		t.Errorf("constellations = %v, regions = %v", got.Constellations, got.Regions)
	}

	again, err := svc.Map(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if again != got {
		t.Error("second call rebuilt the map for the same build")
	}
}
