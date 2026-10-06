package shiptree

import (
	"archive/zip"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/eve-online-tools/yulai/core/sde"
)

// Records in CCP's export format, trimmed to the fields the tree reads.
var fixture = map[string]string{
	"_sde.jsonl": `{"_key": "sde", "buildNumber": 1, "releaseDate": "2026-10-02T11:08:57Z"}`,
	"groups.jsonl": `{"_key": 25, "categoryID": 6, "name": {"en": "Frigate"}}
{"_key": 513, "categoryID": 6, "name": {"en": "Freighter"}}
{"_key": 76, "categoryID": 7, "name": {"en": "Capacitor Booster"}}
{"_key": 257, "categoryID": 16, "name": {"en": "Spaceship Command"}}`,
	// A frigate, a freighter, a module that also carries a ship tree group, and
	// skills: required, group prerequisite and unrelated.
	"types.jsonl": `{"_key": 582, "groupID": 25, "factionID": 500001, "metaGroupID": 1, "shipTreeGroupID": 8, "name": {"en": "Bantam"}}
{"_key": 20185, "groupID": 513, "factionID": 500001, "shipTreeGroupID": 37, "techLevel": 1}
{"_key": 14180, "groupID": 76, "shipTreeGroupID": 8}
{"_key": 3330, "groupID": 257, "name": {"en": "Caldari Frigate"}}
{"_key": 3331, "groupID": 257, "name": {"en": "Amarr Frigate"}}
{"_key": 3327, "groupID": 257, "name": {"en": "Spaceship Command"}}`,
	"typeDogma.jsonl": `{"_key": 582, "dogmaAttributes": [{"attributeID": 182, "value": 3330}, {"attributeID": 277, "value": 1}, {"attributeID": 183, "value": 3300}, {"attributeID": 278, "value": 2}, {"attributeID": 1547, "value": 1}, {"attributeID": 422, "value": 2}, {"attributeID": 9, "value": 400}]}
{"_key": 20185, "dogmaAttributes": [{"attributeID": 182, "value": 20342}, {"attributeID": 277, "value": 1}]}
{"_key": 14180, "dogmaAttributes": [{"attributeID": 182, "value": 3418}, {"attributeID": 277, "value": 1}]}`,
	"masteries.jsonl": `{"_key": 582, "_value": [{"_key": 0, "_value": [96, 139]}, {"_key": 1, "_value": [96]}]}
{"_key": 14180, "_value": [{"_key": 0, "_value": [1]}]}`,
	"certificates.jsonl": `{"_key": 96, "name": {"en": "Core Fitting"}, "skillTypes": [{"_key": 3413, "basic": 1, "standard": 2, "improved": 3, "advanced": 4, "elite": 5}]}`,
	"cloneGrades.jsonl":  `{"_key": 1, "name": "Alpha Caldari", "skills": [{"level": 5, "typeID": 3300}, {"level": 4, "typeID": 3330}]}`,
	"shipTreeGroups.jsonl": `{"_key": 8, "name": {"en": "Frigate"}, "description": {"en": "Small and fast."}, "elements": [{"_key": 1, "_value": 30}, {"_key": 2, "_value": 22}], "preReqSkills": [{"_key": 500001, "skills": [{"_key": 3330, "display": true, "level": 1}, {"_key": 3327, "display": false, "level": 1}]}, {"_key": 500004, "skills": [{"_key": 3328, "display": true, "level": 1}]}]}
{"_key": 37, "name": {"en": "Freighter"}}`,
	"shipTreeFactions.jsonl": `{"_key": 500002, "description": {"en": "Prefer Projectile Turrets."}, "icon": "res:/x.png", "elements": [{"_key": 2, "_value": 7}, {"_key": 1, "_value": 10}]}`,
	"shipTreeElements.jsonl": `{"_key": 7, "name": {"en": "Armor"}, "description": {"en": "Low slot defense."}, "icon": "armor"}
{"_key": 10, "name": {"en": "Projectile Turrets"}, "icon": "gunnery"}`,
	"typeElements.jsonl": `{"_key": 582, "elements": [{"_key": 1, "_value": 30}, {"_key": 2, "_value": 25}]}
{"_key": 14180, "elements": [{"_key": 1, "_value": 30}]}`,
	"typeBonus.jsonl": `{"_key": 582, "roleBonuses": [{"bonus": 300, "bonusText": {"en": "bonus to falloff"}, "importance": 1, "unitID": 105}], "types": [{"_key": 3330, "_value": [{"bonus": 7.5, "bonusText": {"en": "bonus to amount"}, "importance": 1, "unitID": 105}, {"bonus": 10, "bonusText": {"en": "reduction in cost"}, "importance": 2, "unitID": 105}]}]}
{"_key": 20185, "miscBonuses": [{"bonusText": {"en": "Can fit a Jump Drive"}, "importance": 1, "isPositive": true}]}
{"_key": 14180, "roleBonuses": [{"bonus": 1, "bonusText": {"en": "module"}, "importance": 1}]}`,
}

const want = `{
  "types": {
    "582": {"name": {"en": "Bantam"}, "shipTreeGroupID": 8, "factionID": 500001, "metaGroupID": 1, "techLevel": 2},
    "20185": {"shipTreeGroupID": 37, "factionID": 500001, "techLevel": 1}
  },
  "requiredSkills": {
    "582": {"requiredSkills": {"3300": 2, "3330": 1}},
    "20185": {"requiredSkills": {"20342": 1}}
  },
  "certificates": {
    "96": {"skillTypes": [{"_key": 3413, "basic": 1, "standard": 2, "improved": 3, "advanced": 4, "elite": 5}]}
  },
  "masteries": {
    "582": [{"_key": 0, "_value": [96, 139]}, {"_key": 1, "_value": [96]}]
  },
  "cloneGrades": {
    "1": {"skills": [{"level": 5, "typeID": 3300}, {"level": 4, "typeID": 3330}]}
  },
  "shipTreeGroups": {
    "8": {
      "name": {"en": "Frigate"},
      "description": {"en": "Small and fast."},
      "elements": [{"_key": 1, "_value": 30}, {"_key": 2, "_value": 22}],
      "preReqSkills": [
        {"_key": 500001, "skills": [{"_key": 3330, "display": true, "level": 1}, {"_key": 3327, "display": false, "level": 1}]},
        {"_key": 500004, "skills": [{"_key": 3328, "display": true, "level": 1}]}
      ]
    },
    "37": {"name": {"en": "Freighter"}, "elements": [], "preReqSkills": []}
  },
  "shipSizes": {
    "1": {"typeIDs": [582]},
    "4": {"typeIDs": [20185]}
  },
  "shipTreeFactions": {
    "500002": {"description": {"en": "Prefer Projectile Turrets."}, "elements": [{"_key": 1, "_value": 10}, {"_key": 2, "_value": 7}]}
  },
  "shipTreeElements": {
    "7": {"name": {"en": "Armor"}, "description": {"en": "Low slot defense."}},
    "10": {"name": {"en": "Projectile Turrets"}}
  },
  "skills": {
    "3327": {"name": {"en": "Spaceship Command"}},
    "3330": {"name": {"en": "Caldari Frigate"}}
  },
  "typeBonus": {
    "582": {
      "types": [{"_key": 3330, "_value": [
        {"bonus": 7.5, "bonusText": {"en": "bonus to amount"}, "importance": 1, "unitID": 105},
        {"bonus": 10, "bonusText": {"en": "reduction in cost"}, "importance": 2, "unitID": 105}
      ]}],
      "roleBonuses": [{"bonus": 300, "bonusText": {"en": "bonus to falloff"}, "importance": 1, "unitID": 105}]
    },
    "20185": {"miscBonuses": [{"bonusText": {"en": "Can fit a Jump Drive"}, "importance": 1}]}
  },
  "typeElements": {
    "582": {"elements": [{"_key": 1, "_value": 30}, {"_key": 2, "_value": 25}]}
  }
}`

func TestBuild(t *testing.T) {
	s, err := sde.Open(t.Context(), t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Install(t.Context(), writeZip(t, fixture)); err != nil {
		t.Fatal(err)
	}

	d, err := NewService(s).Data(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if !jsonEqual(t, got, []byte(want)) {
		t.Fatalf("got %s", got)
	}
}

func TestBuildNotInstalled(t *testing.T) {
	s, err := sde.Open(t.Context(), t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err := NewService(s).Data(t.Context()); err != sde.ErrNotInstalled {
		t.Fatalf("err = %v", err)
	}
}

func writeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "export.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, body+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
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
