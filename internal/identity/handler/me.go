package handler

import (
	"net/http"

	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/AgroBench/backend/internal/identity/contract"
)

type Me struct{ uc contract.Me }

func NewMe(uc contract.Me) *Me { return &Me{uc: uc} }

func (h *Me) Handle(w http.ResponseWriter, r *http.Request) {
	out, err := h.uc.Execute(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
