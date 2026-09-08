package output

import (
	"time"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/car/domain"
)

type Property struct {
	ID            uuid.UUID `json:"id"`
	MicroRegionID uuid.UUID `json:"micro_region_id"`
	CARStatus     string    `json:"car_status"`
	VerifiedAt    *string   `json:"verified_at,omitempty"`
}

func NewProperty(p domain.Property) Property {
	out := Property{
		ID:            p.ID,
		MicroRegionID: p.MicroRegionID,
		CARStatus:     string(p.CARStatus),
	}
	if p.VerifiedAt != nil {
		s := p.VerifiedAt.UTC().Format(time.RFC3339)
		out.VerifiedAt = &s
	}
	return out
}
