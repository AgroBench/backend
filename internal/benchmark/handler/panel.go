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
			httpx.JSON(w, http.StatusForbidden, panelForbiddenJSON(err))
			return
		}
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func panelForbiddenJSON(err error) map[string]any {
	need := viper.GetInt("benchmark.free_after_cycles")
	if need <= 0 {
		need = 3
	}
	cyclesValidated := 0
	var blocked *usecase.PanelBlocked
	if errors.As(err, &blocked) {
		cyclesValidated = blocked.CyclesValidated
		if blocked.CyclesRequired > 0 {
			need = blocked.CyclesRequired
		}
	}
	detail := ""
	var app *apperrors.AppError
	if errors.As(err, &app) {
		detail = app.Detail
	}
	return map[string]any{
		"code": "FORBIDDEN", "message": "Painel bloqueado",
		"detail": detail, "cycles_validated": cyclesValidated, "cycles_required": need,
	}
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
