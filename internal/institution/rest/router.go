package rest

import (
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/core/auth"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/institution/handler"
	"github.com/AgroBench/backend/internal/institution/usecase"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

func Register(r chi.Router, db *sqlx.DB, adapters *registry.Adapters) {
	reg := handler.NewRegister(usecase.NewRegister(db))
	me := handler.NewMe(usecase.NewMe(db))
	dec := handler.NewDecide(usecase.NewDecide(db))
	sub := handler.NewSubscribe(usecase.NewSubscribe(db, adapters.Payment))
	conf := handler.NewConfirm(usecase.NewConfirmPayment(db, adapters.Payment, adapters.Chain))
	hook := handler.NewWebhook(usecase.NewWebhook(db, adapters.Payment, adapters.Chain))

	r.Post("/institutions/register", reg.Handle)
	r.Post("/webhooks/payment", hook.Handle)
	r.Group(func(inst chi.Router) {
		inst.Use(auth.Require(coredomain.RoleInstitution))
		inst.Get("/institutions/me", me.Handle)
		inst.Post("/institutions/subscribe", sub.Handle)
	})
	r.Group(func(adm chi.Router) {
		adm.Use(auth.Require(coredomain.RoleAdmin))
		adm.Post("/admin/institutions/{id}/approve", dec.Handle)
		adm.Post("/admin/institutions/{id}/reject", dec.Handle)
		adm.Post("/admin/payments/{id}/confirm", conf.Handle)
	})
}
