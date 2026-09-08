package handler

import (
	"net/http"

	"github.com/AgroBench/backend/internal/car/contract"
	"github.com/AgroBench/backend/internal/car/types/input"
	"github.com/AgroBench/backend/internal/car/usecase"
	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/core/httpx"
)

type CreateProperty struct{ uc contract.CreateProperty }

func NewCreateProperty(uc contract.CreateProperty) *CreateProperty { return &CreateProperty{uc: uc} }

func (h *CreateProperty) Handle(w http.ResponseWriter, r *http.Request) {
	var in input.CreateProperty
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	out, err := h.uc.Execute(r.Context(), auth.UserID(r.Context()), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

type GetProperty struct{ uc contract.GetProperty }

func NewGetProperty(uc contract.GetProperty) *GetProperty { return &GetProperty{uc: uc} }

func (h *GetProperty) Handle(w http.ResponseWriter, r *http.Request) {
	out, err := h.uc.Execute(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

type ListRegions struct{ uc *usecase.ListRegions }

func NewListRegions(uc *usecase.ListRegions) *ListRegions { return &ListRegions{uc: uc} }

func (h *ListRegions) Handle(w http.ResponseWriter, r *http.Request) {
	out, err := h.uc.Execute(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

type ListCultures struct{ uc *usecase.ListCultures }

func NewListCultures(uc *usecase.ListCultures) *ListCultures { return &ListCultures{uc: uc} }

func (h *ListCultures) Handle(w http.ResponseWriter, r *http.Request) {
	out, err := h.uc.Execute(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
