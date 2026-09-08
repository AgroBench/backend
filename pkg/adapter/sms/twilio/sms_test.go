package twilio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSendPostsFormWithBasicAuth(t *testing.T) {
	var gotTo, gotFrom, gotBody, user, pass string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/2010-04-01/Accounts/ACtest/Messages.json", r.URL.Path)
		user, pass, _ = r.BasicAuth()
		require.NoError(t, r.ParseForm())
		gotTo = r.Form.Get("To")
		gotFrom = r.Form.Get("From")
		gotBody = r.Form.Get("Body")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sid":"SM123"}`))
	}))
	t.Cleanup(ts.Close)

	s, err := New(Config{
		AccountSID: "ACtest", AuthToken: "secret", From: "+5500000000000", APIBase: ts.URL,
	})
	require.NoError(t, err)
	require.NoError(t, s.Send(context.Background(), "+5551999999999", "seu codigo e 123456"))
	require.Equal(t, "ACtest", user)
	require.Equal(t, "secret", pass)
	require.Equal(t, "+5551999999999", gotTo)
	require.Equal(t, "+5500000000000", gotFrom)
	require.Equal(t, "seu codigo e 123456", gotBody)
}

func TestSendMessagingServiceSID(t *testing.T) {
	var gotMS string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		gotMS = r.Form.Get("MessagingServiceSid")
		require.Empty(t, r.Form.Get("From"))
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(ts.Close)

	s, err := New(Config{AccountSID: "ACtest", AuthToken: "secret", From: "MG123", APIBase: ts.URL})
	require.NoError(t, err)
	require.NoError(t, s.Send(context.Background(), "+5551999999999", "oi"))
	require.Equal(t, "MG123", gotMS)
}

func TestSendErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid"}`))
	}))
	t.Cleanup(ts.Close)

	s, err := New(Config{AccountSID: "ACtest", AuthToken: "bad", From: "+55000", APIBase: ts.URL})
	require.NoError(t, err)
	err = s.Send(context.Background(), "+5551", "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestNewRequiresFields(t *testing.T) {
	_, err := New(Config{})
	require.Error(t, err)
}
