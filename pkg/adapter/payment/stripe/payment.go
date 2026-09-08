// Package stripe é a implementação REAL de port.PaymentGateway (NÃO ativa no pitch).
//
// Fluxo: CreateCheckout cria uma Checkout Session (pagamento único, BRL ou USD conforme
// config) com subscription_id no client_reference_id e nos metadados; o webhook
// checkout.session.completed confirma o pagamento e devolve o valor pago.
//
// Status: semi-pronto. Segue o stripe-go v83; não foi executado em modo test dentro do
// escopo do MVP. Passos que faltam: cadastrar o webhook no dashboard apontando para
// POST /api/v1/webhooks/payment e definir STRIPE_WEBHOOK_SECRET.
package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	stripe "github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/webhook"

	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/pkg/port"
)

var _ port.PaymentGateway = (*Gateway)(nil)

type Config struct {
	SecretKey     string
	WebhookSecret string
	SuccessURL    string
	CancelURL     string
	Currency      string  // "brl" | "usd". Default "usd" (1 USDC ≈ 1 USD).
	FiatPerUSDC   float64 // taxa de conversão quando Currency != usd. Default 1.
}

type Gateway struct {
	cfg    Config
	client *stripe.Client
}

func New(cfg Config) (*Gateway, error) {
	if cfg.SecretKey == "" || cfg.WebhookSecret == "" || cfg.SuccessURL == "" || cfg.CancelURL == "" {
		return nil, errors.New("payment stripe: secret_key, webhook_secret, success_url e cancel_url são obrigatórios")
	}
	if cfg.Currency == "" {
		cfg.Currency = "usd"
	}
	if cfg.FiatPerUSDC == 0 {
		cfg.FiatPerUSDC = 1
	}
	return &Gateway{cfg: cfg, client: stripe.NewClient(cfg.SecretKey)}, nil
}

func (g *Gateway) CreateCheckout(ctx context.Context, req port.CheckoutRequest) (port.Checkout, error) {
	// Stripe cobra em centavos da moeda fiat.
	unitAmount := int64(req.Amount.Float() * g.cfg.FiatPerUSDC * 100)

	params := &stripe.CheckoutSessionCreateParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:        stripe.String(g.cfg.SuccessURL),
		CancelURL:         stripe.String(g.cfg.CancelURL),
		ClientReferenceID: stripe.String(req.SubscriptionID.String()),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{{
			Quantity: stripe.Int64(1),
			PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
				Currency:   stripe.String(g.cfg.Currency),
				UnitAmount: stripe.Int64(unitAmount),
				ProductData: &stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{
					Name: stripe.String(fmt.Sprintf("AgroBench — plano %s", req.Plan)),
				},
			},
		}},
	}
	if req.CustomerEmail != "" {
		params.CustomerEmail = stripe.String(req.CustomerEmail)
	}
	params.AddMetadata("subscription_id", req.SubscriptionID.String())
	params.AddMetadata("institution_id", req.InstitutionID.String())
	params.AddMetadata("amount_usdc", fmt.Sprintf("%.6f", req.Amount.Float()))

	sess, err := g.client.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return port.Checkout{}, fmt.Errorf("payment stripe: criando checkout: %w", err)
	}
	return port.Checkout{ProviderRef: sess.ID, URL: sess.URL}, nil
}

func (g *Gateway) ParseWebhook(_ context.Context, body []byte, signature string) (port.PaymentEvent, error) {
	event, err := webhook.ConstructEvent(body, signature, g.cfg.WebhookSecret)
	if err != nil {
		return port.PaymentEvent{}, fmt.Errorf("payment stripe: assinatura do webhook inválida: %w", err)
	}
	if event.Type != stripe.EventTypeCheckoutSessionCompleted {
		return port.PaymentEvent{}, nil // evento irrelevante
	}

	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return port.PaymentEvent{}, fmt.Errorf("payment stripe: sessão inválida no evento: %w", err)
	}
	subID, err := uuid.Parse(sess.ClientReferenceID)
	if err != nil {
		return port.PaymentEvent{}, fmt.Errorf("payment stripe: client_reference_id não é um subscription_id: %w", err)
	}

	// Valor em USDC: prefere o metadado gravado no checkout; senão converte do total pago.
	amount := domain.MicroUSDC(0)
	if s := sess.Metadata["amount_usdc"]; s != "" {
		var f float64
		if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
			amount = domain.USDC(f)
		}
	}
	if amount == 0 {
		amount = domain.USDC(float64(sess.AmountTotal) / 100 / g.cfg.FiatPerUSDC)
	}

	return port.PaymentEvent{
		ProviderRef:    sess.ID,
		SubscriptionID: subID,
		Amount:         amount,
		Confirmed:      strings.EqualFold(string(sess.PaymentStatus), "paid"),
	}, nil
}
