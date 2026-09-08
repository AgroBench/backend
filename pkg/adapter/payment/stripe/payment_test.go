package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	libstripe "github.com/stripe/stripe-go/v83"

	"github.com/AgroBench/backend/pkg/port"
)

func TestNewRequiresFields(t *testing.T) {
	_, err := New(Config{})
	require.Error(t, err)
	_, err = New(Config{SecretKey: "sk_test_x", WebhookSecret: "whsec_x"})
	require.Error(t, err)
}

func TestParseWebhookCheckoutCompleted(t *testing.T) {
	secret := "whsec_test_agrobench"
	g, err := New(Config{
		SecretKey: "sk_test_x", WebhookSecret: secret,
		SuccessURL: "http://localhost/ok", CancelURL: "http://localhost/cancel",
	})
	require.NoError(t, err)

	subID := uuid.New()
	body := []byte(fmt.Sprintf(`{
		"id": "evt_test",
		"object": "event",
		"api_version": "%s",
		"type": "checkout.session.completed",
		"data": {
			"object": {
				"id": "cs_test_abc",
				"object": "checkout.session",
				"client_reference_id": "%s",
				"payment_status": "paid",
				"amount_total": 50000,
				"metadata": {"amount_usdc": "500.000000"}
			}
		}
	}`, libstripe.APIVersion, subID.String()))

	signed := libstripe.GenerateTestSignedPayload(&libstripe.UnsignedPayload{
		Payload: body, Secret: secret, Timestamp: time.Now(),
	})

	ev, err := g.ParseWebhook(context.Background(), signed.Payload, signed.Header)
	require.NoError(t, err)
	require.True(t, ev.Confirmed)
	require.Equal(t, "cs_test_abc", ev.ProviderRef)
	require.Equal(t, subID, ev.SubscriptionID)
	require.InDelta(t, 500.0, ev.Amount.Float(), 0.000001)
}

func TestParseWebhookIgnoresOtherEventsAndRejectsBadSig(t *testing.T) {
	secret := "whsec_test_agrobench"
	g, err := New(Config{
		SecretKey: "sk_test_x", WebhookSecret: secret,
		SuccessURL: "http://localhost/ok", CancelURL: "http://localhost/cancel",
	})
	require.NoError(t, err)

	body, _ := json.Marshal(map[string]any{
		"id": "evt_other", "object": "event",
		"api_version": libstripe.APIVersion,
		"type":        "customer.created",
		"data":        map[string]any{"object": map[string]any{"id": "cus_x"}},
	})
	signed := libstripe.GenerateTestSignedPayload(&libstripe.UnsignedPayload{
		Payload: body, Secret: secret, Timestamp: time.Now(),
	})
	ev, err := g.ParseWebhook(context.Background(), signed.Payload, signed.Header)
	require.NoError(t, err)
	require.False(t, ev.Confirmed)
	require.Equal(t, port.PaymentEvent{}, ev)

	_, err = g.ParseWebhook(context.Background(), body, "t=1,v1=deadbeef")
	require.Error(t, err)
}
