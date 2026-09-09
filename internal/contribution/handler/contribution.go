package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/contribution/types/input"
	"github.com/AgroBench/backend/internal/contribution/usecase"
	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/core/httpx"
	walletcontract "github.com/AgroBench/backend/internal/wallet/contract"
)

type Commit struct{ uc *usecase.Commit }

func NewCommit(uc *usecase.Commit) *Commit { return &Commit{uc: uc} }

func (h *Commit) Handle(w http.ResponseWriter, r *http.Request) {
	var in input.Commit
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

type Reveal struct{ uc *usecase.Reveal }

func NewReveal(uc *usecase.Reveal) *Reveal { return &Reveal{uc: uc} }

func (h *Reveal) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, apperrors.Validation("contribution.Reveal", err).WithDetail("id inválido"))
		return
	}
	var in input.Reveal
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	out, err := h.uc.Execute(r.Context(), auth.UserID(r.Context()), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

type List struct {
	uc      *usecase.List
	wallets walletcontract.WalletRepo
}

func NewList(uc *usecase.List, wallets walletcontract.WalletRepo) *List {
	return &List{uc: uc, wallets: wallets}
}

func (h *List) Handle(w http.ResponseWriter, r *http.Request) {
	wal, err := h.wallets.GetByUserID(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	out, err := h.uc.Execute(r.Context(), wal.ID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

type Get struct{ uc *usecase.Get }

func NewGet(uc *usecase.Get) *Get { return &Get{uc: uc} }

func (h *Get) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, apperrors.Validation("contribution.Get", err).WithDetail("id inválido"))
		return
	}
	out, err := h.uc.Execute(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

type EnclaveKey struct{ uc *usecase.EnclaveKey }

func NewEnclaveKey(uc *usecase.EnclaveKey) *EnclaveKey { return &EnclaveKey{uc: uc} }

func (h *EnclaveKey) Handle(w http.ResponseWriter, r *http.Request) {
	out, err := h.uc.Execute(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

type LockTx struct{ uc *usecase.LockTx }

func NewLockTx(uc *usecase.LockTx) *LockTx { return &LockTx{uc: uc} }

func (h *LockTx) Handle(w http.ResponseWriter, r *http.Request) {
	var in input.LockStakeTx
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	out, err := h.uc.Execute(r.Context(), auth.UserID(r.Context()), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

type LockSubmit struct{ uc *usecase.LockSubmit }

func NewLockSubmit(uc *usecase.LockSubmit) *LockSubmit { return &LockSubmit{uc: uc} }

func (h *LockSubmit) Handle(w http.ResponseWriter, r *http.Request) {
	var in input.SubmitSignedTx
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

type ReleaseTx struct{ uc *usecase.ReleaseTx }

func NewReleaseTx(uc *usecase.ReleaseTx) *ReleaseTx { return &ReleaseTx{uc: uc} }

func (h *ReleaseTx) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, apperrors.Validation("contribution.ReleaseTx", err).WithDetail("id inválido"))
		return
	}
	out, err := h.uc.Execute(r.Context(), auth.UserID(r.Context()), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

type ReleaseSubmit struct{ uc *usecase.ReleaseSubmit }

func NewReleaseSubmit(uc *usecase.ReleaseSubmit) *ReleaseSubmit { return &ReleaseSubmit{uc: uc} }

func (h *ReleaseSubmit) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, apperrors.Validation("contribution.ReleaseSubmit", err).WithDetail("id inválido"))
		return
	}
	var in input.SubmitSignedTx
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	out, err := h.uc.Execute(r.Context(), auth.UserID(r.Context()), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
