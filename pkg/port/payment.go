package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/core/domain"
)

type CheckoutRequest struct {
	InstitutionID  uuid.UUID
	SubscriptionID uuid.UUID
	Plan           string
	Amount         domain.MicroUSDC
	CustomerEmail  string
}

type Checkout struct {
	ProviderRef string // id da sessão no provedor (ou fake no mock)
	URL         string // pra onde redirecionar a instituição
}

type PaymentEvent struct {
	ProviderRef    string
	SubscriptionID uuid.UUID
	Amount         domain.MicroUSDC
	Confirmed      bool
}

// PaymentGateway cobra a assinatura das instituições (README raiz §6, §7).
//
// Implementações:
//   - pkg/adapter/payment/mock   → ATIVA NO PITCH. CreateCheckout devolve uma URL fake;
//     a confirmação vem por POST /admin/payments/{id}/confirm, que chama ParseWebhook com um
//     JSON simples {"ref": "...", "status": "paid"}.
//   - pkg/adapter/payment/stripe → REAL. Stripe Checkout Session + webhook
//     checkout.session.completed (assinatura verificada com o webhook secret).
//
// Seleção: config `adapters.payment` (mock | stripe).
type PaymentGateway interface {
	CreateCheckout(ctx context.Context, req CheckoutRequest) (Checkout, error)
	// ParseWebhook valida a assinatura e traduz o evento do provedor. Eventos irrelevantes
	// devolvem (PaymentEvent{}, nil) com Confirmed=false.
	ParseWebhook(ctx context.Context, body []byte, signature string) (PaymentEvent, error)
}
