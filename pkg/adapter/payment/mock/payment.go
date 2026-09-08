// Package mock é a implementação de port.PaymentGateway ATIVA NO PITCH.
// Não há cobrança: o checkout devolve uma URL fake e a confirmação é feita pelo admin
// (POST /admin/payments/{id}/confirm), que monta o "webhook" abaixo.
package mock

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/pkg/port"
)

type Gateway struct {
	baseURL string
}

func New(baseURL string) *Gateway { return &Gateway{baseURL: baseURL} }

func (g *Gateway) CreateCheckout(_ context.Context, req port.CheckoutRequest) (port.Checkout, error) {
	ref := "mock_cs_" + req.SubscriptionID.String()
	return port.Checkout{
		ProviderRef: ref,
		URL:         fmt.Sprintf("%s/mock/checkout/%s", g.baseURL, ref),
	}, nil
}

// WebhookBody é o formato aceito por ParseWebhook no mock.
type WebhookBody struct {
	Ref            string    `json:"ref"`
	SubscriptionID uuid.UUID `json:"subscription_id"`
	AmountUSDC     float64   `json:"amount_usdc"`
	Status         string    `json:"status"` // "paid" | qualquer outra coisa = ignorado
}

// ParseWebhook ignora a assinatura (mock) e lê o JSON simples.
func (g *Gateway) ParseWebhook(_ context.Context, body []byte, _ string) (port.PaymentEvent, error) {
	var wb WebhookBody
	if err := json.Unmarshal(body, &wb); err != nil {
		return port.PaymentEvent{}, fmt.Errorf("payment mock: webhook inválido: %w", err)
	}
	return port.PaymentEvent{
		ProviderRef:    wb.Ref,
		SubscriptionID: wb.SubscriptionID,
		Amount:         domain.USDC(wb.AmountUSDC),
		Confirmed:      wb.Status == "paid",
	}, nil
}

// BuildWebhook monta o body que o endpoint admin envia ao ParseWebhook.
func BuildWebhook(ref string, subscriptionID uuid.UUID, amount domain.MicroUSDC) []byte {
	b, _ := json.Marshal(WebhookBody{Ref: ref, SubscriptionID: subscriptionID, AmountUSDC: amount.Float(), Status: "paid"})
	return b
}
