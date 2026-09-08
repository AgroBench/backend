package identity_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/testutil"
	"github.com/AgroBench/backend/pkg/adapter/registry"
	"github.com/AgroBench/backend/pkg/adapter/rest"
)

func TestOnboardingIntegration(t *testing.T) {
	db := testutil.Postgres(t)
	ctx := context.Background()
	viper.Set("auth.pepper", "test-pepper-32-bytes-minimum-ok")
	viper.Set("auth.jwt_secret", "test-secret-at-least-32-bytes-long!!")
	viper.Set("auth.access_ttl_minutes", 15)
	viper.Set("auth.refresh_ttl_days", 30)
	viper.Set("auth.otp_ttl_minutes", 5)
	viper.Set("auth.otp_max_attempts", 5)
	viper.Set("adapters.chain", "mock")
	viper.Set("adapters.enclave", "mock")
	viper.Set("adapters.sicar", "mock")
	viper.Set("adapters.reference_data", "mock")
	viper.Set("adapters.sms", "mock")
	viper.Set("adapters.payment", "mock")

	adapters, err := registry.Build(db)
	require.NoError(t, err)
	h := rest.NewRouter(db, adapters, nil)

	body := map[string]string{
		"email": "prod@test.local", "phone": "+5554999000001",
		"cpf": "52998224725", "password": "senha-segura",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(b)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var reg struct {
		MFAToken string `json:"mfa_token"`
		UserID   string `json:"user_id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &reg))

	otpReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/otp/"+reg.UserID, nil)
	otpRec := httptest.NewRecorder()
	h.ServeHTTP(otpRec, otpReq)
	require.Equal(t, http.StatusOK, otpRec.Code, otpRec.Body.String())
	var otp struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(otpRec.Body.Bytes(), &otp))
	require.Regexp(t, regexp.MustCompile(`^\d{6}$`), otp.Code)

	mfaBody, _ := json.Marshal(map[string]string{"mfa_token": reg.MFAToken, "code": otp.Code})
	mfaReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(mfaBody))
	mfaReq.Header.Set("Content-Type", "application/json")
	mfaRec := httptest.NewRecorder()
	h.ServeHTTP(mfaRec, mfaReq)
	require.Equal(t, http.StatusOK, mfaRec.Code, mfaRec.Body.String())

	var toks struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.Unmarshal(mfaRec.Body.Bytes(), &toks))
	require.NotEmpty(t, toks.AccessToken)

	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+toks.AccessToken)
	meRec := httptest.NewRecorder()
	h.ServeHTTP(meRec, meReq)
	require.Equal(t, http.StatusOK, meRec.Code, meRec.Body.String())
}
