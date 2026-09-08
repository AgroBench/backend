package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/AgroBench/backend/internal/identity/usecase"
)

type GetPendingOTP struct{ uc *usecase.GetPendingOTP }

func NewGetPendingOTP(uc *usecase.GetPendingOTP) *GetPendingOTP { return &GetPendingOTP{uc: uc} }

func (h *GetPendingOTP) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "user_id"))
	if err != nil {
		httpx.Error(w, r, apperrors.Validation("identity.GetPendingOTP", err).WithDetail("user_id inválido"))
		return
	}
	out, err := h.uc.Execute(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
