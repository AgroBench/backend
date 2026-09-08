package handler

import (
	"net/http"

	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/types/input"
)

type Refresh struct{ uc contract.Refresh }

func NewRefresh(uc contract.Refresh) *Refresh { return &Refresh{uc: uc} }

func (h *Refresh) Handle(w http.ResponseWriter, r *http.Request) {
	var in input.Refresh
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
