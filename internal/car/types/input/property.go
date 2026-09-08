package input

import "github.com/google/uuid"

type CreateProperty struct {
	CAR           string    `json:"car"             validate:"required,min=8,max=60"`
	MicroRegionID uuid.UUID `json:"micro_region_id" validate:"required"`
}
