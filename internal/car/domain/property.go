package domain

import (
	"time"

	"github.com/google/uuid"
)

type CARStatus string

const (
	CARPending  CARStatus = "pending"
	CARApproved CARStatus = "approved"
	CARRejected CARStatus = "rejected"
)

type Property struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	CARHMAC       string
	MicroRegionID uuid.UUID
	CARStatus     CARStatus
	VerifiedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type MicroRegion struct {
	ID       uuid.UUID `json:"id" db:"id"`
	IBGECode string    `json:"ibge_code" db:"ibge_code"`
	Name     string    `json:"name" db:"name"`
	UF       string    `json:"uf" db:"uf"`
}

type Culture struct {
	ID   uuid.UUID `json:"id" db:"id"`
	Code string    `json:"code" db:"code"`
	Name string    `json:"name" db:"name"`
}
