package skills

import (
	"context"
	"database/sql"
)

// Service is the Wails-facing side of the stored skills.
type Service struct {
	q *Queries
}

func NewService(conn *sql.DB) *Service { return &Service{q: New(conn)} }

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
