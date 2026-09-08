package handler

import (
	"net/http"

	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/types/input"
)

type Logout struct{ uc contract.Logout }

func NewLogout(uc contract.Logout) *Logout { return &Logout{uc: uc} }

func (h *Logout) Handle(w http.ResponseWriter, r *http.Request) {
	var in input.Logout
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
