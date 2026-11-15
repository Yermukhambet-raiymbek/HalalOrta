package store

import "context"

// PlanInterest is how much of a family has asked for the paid plan.
type PlanInterest struct {
	// Interested is true when the member has registered interest.
	Interested bool `json:"interested"`
	// FamilyInterested counts interested members of the member's family.
	FamilyInterested int `json:"family_interested"`
	FamilyMembers    int `json:"family_members"`
}

// PlanInterestFor returns the member's own interest and their family's
// totals. Callers must pass both IDs from the session.
func (s *Store) PlanInterestFor(ctx context.Context, familyID, memberID string) (PlanInterest, error) {
	var p PlanInterest
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM premium_interest WHERE member_id = $2),
		       (SELECT count(*) FROM premium_interest pi
		        JOIN members m ON m.id = pi.member_id WHERE m.family_id = $1),
		       (SELECT count(*) FROM members WHERE family_id = $1)`,
		familyID, memberID,
	).Scan(&p.Interested, &p.FamilyInterested, &p.FamilyMembers)
	return p, err
}

// RegisterPlanInterest records that the member would use the paid plan.
// Repeating it changes nothing.
func (s *Store) RegisterPlanInterest(ctx context.Context, memberID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO premium_interest (member_id) VALUES ($1)
		ON CONFLICT (member_id) DO NOTHING`, memberID)
	return err
}
