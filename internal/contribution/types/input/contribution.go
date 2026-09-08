package input

import "github.com/google/uuid"

type Commit struct {
	CycleID    uuid.UUID `json:"cycle_id"    validate:"required"`
	Level      string    `json:"level"       validate:"required,oneof=basic intermediate advanced"`
	Hash       string    `json:"hash"        validate:"required,len=64"`
	PropertyID uuid.UUID `json:"property_id" validate:"required"`
}

type Reveal struct {
	Ciphertext string `json:"ciphertext" validate:"required"` // base64
}
