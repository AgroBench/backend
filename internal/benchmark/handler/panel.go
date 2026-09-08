package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/benchmark/usecase"
	"github.com/AgroBench/backend/internal/core/auth"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/spf13/viper"
)

type Me struct{ uc *usecase.Me }

func NewMe(uc *usecase.Me) *Me { return &Me{uc: uc} }

func (h *Me) Handle(w http.ResponseWriter, r *http.Request) {
	cycleID, err := uuid.Parse(r.URL.Query().Get("cycle"))
	if err != nil {
		httpx.Error(w, r, apperrors.Validation("benchmark.Me", err).WithDetail("cycle inválido"))
		return
	}
	out, err := h.uc.Execute(r.Context(), auth.UserID(r.Context()), cycleID)
	if err != nil {
		if apperrors.Is(err, apperrors.ErrForbidden) {
			need := viper.GetInt("benchmark.free_after_cycles")
			if need <= 0 {
				need = 3
			}
			detail := ""
			var app *apperrors.AppError
			if errors.As(err, &app) {
				detail = app.Detail
			}
			httpx.JSON(w, http.StatusForbidden, map[string]any{
				"code": "FORBIDDEN", "message": "Painel bloqueado",
				"detail": detail, "cycles_validated": 0, "cycles_required": need,
			})
			return
		}
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

type Report struct{ uc *usecase.Report }

func NewReport(uc *usecase.Report) *Report { return &Report{uc: uc} }

func (h *Report) Handle(w http.ResponseWriter, r *http.Request) {
	cycleID, err := uuid.Parse(r.URL.Query().Get("cycle"))
	if err != nil {
		httpx.Error(w, r, apperrors.Validation("benchmark.Report", err).WithDetail("cycle inválido"))
		return
	}
	out, err := h.uc.Execute(r.Context(), auth.UserID(r.Context()), cycleID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

var _ = coredomain.RoleInstitution
