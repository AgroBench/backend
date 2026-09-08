// Package registry monta o conjunto de adapters externos a partir da config
// (`adapters.<port>` = mock | <real>). É o ÚNICO lugar que decide mock × real.
// Cada escolha é logada no boot para não haver dúvida sobre o que está ativo.
package registry

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"

	chainMock "github.com/AgroBench/backend/pkg/adapter/chain/mock"
	chainSolana "github.com/AgroBench/backend/pkg/adapter/chain/solana"
	enclaveMock "github.com/AgroBench/backend/pkg/adapter/enclave/mock"
	enclaveNitro "github.com/AgroBench/backend/pkg/adapter/enclave/nitro"
	paymentMock "github.com/AgroBench/backend/pkg/adapter/payment/mock"
	paymentStripe "github.com/AgroBench/backend/pkg/adapter/payment/stripe"
	refdataConab "github.com/AgroBench/backend/pkg/adapter/refdata/conab"
	refdataMock "github.com/AgroBench/backend/pkg/adapter/refdata/mock"
	sicarHTTP "github.com/AgroBench/backend/pkg/adapter/sicar/http"
	sicarMock "github.com/AgroBench/backend/pkg/adapter/sicar/mock"
	smsMock "github.com/AgroBench/backend/pkg/adapter/sms/mock"
	smsTwilio "github.com/AgroBench/backend/pkg/adapter/sms/twilio"
	"github.com/AgroBench/backend/pkg/port"
)

// Adapters é injetado nos DIs dos módulos.
type Adapters struct {
	Chain   port.ChainClient
	Enclave port.Enclave
	Sicar   port.SicarClient
	RefData port.ReferenceDataClient
	SMS     port.SmsSender
	Payment port.PaymentGateway

	// Acesso ao tipo concreto do mock quando ativo (seeds, endpoints de demo). nil se real.
	ChainMock *chainMock.Chain
	SMSMock   *smsMock.Sender
}

func Build(db *sqlx.DB) (*Adapters, error) {
	a := &Adapters{}
	var err error

	if a.Chain, a.ChainMock, err = buildChain(db); err != nil {
		return nil, err
	}
	if a.Enclave, err = buildEnclave(); err != nil {
		return nil, err
	}
	if a.Sicar, err = buildSicar(db); err != nil {
		return nil, err
	}
	if a.RefData, err = buildRefData(db); err != nil {
		return nil, err
	}
	if a.SMS, a.SMSMock, err = buildSMS(); err != nil {
		return nil, err
	}
	if a.Payment, err = buildPayment(); err != nil {
		return nil, err
	}
	return a, nil
}

func choice(name string) string {
	impl := viper.GetString("adapters." + name)
	if impl == "" {
		impl = "mock"
	}
	slog.Info("adapter selecionado", "port", name, "impl", impl, "mock", impl == "mock")
	return impl
}

func unknown(name, impl string) error {
	return fmt.Errorf("adapters.%s: implementação desconhecida %q", name, impl)
}

func seconds(key string) time.Duration {
	n := viper.GetInt(key)
	if n <= 0 {
		return 0
	}
	return time.Duration(n) * time.Second
}

func buildChain(db *sqlx.DB) (port.ChainClient, *chainMock.Chain, error) {
	switch impl := choice("chain"); impl {
	case "mock":
		m := chainMock.New(db, chainMock.Config{
			Treasury: port.Account(viper.GetString("chain.mock.treasury")),
			Pool:     port.Account(viper.GetString("chain.mock.pool")),
		})
		return m, m, nil
	case "solana":
		c, err := chainSolana.New(chainSolana.Config{
			RPCURL:             viper.GetString("chain.solana.rpc_url"),
			TreasuryPrivateKey: viper.GetString("chain.solana.treasury_private_key"),
			USDCMint:           viper.GetString("chain.solana.usdc_mint"),
			PoolPubkey:         viper.GetString("chain.solana.pool_pubkey"),
			ConfirmTimeout:     seconds("chain.solana.confirm_timeout_seconds"),
		})
		return c, nil, err
	default:
		return nil, nil, unknown("chain", impl)
	}
}

func buildEnclave() (port.Enclave, error) {
	switch impl := choice("enclave"); impl {
	case "mock":
		return enclaveMock.New(enclaveMock.Config{
			BoxPrivateKeyHex:      viper.GetString("enclave.mock.box_private_key"),
			SigningPrivateSeedHex: viper.GetString("enclave.mock.signing_private_seed"),
		})
	case "nitro":
		rootPEM, err := os.ReadFile(viper.GetString("enclave.nitro.root_cert_path"))
		if err != nil {
			return nil, fmt.Errorf("enclave.nitro.root_cert_path: %w", err)
		}
		pcrs := map[int]string{}
		for k, v := range viper.GetStringMapString("enclave.nitro.expected_pcrs") {
			idx, err := strconv.Atoi(k)
			if err != nil {
				return nil, fmt.Errorf("enclave.nitro.expected_pcrs: índice inválido %q", k)
			}
			if v != "" {
				pcrs[idx] = v
			}
		}
		return enclaveNitro.New(enclaveNitro.Config{
			CID:          uint32(viper.GetInt("enclave.nitro.cid")),
			Port:         uint32(viper.GetInt("enclave.nitro.port")),
			RootCertPEM:  rootPEM,
			ExpectedPCRs: pcrs,
			Timeout:      seconds("enclave.nitro.timeout_seconds"),
		})
	default:
		return nil, unknown("enclave", impl)
	}
}

func buildSicar(db *sqlx.DB) (port.SicarClient, error) {
	switch impl := choice("sicar"); impl {
	case "mock":
		return sicarMock.New(db), nil
	case "http":
		return sicarHTTP.New(sicarHTTP.Config{
			BaseURL:      viper.GetString("sicar.http.base_url"),
			APIKey:       viper.GetString("sicar.http.api_key"),
			PathTemplate: viper.GetString("sicar.http.path_template"),
			Timeout:      seconds("sicar.http.timeout_seconds"),
		})
	default:
		return nil, unknown("sicar", impl)
	}
}

func buildRefData(db *sqlx.DB) (port.ReferenceDataClient, error) {
	switch impl := choice("reference_data"); impl {
	case "mock":
		return refdataMock.New(db), nil
	case "conab":
		return refdataConab.New(refdataConab.Config{
			BaseURL:      viper.GetString("reference_data.conab.base_url"),
			PathTemplate: viper.GetString("reference_data.conab.path_template"),
			TolerancePct: viper.GetFloat64("reference_data.conab.tolerance_pct"),
			Timeout:      seconds("reference_data.conab.timeout_seconds"),
		})
	default:
		return nil, unknown("reference_data", impl)
	}
}

func buildSMS() (port.SmsSender, *smsMock.Sender, error) {
	switch impl := choice("sms"); impl {
	case "mock":
		m := smsMock.New()
		return m, m, nil
	case "twilio":
		s, err := smsTwilio.New(smsTwilio.Config{
			AccountSID: viper.GetString("sms.twilio.account_sid"),
			AuthToken:  viper.GetString("sms.twilio.auth_token"),
			From:       viper.GetString("sms.twilio.from"),
			Timeout:    seconds("sms.twilio.timeout_seconds"),
		})
		return s, nil, err
	default:
		return nil, nil, unknown("sms", impl)
	}
}

func buildPayment() (port.PaymentGateway, error) {
	switch impl := choice("payment"); impl {
	case "mock":
		return paymentMock.New(viper.GetString("payment.mock.base_url")), nil
	case "stripe":
		return paymentStripe.New(paymentStripe.Config{
			SecretKey:     viper.GetString("payment.stripe.secret_key"),
			WebhookSecret: viper.GetString("payment.stripe.webhook_secret"),
			SuccessURL:    viper.GetString("payment.stripe.success_url"),
			CancelURL:     viper.GetString("payment.stripe.cancel_url"),
			Currency:      viper.GetString("payment.stripe.currency"),
			FiatPerUSDC:   viper.GetFloat64("payment.stripe.fiat_per_usdc"),
		})
	default:
		return nil, unknown("payment", impl)
	}
}
