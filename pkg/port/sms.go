package port

import "context"

// SmsSender envia o OTP do MFA e da recuperação de conta (README raiz §6.1).
//
// Implementações:
//   - pkg/adapter/sms/mock   → ATIVA NO PITCH. Loga a mensagem no slog e guarda em memória
//     (Outbox) para testes. Em dev o código também é lido via GET /admin/otp/{user_id}.
//   - pkg/adapter/sms/twilio → REAL. Twilio Messages API via HTTP (sem SDK).
//
// Seleção: config `adapters.sms` (mock | twilio).
type SmsSender interface {
	Send(ctx context.Context, phoneE164, message string) error
}
