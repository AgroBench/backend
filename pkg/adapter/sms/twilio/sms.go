// Package twilio é a implementação REAL de port.SmsSender (NÃO ativa no pitch).
// Usa a Messages API por HTTP direto (sem SDK): POST /2010-04-01/Accounts/{sid}/Messages.json
// com basic auth (account_sid:auth_token).
//
// Status: semi-pronto. Segue a API documentada; não foi executado com credenciais reais
// dentro do escopo do MVP.
package twilio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	AccountSID string
	AuthToken  string
	From       string // número Twilio em E.164 ou Messaging Service SID
	Timeout    time.Duration
}

type Sender struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) (*Sender, error) {
	if cfg.AccountSID == "" || cfg.AuthToken == "" || cfg.From == "" {
		return nil, errors.New("sms twilio: account_sid, auth_token e from são obrigatórios")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Sender{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}, nil
}

func (s *Sender) Send(ctx context.Context, phoneE164, message string) error {
	form := url.Values{"To": {phoneE164}, "Body": {message}}
	if strings.HasPrefix(s.cfg.From, "MG") {
		form.Set("MessagingServiceSid", s.cfg.From)
	} else {
		form.Set("From", s.cfg.From)
	}

	endpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", s.cfg.AccountSID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(s.cfg.AccountSID, s.cfg.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("sms twilio: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("sms twilio: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
