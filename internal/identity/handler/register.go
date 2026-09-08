package handler

import (
	"net/http"

	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/types/input"
)

type Register struct{ uc contract.Register }

func NewRegister(uc contract.Register) *Register { return &Register{uc: uc} }

func (h *Register) Handle(w http.ResponseWriter, r *http.Request) {
	var in input.Register
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	out, err := h.uc.Execute(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}
