// Package mock é a implementação de port.SmsSender ATIVA NO PITCH: não envia nada,
// loga a mensagem e guarda as últimas em memória para testes e para a demo.
package mock

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Message struct {
	Phone  string
	Body   string
	SentAt time.Time
}

type Sender struct {
	mu     sync.Mutex
	outbox []Message
}

const outboxLimit = 200

func New() *Sender { return &Sender{} }

func (s *Sender) Send(_ context.Context, phoneE164, message string) error {
	slog.Info("SMS (mock)", "to", phoneE164, "body", message)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outbox = append(s.outbox, Message{Phone: phoneE164, Body: message, SentAt: time.Now()})
	if len(s.outbox) > outboxLimit {
		s.outbox = s.outbox[len(s.outbox)-outboxLimit:]
	}
	return nil
}

// LastTo devolve a última mensagem enviada para o telefone (ou false).
func (s *Sender) LastTo(phoneE164 string) (Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.outbox) - 1; i >= 0; i-- {
		if s.outbox[i].Phone == phoneE164 {
			return s.outbox[i], true
		}
	}
	return Message{}, false
}
