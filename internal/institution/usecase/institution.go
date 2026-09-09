package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"

	"github.com/AgroBench/backend/internal/apperrors"
	"github.com/AgroBench/backend/internal/core/auth"
	"github.com/AgroBench/backend/internal/core/crypto"
	coredomain "github.com/AgroBench/backend/internal/core/domain"
	iddomain "github.com/AgroBench/backend/internal/identity/domain"
	idrepo "github.com/AgroBench/backend/internal/identity/repository"
	paymentmock "github.com/AgroBench/backend/pkg/adapter/payment/mock"
	"github.com/AgroBench/backend/pkg/port"
)

type Institution struct {
	ID     uuid.UUID `json:"id" db:"id"`
	UserID uuid.UUID `json:"-" db:"user_id"`
	Name   string    `json:"name" db:"name"`
	Status string    `json:"status" db:"status"`
}

type RegisterInput struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	CNPJ     string `json:"cnpj"     validate:"required,min=14,max=18"`
	Name     string `json:"name"     validate:"required,min=2,max=160"`
}

type SubscribeInput struct {
	Plan    string      `json:"plan"    validate:"required,oneof=regional national"`
	Regions []uuid.UUID `json:"regions"`
}

type RegisterInstitution struct{ db *sqlx.DB }

func NewRegister(db *sqlx.DB) *RegisterInstitution { return &RegisterInstitution{db: db} }

func (u *RegisterInstitution) Execute(ctx context.Context, in RegisterInput) (Institution, error) {
	const op = "institution.Register"
	cnpj := crypto.NormalizeIdentifier(in.CNPJ)
	if len(cnpj) != 14 {
		return Institution{}, apperrors.Validation(op, errors.New("cnpj")).WithDetail("CNPJ inválido")
	}
	hash, err := crypto.HashPassword(in.Password)
	if err != nil {
		return Institution{}, apperrors.Internal(op, err)
	}
	users := idrepo.NewUserRepo(u.db)
	user := iddomain.User{
		ID: coredomain.NewID(), Email: strings.ToLower(in.Email), Phone: "+5500000000000",
		CPFHMAC: auth.HMACIdentifier("CNPJ:" + cnpj), PasswordHash: hash,
		Role: coredomain.RoleInstitution, MFAEnabled: false, Status: iddomain.UserActive,
	}
	if err := users.Create(ctx, user); err != nil {
		if apperrors.Is(err, apperrors.ErrConflict) {
			return Institution{}, apperrors.Conflict(op, err).WithDetail("email ou CNPJ já cadastrado")
		}
		return Institution{}, err
	}
	inst := Institution{ID: coredomain.NewID(), UserID: user.ID, Name: in.Name, Status: "pending"}
	_, err = u.db.ExecContext(ctx, `
		INSERT INTO institutions (id, user_id, name, cnpj_hmac, status) VALUES ($1,$2,$3,$4,'pending')`,
		inst.ID, user.ID, in.Name, auth.HMACIdentifier(cnpj))
	if err != nil {
		return Institution{}, apperrors.FromDBError(op, err)
	}
	return inst, nil
}

type Me struct{ db *sqlx.DB }

func NewMe(db *sqlx.DB) *Me { return &Me{db: db} }

func (u *Me) Execute(ctx context.Context, userID uuid.UUID) (Institution, error) {
	var inst Institution
	err := sqlx.GetContext(ctx, u.db, &inst, `SELECT id, user_id, name, status FROM institutions WHERE user_id = $1`, userID)
	if err != nil {
		return Institution{}, apperrors.NotFound("institution.Me", err).WithDetail("instituição não encontrada")
	}
	return inst, nil
}

type Decide struct{ db *sqlx.DB }

func NewDecide(db *sqlx.DB) *Decide { return &Decide{db: db} }

func (u *Decide) Execute(ctx context.Context, id uuid.UUID, approve bool) error {
	status := "rejected"
	var approved any
	if approve {
		status = "approved"
		approved = time.Now()
	}
	res, err := u.db.ExecContext(ctx, `UPDATE institutions SET status = $2, approved_at = $3 WHERE id = $1`, id, status, approved)
	if err != nil {
		return apperrors.FromDBError("institution.Decide", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NotFound("institution.Decide", errors.New("nf")).WithDetail("instituição não encontrada")
	}
	return nil
}

type Subscribe struct {
	db      *sqlx.DB
	payment port.PaymentGateway
}

func NewSubscribe(db *sqlx.DB, pay port.PaymentGateway) *Subscribe {
	return &Subscribe{db: db, payment: pay}
}

type CheckoutOut struct {
	SubscriptionID uuid.UUID `json:"subscription_id"`
	CheckoutURL    string    `json:"checkout_url"`
	ProviderRef    string    `json:"provider_ref"`
}

func (u *Subscribe) Execute(ctx context.Context, userID uuid.UUID, in SubscribeInput) (CheckoutOut, error) {
	const op = "institution.Subscribe"
	inst, err := NewMe(u.db).Execute(ctx, userID)
	if err != nil {
		return CheckoutOut{}, err
	}
	if inst.Status != "approved" {
		return CheckoutOut{}, apperrors.Forbidden(op, errors.New("pending")).WithDetail("instituição ainda não aprovada")
	}
	max := viper.GetInt("plans.regional.max_regions")
	if max <= 0 {
		max = 5
	}
	if in.Plan == "regional" {
		if len(in.Regions) == 0 || len(in.Regions) > max {
			return CheckoutOut{}, apperrors.Validation(op, errors.New("regions")).WithDetail("plano regional exige 1 a 5 microrregiões")
		}
	}
	price := viper.GetFloat64("plans." + in.Plan + ".price_usdc")
	if in.Plan == "national" {
		price = viper.GetFloat64("plans.national.price_usdc")
	} else {
		price = viper.GetFloat64("plans.regional.price_usdc")
	}
	subID := coredomain.NewID()
	now := time.Now()
	_, err = u.db.ExecContext(ctx, `
		INSERT INTO subscriptions (id, institution_id, plan, regions, period_start, period_end, status)
		VALUES ($1,$2,$3,$4,$5,$6,'pending')`,
		subID, inst.ID, in.Plan, in.Regions, now, now.AddDate(0, 1, 0))
	if err != nil {
		return CheckoutOut{}, apperrors.FromDBError(op, err)
	}
	co, err := u.payment.CreateCheckout(ctx, port.CheckoutRequest{
		InstitutionID: inst.ID, SubscriptionID: subID, Plan: in.Plan,
		Amount: coredomain.USDC(price), CustomerEmail: "",
	})
	if err != nil {
		return CheckoutOut{}, apperrors.External(op, 0, err)
	}
	_, _ = u.db.ExecContext(ctx, `
		INSERT INTO payments (id, subscription_id, amount, provider, provider_ref)
		VALUES ($1,$2,$3,$4,$5)`,
		coredomain.NewID(), subID, price, "mock", co.ProviderRef)
	return CheckoutOut{SubscriptionID: subID, CheckoutURL: co.URL, ProviderRef: co.ProviderRef}, nil
}

type ConfirmPayment struct {
	db      *sqlx.DB
	payment port.PaymentGateway
	chain   port.ChainClient
}

func NewConfirmPayment(db *sqlx.DB, pay port.PaymentGateway, chain port.ChainClient) *ConfirmPayment {
	return &ConfirmPayment{db: db, payment: pay, chain: chain}
}

func (u *ConfirmPayment) Execute(ctx context.Context, paymentID uuid.UUID) error {
	const op = "institution.ConfirmPayment"
	var p struct {
		ID     uuid.UUID `db:"id"`
		SubID  uuid.UUID `db:"subscription_id"`
		Amount float64   `db:"amount"`
		Ref    string    `db:"provider_ref"`
	}
	if err := sqlx.GetContext(ctx, u.db, &p, `SELECT id, subscription_id, amount, provider_ref FROM payments WHERE id = $1`, paymentID); err != nil {
		return apperrors.NotFound(op, err).WithDetail("pagamento não encontrado")
	}
	body := paymentmock.BuildWebhook(p.Ref, p.SubID, coredomain.USDC(p.Amount))
	ev, err := u.payment.ParseWebhook(ctx, body, "")
	if err != nil {
		return apperrors.External(op, 0, err)
	}
	if !ev.Confirmed {
		return apperrors.Validation(op, errors.New("not paid")).WithDetail("pagamento não confirmado")
	}
	tx, err := u.chain.CreditPool(ctx, coredomain.USDC(p.Amount), "pool:"+p.SubID.String())
	if err != nil {
		return apperrors.External(op, 0, err)
	}
	_, err = u.db.ExecContext(ctx, `UPDATE payments SET confirmed_at = now(), pool_tx = $2 WHERE id = $1`, p.ID, string(tx))
	if err != nil {
		return apperrors.FromDBError(op, err)
	}
	_, err = u.db.ExecContext(ctx, `UPDATE subscriptions SET status = 'active' WHERE id = $1`, p.SubID)
	return err
}

type Webhook struct {
	db      *sqlx.DB
	payment port.PaymentGateway
	chain   port.ChainClient
}

func NewWebhook(db *sqlx.DB, pay port.PaymentGateway, chain port.ChainClient) *Webhook {
	return &Webhook{db: db, payment: pay, chain: chain}
}

func (u *Webhook) Execute(ctx context.Context, body []byte, sig string) error {
	ev, err := u.payment.ParseWebhook(ctx, body, sig)
	if err != nil {
		return apperrors.External("institution.Webhook", 0, err)
	}
	if !ev.Confirmed {
		return nil
	}
	var paymentID uuid.UUID
	err = sqlx.GetContext(ctx, u.db, &paymentID, `SELECT id FROM payments WHERE provider_ref = $1`, ev.ProviderRef)
	if err != nil {
		return nil
	}
	return NewConfirmPayment(u.db, u.payment, u.chain).Execute(ctx, paymentID)
}
