// Package shiptree builds the static data tables of the eve-ship-tree frontend
// package from the installed SDE, so the tree follows the latest build.
package shiptree

import (
	"context"

	"github.com/eve-online-tools/yulai/core/sde"
)

// Dogma attributes read from the ship types. Keep in sync with ShipTreeTypeAttributes.
const (
	attrTechLevel = 422
	attrRigSize   = 1547
)

// requiredSkillAttrs pairs the required skill attributes with their level attributes.
var requiredSkillAttrs = [][2]int64{
	{182, 277},
	{183, 278},
	{184, 279},
	{1285, 1286},
	{1289, 1287},
}

// Freighters have no rig size; the tree draws them a size above capitals.
var freighterGroups = map[int64]bool{
	37: true, // Freighter
	38: true, // Jump Freighters
}

const freighterSize = 4

// Data is the subset of the package's Data tables the tree reads, in the same
// shape as its JSONL files. Keys are SDE keys.
type Data struct {
	Types          map[int64]Type           `json:"types"`
	RequiredSkills map[int64]RequiredSkills `json:"requiredSkills"`
	Certificates   map[int64]Certificate    `json:"certificates"`
	Masteries      map[int64][]Keyed        `json:"masteries"`
	CloneGrades    map[int64]CloneGrade     `json:"cloneGrades"`
	ShipTreeGroups map[int64]ShipTreeGroup  `json:"shipTreeGroups"`
	ShipSizes      map[int64]ShipSize       `json:"shipSizes"`
	// For the faction summary and tooltips; the tree itself does not read these.
	ShipTreeFactions map[int64]ShipTreeFaction `json:"shipTreeFactions"`
	ShipTreeElements map[int64]ShipTreeElement `json:"shipTreeElements"`
	Skills           map[int64]Skill           `json:"skills"`
	TypeBonus        map[int64]TypeBonus       `json:"typeBonus"`
	TypeElements     map[int64]TypeElements    `json:"typeElements"`
}

type Type struct {
	Name            *Text  `json:"name,omitempty"`
	ShipTreeGroupID *int64 `json:"shipTreeGroupID,omitempty"`
	FactionID       *int64 `json:"factionID,omitempty"`
	MetaGroupID     *int64 `json:"metaGroupID,omitempty"`
	TechLevel       int64  `json:"techLevel"`
}

type RequiredSkills struct {
	RequiredSkills map[int64]int64 `json:"requiredSkills"`
}

type Certificate struct {
	SkillTypes []CertificateSkill `json:"skillTypes"`
}

type CertificateSkill struct {
	Key      int64 `json:"_key"`
	Basic    int64 `json:"basic"`
	Standard int64 `json:"standard"`
	Improved int64 `json:"improved"`
	Advanced int64 `json:"advanced"`
	Elite    int64 `json:"elite"`
}

// Keyed is a mastery level (0-4) and the certificates it needs.
type Keyed struct {
	Key   int64   `json:"_key"`
	Value []int64 `json:"_value"`
}

type CloneGrade struct {
	Skills []CloneGradeSkill `json:"skills"`
}

type CloneGradeSkill struct {
	Level  int64 `json:"level"`
	TypeID int64 `json:"typeID"`
}

type ShipTreeGroup struct {
	Name         *Text         `json:"name,omitempty"`
	Description  *Text         `json:"description,omitempty"`
	Elements     []Element     `json:"elements"`
	PreReqSkills []GroupSkills `json:"preReqSkills"`
}

type Element struct {
	Key   int64 `json:"_key"`
	Value int64 `json:"_value"`
}

// GroupSkills are the skills a group needs for one faction.
type GroupSkills struct {
	Key    int64        `json:"_key"`
	Skills []GroupSkill `json:"skills"`
}

type GroupSkill struct {
	Key     int64 `json:"_key"`
	Display bool  `json:"display"`
	Level   int64 `json:"level"`
}

// ShipTreeFaction is a faction's summary: what it excels at and how it fights.
type ShipTreeFaction struct {
	Description *Text     `json:"description,omitempty"`
	Elements    []Element `json:"elements"`
}

// ShipTreeElement is a trait a faction or group excels at, like Armor or Drones.
type ShipTreeElement struct {
	Name        *Text `json:"name,omitempty"`
	Description *Text `json:"description,omitempty"`
}

// Skill is a skill the ships require, named in tooltips.
type Skill struct {
	Name *Text `json:"name,omitempty"`
}

// TypeBonus holds a ship's bonuses: per level of the skill in Types[].Key, role and misc.
type TypeBonus struct {
	Types       []SkillBonuses `json:"types,omitempty"`
	RoleBonuses []Bonus        `json:"roleBonuses,omitempty"`
	MiscBonuses []Bonus        `json:"miscBonuses,omitempty"`
}

type SkillBonuses struct {
	Key   int64   `json:"_key"`
	Value []Bonus `json:"_value"`
}

type Bonus struct {
	Bonus      *float64 `json:"bonus,omitempty"`
	BonusText  *Text    `json:"bonusText,omitempty"`
	Importance *int64   `json:"importance,omitempty"`
	UnitID     *int64   `json:"unitID,omitempty"`
}

// TypeElements are the traits shown on a ship's tooltip, size first.
type TypeElements struct {
	Elements []Element `json:"elements"`
}

// Text is a localized SDE string. Only English is built.
type Text struct {
	En string `json:"en"`
}

type ShipSize struct {
	TypeIDs []int64 `json:"typeIDs"`
}

// build reads the tables from the installed SDE. The result is shared between
// callers, so it is not modified after.
func build(ctx context.Context, q *sde.Queries) (*Data, error) {
	d := &Data{
		Types:          map[int64]Type{},
		RequiredSkills: map[int64]RequiredSkills{},
		Certificates:   map[int64]Certificate{},
		Masteries:      map[int64][]Keyed{},
		CloneGrades:    map[int64]CloneGrade{},
		ShipTreeGroups: map[int64]ShipTreeGroup{},
		ShipSizes:      map[int64]ShipSize{},

		ShipTreeFactions: map[int64]ShipTreeFaction{},
		ShipTreeElements: map[int64]ShipTreeElement{},
		Skills:           map[int64]Skill{},
		TypeBonus:        map[int64]TypeBonus{},
		TypeElements:     map[int64]TypeElements{},
	}

	types, err := q.ShipTreeTypes(ctx)
	if err != nil {
		return nil, err
	}
	attrRows, err := q.ShipTreeTypeAttributes(ctx)
	if err != nil {
		return nil, err
	}
	attrs := map[int64]map[int64]float64{}
	for _, a := range attrRows {
		if a.AttributeID == nil || a.Value == nil {
			continue
		}
		id := int64(a.Parent)
		if attrs[id] == nil {
			attrs[id] = map[int64]float64{}
		}
		attrs[id][*a.AttributeID] = *a.Value
	}
	for _, t := range types {
		id := int64(t.Key)
		a := attrs[id]
		row := Type{
			Name:            text(t.NameEn),
			ShipTreeGroupID: t.ShipTreeGroupID,
			MetaGroupID:     t.MetaGroupID,
			TechLevel:       techLevel(t.TechLevel, a),
		}
		if t.FactionID != nil {
			f := int64(*t.FactionID)
			row.FactionID = &f
		}
		d.Types[id] = row
		d.RequiredSkills[id] = RequiredSkills{RequiredSkills: requiredSkills(a)}
		size := rigSize(*t.ShipTreeGroupID, a)
		d.ShipSizes[size] = ShipSize{TypeIDs: append(d.ShipSizes[size].TypeIDs, id)}
	}

	masteries, err := q.ShipTreeMasteries(ctx)
	if err != nil {
		return nil, err
	}
	for _, m := range masteries {
		if m.CertificateID == nil {
			continue
		}
		levels := d.Masteries[m.TypeID]
		if n := len(levels); n == 0 || levels[n-1].Key != m.Level {
			levels = append(levels, Keyed{Key: m.Level})
		}
		last := &levels[len(levels)-1]
		last.Value = append(last.Value, *m.CertificateID)
		d.Masteries[m.TypeID] = levels
	}

	certs, err := q.CertificateSkills(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range certs {
		cert := d.Certificates[c.Parent]
		cert.SkillTypes = append(cert.SkillTypes, CertificateSkill{
			Key:      c.Key,
			Basic:    deref(c.Basic),
			Standard: deref(c.Standard),
			Improved: deref(c.Improved),
			Advanced: deref(c.Advanced),
			Elite:    deref(c.Elite),
		})
		d.Certificates[c.Parent] = cert
	}

	grades, err := q.CloneGradeSkills(ctx)
	if err != nil {
		return nil, err
	}
	for _, g := range grades {
		if g.TypeID == nil {
			continue
		}
		grade := d.CloneGrades[g.Parent]
		grade.Skills = append(grade.Skills, CloneGradeSkill{Level: deref(g.Level), TypeID: int64(*g.TypeID)})
		d.CloneGrades[g.Parent] = grade
	}

	groups, err := q.ShipTreeGroups(ctx)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		d.ShipTreeGroups[g.Key] = ShipTreeGroup{
			Name:         text(g.NameEn),
			Description:  text(g.DescriptionEn),
			Elements:     []Element{},
			PreReqSkills: []GroupSkills{},
		}
	}

	elements, err := q.ShipTreeGroupElements(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range elements {
		if e.Value == nil {
			continue
		}
		group := d.ShipTreeGroups[e.Parent]
		group.Elements = append(group.Elements, Element{Key: e.Key, Value: *e.Value})
		d.ShipTreeGroups[e.Parent] = group
	}

	skills, err := q.ShipTreeGroupSkills(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range skills {
		group := d.ShipTreeGroups[s.GroupID]
		if n := len(group.PreReqSkills); n == 0 || group.PreReqSkills[n-1].Key != s.FactionID {
			group.PreReqSkills = append(group.PreReqSkills, GroupSkills{Key: s.FactionID})
		}
		last := &group.PreReqSkills[len(group.PreReqSkills)-1]
		last.Skills = append(last.Skills, GroupSkill{
			Key:     s.SkillID,
			Display: s.Display != nil && *s.Display,
			Level:   deref(s.Level),
		})
		d.ShipTreeGroups[s.GroupID] = group
	}

	factions, err := q.ShipTreeFactions(ctx)
	if err != nil {
		return nil, err
	}
	for _, f := range factions {
		d.ShipTreeFactions[f.Key] = ShipTreeFaction{Description: text(f.DescriptionEn), Elements: []Element{}}
	}
	factionElements, err := q.ShipTreeFactionElements(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range factionElements {
		f, ok := d.ShipTreeFactions[e.Parent]
		if !ok || e.Value == nil {
			continue
		}
		f.Elements = append(f.Elements, Element{Key: e.Key, Value: *e.Value})
		d.ShipTreeFactions[e.Parent] = f
	}

	shipTreeElements, err := q.ShipTreeElements(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range shipTreeElements {
		d.ShipTreeElements[e.Key] = ShipTreeElement{Name: text(e.NameEn), Description: text(e.DescriptionEn)}
	}

	if err := buildTooltips(ctx, q, d); err != nil {
		return nil, err
	}
	return d, nil
}

// buildTooltips adds the ship bonuses and elements, and the names of the
// skills the tooltips list.
func buildTooltips(ctx context.Context, q *sde.Queries, d *Data) error {
	elements, err := q.ShipTreeTypeElements(ctx)
	if err != nil {
		return err
	}
	for _, e := range elements {
		if e.Value == nil {
			continue
		}
		t := d.TypeElements[e.Parent]
		t.Elements = append(t.Elements, Element{Key: e.Key, Value: *e.Value})
		d.TypeElements[e.Parent] = t
	}

	skillBonuses, err := q.ShipTreeSkillBonuses(ctx)
	if err != nil {
		return err
	}
	for _, b := range skillBonuses {
		id := int64(b.TypeID)
		t := d.TypeBonus[id]
		if n := len(t.Types); n == 0 || t.Types[n-1].Key != b.SkillID {
			t.Types = append(t.Types, SkillBonuses{Key: b.SkillID})
		}
		last := &t.Types[len(t.Types)-1]
		last.Value = append(last.Value, Bonus{Bonus: b.Bonus, BonusText: text(b.BonusTextEn), Importance: b.Importance, UnitID: b.UnitID})
		d.TypeBonus[id] = t
	}

	roleBonuses, err := q.ShipTreeRoleBonuses(ctx)
	if err != nil {
		return err
	}
	for _, b := range roleBonuses {
		id := int64(b.TypeID)
		t := d.TypeBonus[id]
		t.RoleBonuses = append(t.RoleBonuses, Bonus{Bonus: b.Bonus, BonusText: text(b.BonusTextEn), Importance: b.Importance, UnitID: b.UnitID})
		d.TypeBonus[id] = t
	}

	miscBonuses, err := q.ShipTreeMiscBonuses(ctx)
	if err != nil {
		return err
	}
	for _, b := range miscBonuses {
		id := int64(b.TypeID)
		t := d.TypeBonus[id]
		t.MiscBonuses = append(t.MiscBonuses, Bonus{Bonus: b.Bonus, BonusText: text(b.BonusTextEn), Importance: b.Importance, UnitID: b.UnitID})
		d.TypeBonus[id] = t
	}

	used := map[int64]bool{}
	for _, r := range d.RequiredSkills {
		for id := range r.RequiredSkills {
			used[id] = true
		}
	}
	for _, g := range d.ShipTreeGroups {
		for _, f := range g.PreReqSkills {
			for _, s := range f.Skills {
				used[s.Key] = true
			}
		}
	}
	for _, t := range d.TypeBonus {
		for _, s := range t.Types {
			used[s.Key] = true
		}
	}
	names, err := q.SkillNames(ctx)
	if err != nil {
		return err
	}
	for _, n := range names {
		if used[int64(n.Key)] {
			d.Skills[int64(n.Key)] = Skill{Name: text(n.NameEn)}
		}
	}
	return nil
}

// techLevel prefers the type's own field, then the dogma attribute, then 1.
func techLevel(field *int64, attrs map[int64]float64) int64 {
	if field != nil {
		return *field
	}
	if v, ok := attrs[attrTechLevel]; ok {
		return int64(v)
	}
	return 1
}

func requiredSkills(attrs map[int64]float64) map[int64]int64 {
	out := map[int64]int64{}
	for _, p := range requiredSkillAttrs {
		skill := int64(attrs[p[0]])
		if skill == 0 {
			continue
		}
		out[skill] = int64(attrs[p[1]])
	}
	return out
}

func rigSize(group int64, attrs map[int64]float64) int64 {
	if freighterGroups[group] {
		return freighterSize
	}
	return int64(attrs[attrRigSize])
}

func text(v *string) *Text {
	if v == nil {
		return nil
	}
	return &Text{En: *v}
}

func deref(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
