package output

import (
	"time"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/cycle/domain"
)

type Cycle struct {
	ID            uuid.UUID `json:"id"`
	CultureID     uuid.UUID `json:"culture_id"`
	MicroRegionID uuid.UUID `json:"micro_region_id"`
	Label         string    `json:"label"`
	OpensAt       string    `json:"opens_at"`
	ClosesAt      string    `json:"closes_at"`
	Status        string    `json:"status"`
}

func NewCycle(c domain.Cycle) Cycle {
	return Cycle{
		ID: c.ID, CultureID: c.CultureID, MicroRegionID: c.MicroRegionID,
		Label: c.Label, Status: string(c.Status),
		OpensAt: c.OpensAt.UTC().Format(time.RFC3339), ClosesAt: c.ClosesAt.UTC().Format(time.RFC3339),
	}
}

func NewCycles(list []domain.Cycle) []Cycle {
	out := make([]Cycle, len(list))
	for i, c := range list {
		out[i] = NewCycle(c)
	}
	return out
}
