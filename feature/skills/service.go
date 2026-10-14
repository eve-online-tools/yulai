package skills

import (
	"context"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/sde"
)

// Service is the Wails-facing side of the stored skills.
type Service struct {
	q       *Queries
	catalog *sde.Derived[*Catalog]
}

func newService(q *Queries, store *sde.Store) *Service {
	return &Service{q: q, catalog: sde.NewDerived(store, buildCatalog)}
}

func (s *Service) ServiceName() string { return "SkillsService" }

// List returns the character's trained skills, empty before the first fetch.
func (s *Service) List(ctx context.Context, characterID int64) ([]Skill, error) {
	rows, err := s.q.ListSkills(ctx, characterID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []Skill{}
	}
	return rows, nil
}

// Queue returns the character's skill queue in order, empty before the first fetch.
func (s *Service) Queue(ctx context.Context, characterID int64) ([]SkillQueue, error) {
	rows, err := s.q.ListQueue(ctx, characterID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []SkillQueue{}
	}
	return rows, nil
}

// Attributes returns the character's attributes, nil before the first fetch.
func (s *Service) Attributes(ctx context.Context, characterID int64) (*CharacterAttribute, error) {
	row, err := s.q.GetAttributes(ctx, characterID)
	found, err := db.Found(err)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return &row, nil
}

// Totals returns the character's total and unallocated SP, nil before the first fetch.
func (s *Service) Totals(ctx context.Context, characterID int64) (*SkillTotal, error) {
	row, err := s.q.GetTotals(ctx, characterID)
	found, err := db.Found(err)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return &row, nil
}

// Catalog returns every published skill by group, built once per installed SDE.
// It fails before the first install; the frontend refetches when the sde topic changes.
func (s *Service) Catalog(ctx context.Context) (*Catalog, error) {
	return s.catalog.Get(ctx)
}
