package di

import (
	"github.com/jmoiron/sqlx"

	"github.com/AgroBench/backend/internal/identity/contract"
	"github.com/AgroBench/backend/internal/identity/handler"
	"github.com/AgroBench/backend/internal/identity/repository"
	"github.com/AgroBench/backend/internal/identity/usecase"
	"github.com/AgroBench/backend/pkg/adapter/registry"
)

type Handlers struct {
	Register        *handler.Register
	Login           *handler.Login
	MFAVerify       *handler.MFAVerify
	Refresh         *handler.Refresh
	Logout          *handler.Logout
	RecoveryStart   *handler.RecoveryStart
	RecoveryConfirm *handler.RecoveryConfirm
	Me              *handler.Me
	GetPendingOTP   *handler.GetPendingOTP
}

func New(db *sqlx.DB, adapters *registry.Adapters, wallets contract.WalletPubkeyFinder) *Handlers {
	users := repository.NewUserRepo(db)
	otps := repository.NewOTPRepo(db)
	refresh := repository.NewRefreshRepo(db)
	sms := adapters.SMS

	h := &Handlers{
		Register:        handler.NewRegister(usecase.NewRegister(users, otps, sms)),
		Login:           handler.NewLogin(usecase.NewLogin(users, otps, sms, wallets, refresh)),
		MFAVerify:       handler.NewMFAVerify(usecase.NewMFAVerify(users, otps, otps, wallets, refresh)),
		Refresh:         handler.NewRefresh(usecase.NewRefresh(users, refresh, refresh, wallets)),
		Logout:          handler.NewLogout(usecase.NewLogout(refresh, refresh)),
		RecoveryStart:   handler.NewRecoveryStart(usecase.NewRecoveryStart(users, otps, sms)),
		RecoveryConfirm: handler.NewRecoveryConfirm(usecase.NewRecoveryConfirm(users, users, otps, otps, refresh)),
		Me:              handler.NewMe(usecase.NewMe(users)),
	}
	if adapters.SMSMock != nil {
		h.GetPendingOTP = handler.NewGetPendingOTP(usecase.NewGetPendingOTP(users, adapters.SMSMock))
	}
	return h
}
