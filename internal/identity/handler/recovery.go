package handler

import (
	"net/http"

	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/types/input"
)

type RecoveryStart struct{ uc contract.RecoveryStart }

func NewRecoveryStart(uc contract.RecoveryStart) *RecoveryStart { return &RecoveryStart{uc: uc} }

func (h *RecoveryStart) Handle(w http.ResponseWriter, r *http.Request) {
	var in input.RecoveryStart
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.uc.Execute(r.Context(), in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type RecoveryConfirm struct{ uc contract.RecoveryConfirm }

func NewRecoveryConfirm(uc contract.RecoveryConfirm) *RecoveryConfirm {
	return &RecoveryConfirm{uc: uc}
}

func (h *RecoveryConfirm) Handle(w http.ResponseWriter, r *http.Request) {
	var in input.RecoveryConfirm
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.uc.Execute(r.Context(), in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
