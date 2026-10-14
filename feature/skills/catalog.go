package skills

import (
	"context"

	"github.com/eve-online-tools/yulai/core/sde"
)

// Dogma attributes read from the skill types. Keep in sync with SkillTypeAttributes.
const (
	attrPrimary   = 180
	attrSecondary = 181
	attrRank      = 275
)

// requiredSkillAttrs pairs the required skill attributes with their level attributes.
var requiredSkillAttrs = [][2]int64{
	{182, 277},
	{183, 278},
	{184, 279},
	{1285, 1286},
	{1289, 1287},
	{1290, 1288},
}

// Character attributes by their dogma attribute ID, as named in the attributes response.
var attributeNames = map[int64]string{
	164: "charisma",
	165: "intelligence",
	166: "memory",
	167: "perception",
	168: "willpower",
}

// Catalog is every published skill, by group, both sorted by name.
type Catalog struct {
	Groups []SkillGroup `json:"groups"`
}

type SkillGroup struct {
	ID     int64       `json:"id"`
	Name   string      `json:"name"`
	Skills []SkillType `json:"skills"`
}

type SkillType struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Rank        int64  `json:"rank"`
	// Attribute names as in CharacterAttribute, empty when the SDE has none.
	Primary   string          `json:"primary"`
	Secondary string          `json:"secondary"`
	Requires  []RequiredSkill `json:"requires"`
}

type RequiredSkill struct {
	SkillID int64 `json:"skillId"`
	Level   int64 `json:"level"`
}

// buildCatalog reads the skill catalog from the installed SDE. The result is
// shared between callers, so it is not modified after.
func buildCatalog(ctx context.Context, q *sde.Queries) (*Catalog, error) {
	groupRows, err := q.SkillGroups(ctx)
	if err != nil {
		return nil, err
	}
	typeRows, err := q.SkillTypes(ctx)
	if err != nil {
		return nil, err
	}
	attrRows, err := q.SkillTypeAttributes(ctx)
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

	byGroup := map[int64][]SkillType{}
	for _, t := range typeRows {
		if t.GroupID == nil {
			continue
		}
		id := int64(t.Key)
		a := attrs[id]
		s := SkillType{
			ID:          id,
			Name:        deref(t.NameEn),
			Description: deref(t.DescriptionEn),
			Rank:        int64(a[attrRank]),
			Primary:     attributeNames[int64(a[attrPrimary])],
			Secondary:   attributeNames[int64(a[attrSecondary])],
			Requires:    []RequiredSkill{},
		}
		for _, p := range requiredSkillAttrs {
			skill, ok := a[p[0]]
			if !ok || skill == 0 {
				continue
			}
			s.Requires = append(s.Requires, RequiredSkill{SkillID: int64(skill), Level: int64(a[p[1]])})
		}
		byGroup[*t.GroupID] = append(byGroup[*t.GroupID], s)
	}

	c := &Catalog{Groups: []SkillGroup{}}
	for _, g := range groupRows {
		skills := byGroup[g.Key]
		// Some published groups only hold unpublished skills.
		if len(skills) == 0 {
			continue
		}
		c.Groups = append(c.Groups, SkillGroup{ID: g.Key, Name: deref(g.NameEn), Skills: skills})
	}
	return c, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
