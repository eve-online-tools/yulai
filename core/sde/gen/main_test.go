package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestSnake(t *testing.T) {
	for in, want := range map[string]string{
		"mapSolarSystems":   "map_solar_systems",
		"typeID":            "type_id",
		"planetIDs":         "planet_ids",
		"position2D":        "position2d",
		"research_material": "research_material",
		"blueprintTypeID":   "blueprint_type_id",
	} {
		if got := snake(in); got != want {
			t.Errorf("snake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuild(t *testing.T) {
	recs := []string{
		`{"_key": 1, "name": {"en": "A", "de": "A"}, "position": {"x": 1.5}, "typeID": 3, "planetIDs": [4, 5],
		  "types": [{"_key": 7, "_value": [{"bonus": 1}]}], "rate": 1}`,
		`{"_key": 2, "rate": 0.5, "published": true}`,
	}
	root := &node{}
	for _, r := range recs {
		dec := json.NewDecoder(strings.NewReader(r))
		dec.UseNumber()
		var v map[string]any
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
		if err := root.add(v); err != nil {
			t.Fatal(err)
		}
	}
	tables, err := build("typeBonus", root)
	if err != nil {
		t.Fatal(err)
	}
	if err := unique(tables); err != nil {
		t.Fatal(err)
	}

	got := string(schemaSQL(tables))
	for _, want := range []string{
		`CREATE TABLE "type_bonus" (
    "key" TYPE_ID PRIMARY KEY,
    "name_de" TEXT,
    "name_en" TEXT,
    "position_x" REAL,
    "published" BOOLEAN,
    "rate" REAL,
    "type_id" TYPE_ID
);`,
		`CREATE TABLE "type_bonus_planet_ids" (
    "parent" TYPE_ID NOT NULL,
    "idx" INTEGER NOT NULL,
    "value" INTEGER
);`,
		`CREATE TABLE "type_bonus_types" (
    "id" INTEGER PRIMARY KEY,
    "parent" TYPE_ID NOT NULL,
    "key" INTEGER NOT NULL
);`,
		`CREATE TABLE "type_bonus_types_value" (
    "parent" INTEGER NOT NULL,
    "idx" INTEGER NOT NULL,
    "bonus" INTEGER
);`,
		`CREATE INDEX "type_bonus_types_value_parent" ON "type_bonus_types_value" ("parent");`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("schema lacks\n%s\n\ngot\n%s", want, got)
		}
	}
	if p := tables[3].path; !slices.Equal(p, []string{"_value"}) {
		t.Errorf("value table path = %v", p)
	}
}

func TestBuildRejectsMixedTypes(t *testing.T) {
	root := &node{}
	for _, v := range []any{map[string]any{"_key": json.Number("1"), "x": "a"}, map[string]any{"_key": json.Number("2"), "x": json.Number("1")}} {
		if err := root.add(v); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := build("f", root); err == nil {
		t.Fatal("no error for a field mixing strings and numbers")
	}
}
