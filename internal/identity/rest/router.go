package rest

import (
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/di"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

func Register(r chi.Router, db *sqlx.DB, adapters *registry.Adapters, wallets contract.WalletPubkeyFinder) {
	h := di.New(db, adapters, wallets)

	r.Post("/auth/register", h.Register.Handle)
	r.Post("/auth/login", h.Login.Handle)
	r.Post("/auth/mfa/verify", h.MFAVerify.Handle)
	r.Post("/auth/refresh", h.Refresh.Handle)
	r.Post("/auth/logout", h.Logout.Handle)
	r.Post("/auth/recovery/start", h.RecoveryStart.Handle)
	r.Post("/auth/recovery/confirm", h.RecoveryConfirm.Handle)

	r.Group(func(priv chi.Router) {
		priv.Use(auth.Require())
		priv.Get("/me", h.Me.Handle)
	})

	// Demo: último OTP do mock. Só existe com adapters.sms=mock (README §7).
	if h.GetPendingOTP != nil {
		r.Get("/admin/otp/{user_id}", h.GetPendingOTP.Handle)
	}
}
