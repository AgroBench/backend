package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/AgroBench/backend/internal/cycle/contract"
	"github.com/AgroBench/backend/internal/cycle/types/input"
)

type Create struct{ uc contract.Create }

func NewCreate(uc contract.Create) *Create { return &Create{uc: uc} }

func (h *Create) Handle(w http.ResponseWriter, r *http.Request) {
	var in input.CreateCycle
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

type List struct{ uc contract.List }

func NewList(uc contract.List) *List { return &List{uc: uc} }

func (h *List) Handle(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := h.uc.Execute(r.Context(), input.ListCycles{
		CultureID: q.Get("culture"), RegionID: q.Get("region"), Status: q.Get("status"),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

type Close struct{ uc contract.Close }

func NewClose(uc contract.Close) *Close { return &Close{uc: uc} }

func (h *Close) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, apperrors.Validation("cycle.Close", err).WithDetail("id inválido"))
		return
	}
	out, err := h.uc.Execute(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
