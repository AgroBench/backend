package handler

import (
	"net/http"

	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/types/input"
)

type MFAVerify struct{ uc contract.MFAVerify }

func NewMFAVerify(uc contract.MFAVerify) *MFAVerify { return &MFAVerify{uc: uc} }

func (h *MFAVerify) Handle(w http.ResponseWriter, r *http.Request) {
	var in input.MFAVerify
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	out, err := h.uc.Execute(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
