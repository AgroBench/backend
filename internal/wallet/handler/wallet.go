package handler

import (
	"net/http"

	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/core/httpx"
	"github.com/AgroBench/backend/internal/wallet/contract"
	"github.com/AgroBench/backend/internal/wallet/types/input"
	"github.com/AgroBench/backend/internal/wallet/usecase"
)

type Create struct{ uc contract.Create }

func NewCreate(uc contract.Create) *Create { return &Create{uc: uc} }

func (h *Create) Handle(w http.ResponseWriter, r *http.Request) {
	var in input.CreateWallet
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

type Get struct{ uc contract.Get }

func NewGet(uc contract.Get) *Get { return &Get{uc: uc} }

func (h *Get) Handle(w http.ResponseWriter, r *http.Request) {
	out, err := h.uc.Execute(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

type Export struct {
	export *usecase.Export
	otp    *usecase.RequestExportOTP
}

func NewExport(export *usecase.Export, otp *usecase.RequestExportOTP) *Export {
	return &Export{export: export, otp: otp}
}

func (h *Export) Handle(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		if err := h.otp.Execute(r.Context(), auth.UserID(r.Context())); err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusForbidden, map[string]any{
			"code": "FORBIDDEN", "message": "OTP enviado", "otp_required": true,
		})
		return
	}
	out, err := h.export.Execute(r.Context(), auth.UserID(r.Context()), code)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
