package domain

import (
	"time"

	"github.com/google/uuid"
)

type CycleStatus string

const (
	CycleOpen       CycleStatus = "open"
	CycleClosed     CycleStatus = "closed"
	CycleAggregated CycleStatus = "aggregated"
)

type Cycle struct {
	ID            uuid.UUID
	CultureID     uuid.UUID
	MicroRegionID uuid.UUID
	Label         string
	OpensAt       time.Time
	ClosesAt      time.Time
	Status        CycleStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (c Cycle) IsOpen(now time.Time) bool {
	return c.Status == CycleOpen && !now.Before(c.OpensAt) && now.Before(c.ClosesAt)
}
