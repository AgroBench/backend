package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/pkg/port"
)

func TestLookupActiveAndNotFound(t *testing.T) {
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("/api/v1/imoveis/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		car := r.URL.Path[len("/api/v1/imoveis/"):]
		if car == "MISSING" {
			w.WriteHeader(stdhttp.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"numero": car, "situacao": "AT", "uf": "RS",
			"municipio": map[string]string{"codigo_ibge": "430170"},
		})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	c, err := New(Config{BaseURL: ts.URL, APIKey: "test-key"})
	require.NoError(t, err)

	rec, err := c.Lookup(context.Background(), "RS-4312005-ABC")
	require.NoError(t, err)
	require.Equal(t, port.CARRecord{Exists: true, Active: true, UF: "RS", IBGECode: "430170"}, rec)

	rec, err = c.Lookup(context.Background(), "MISSING")
	require.NoError(t, err)
	require.False(t, rec.Exists)
}

func TestNewRequiresBaseURL(t *testing.T) {
	_, err := New(Config{})
	require.Error(t, err)
}

func TestLookupInactive(t *testing.T) {
	ts := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"situacao": "CA", "uf": "RS"})
	}))
	t.Cleanup(ts.Close)

	c, err := New(Config{BaseURL: ts.URL})
	require.NoError(t, err)
	rec, err := c.Lookup(context.Background(), "ANY")
	require.NoError(t, err)
	require.True(t, rec.Exists)
	require.False(t, rec.Active)
}
