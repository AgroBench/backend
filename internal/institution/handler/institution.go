package handler

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/AgroBench/backend/internal/institution/usecase"
)

type Register struct{ uc *usecase.RegisterInstitution }

func NewRegister(uc *usecase.RegisterInstitution) *Register { return &Register{uc: uc} }

func (h *Register) Handle(w http.ResponseWriter, r *http.Request) {
	var in usecase.RegisterInput
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

type Me struct{ uc *usecase.Me }

func NewMe(uc *usecase.Me) *Me { return &Me{uc: uc} }

func (h *Me) Handle(w http.ResponseWriter, r *http.Request) {
	out, err := h.uc.Execute(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

type Decide struct{ uc *usecase.Decide }

func NewDecide(uc *usecase.Decide) *Decide { return &Decide{uc: uc} }

func (h *Decide) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, apperrors.Validation("institution.Decide", err).WithDetail("id inválido"))
		return
	}
	approve := r.URL.Path[len(r.URL.Path)-7:] != "reject" && chi.URLParam(r, "action") != "reject"
	_ = approve
	ok := r.Method == http.MethodPost
	_ = ok
	approve = true
	if len(r.URL.Path) >= 6 && r.URL.Path[len(r.URL.Path)-6:] == "reject" {
		approve = false
	}
	if err := h.uc.Execute(r.Context(), id, approve); err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type Subscribe struct{ uc *usecase.Subscribe }

func NewSubscribe(uc *usecase.Subscribe) *Subscribe { return &Subscribe{uc: uc} }

func (h *Subscribe) Handle(w http.ResponseWriter, r *http.Request) {
	var in usecase.SubscribeInput
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

type Confirm struct{ uc *usecase.ConfirmPayment }

func NewConfirm(uc *usecase.ConfirmPayment) *Confirm { return &Confirm{uc: uc} }

func (h *Confirm) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, apperrors.Validation("institution.Confirm", err).WithDetail("id inválido"))
		return
	}
	if err := h.uc.Execute(r.Context(), id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type Webhook struct{ uc *usecase.Webhook }

func NewWebhook(uc *usecase.Webhook) *Webhook { return &Webhook{uc: uc} }

func (h *Webhook) Handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.Error(w, r, apperrors.Validation("institution.Webhook", err))
		return
	}
	if err := h.uc.Execute(r.Context(), body, r.Header.Get("Stripe-Signature")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
