package skills

import (
	"encoding/json"
	"testing"

	"github.com/eve-online-tools/yulai/core/sde"
	"github.com/eve-online-tools/yulai/core/sde/sdetest"
)

// Records in CCP's export format, trimmed to the fields the catalog reads.
var catalogFixture = map[string]string{
	"groups.jsonl": `{"_key": 255, "categoryID": 16, "published": true, "name": {"en": "Gunnery"}}
{"_key": 257, "categoryID": 16, "published": true, "name": {"en": "Spaceship Command"}}
{"_key": 505, "categoryID": 16, "published": true, "name": {"en": "Fake Skills"}}
{"_key": 25, "categoryID": 6, "published": true, "name": {"en": "Frigate"}}`,
	// Published skills, an unpublished skill and a ship.
	"types.jsonl": `{"_key": 3300, "groupID": 255, "published": true, "name": {"en": "Gunnery"}, "description": {"en": "Turret operation."}}
{"_key": 3327, "groupID": 257, "published": true, "name": {"en": "Spaceship Command"}}
{"_key": 3330, "groupID": 257, "published": true, "name": {"en": "Caldari Frigate"}}
{"_key": 9955, "groupID": 505, "published": false, "name": {"en": "Polaris"}}
{"_key": 582, "groupID": 25, "published": true, "name": {"en": "Bantam"}}`,
	"typeDogma.jsonl": `{"_key": 3300, "dogmaAttributes": [{"attributeID": 180, "value": 167}, {"attributeID": 181, "value": 168}, {"attributeID": 275, "value": 1}]}
{"_key": 3330, "dogmaAttributes": [{"attributeID": 180, "value": 167}, {"attributeID": 181, "value": 168}, {"attributeID": 275, "value": 2}, {"attributeID": 182, "value": 3327}, {"attributeID": 277, "value": 1}]}`,
}

const wantCatalog = `{"groups": [
  {"id": 255, "name": "Gunnery", "skills": [
    {"id": 3300, "name": "Gunnery", "description": "Turret operation.", "rank": 1, "primary": "perception", "secondary": "willpower", "requires": []}
  ]},
  {"id": 257, "name": "Spaceship Command", "skills": [
    {"id": 3330, "name": "Caldari Frigate", "description": "", "rank": 2, "primary": "perception", "secondary": "willpower", "requires": [{"skillId": 3327, "level": 1}]},
    {"id": 3327, "name": "Spaceship Command", "description": "", "rank": 0, "primary": "", "secondary": "", "requires": []}
  ]}
]}`

func TestCatalog(t *testing.T) {
	c, err := newService(nil, sdetest.Install(t, catalogFixture)).Catalog(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !jsonEqual(t, got, []byte(wantCatalog)) {
		t.Fatalf("got %s", got)
	}
}

func TestCatalogNotInstalled(t *testing.T) {
	if _, err := newService(nil, sdetest.Open(t)).Catalog(t.Context()); err != sde.ErrNotInstalled {
		t.Fatalf("err = %v", err)
	}
}

func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	ja, _ := json.Marshal(x)
	jb, _ := json.Marshal(y)
	return string(ja) == string(jb)
}
