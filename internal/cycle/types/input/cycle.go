package input

import (
	"time"

	"github.com/google/uuid"
)

type CreateCycle struct {
	CultureID     uuid.UUID `json:"culture_id"      validate:"required"`
	MicroRegionID uuid.UUID `json:"micro_region_id" validate:"required"`
	Label         string    `json:"label"           validate:"required,min=4,max=20"`
	OpensAt       time.Time `json:"opens_at"         validate:"required"`
	ClosesAt      time.Time `json:"closes_at"        validate:"required"`
}

type ListCycles struct {
	CultureID string `json:"culture"`
	RegionID  string `json:"region"`
	Status    string `json:"status"`
}
