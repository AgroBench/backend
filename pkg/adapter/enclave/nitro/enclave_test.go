package nitro

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/AgroBench/backend/internal/core/crypto"
	"github.com/AgroBench/backend/internal/core/domain"
	"github.com/AgroBench/backend/internal/core/payload"
	"github.com/AgroBench/backend/pkg/port"
)

func TestHostRoundTripViaTCP(t *testing.T) {
	srv, err := NewServer()
	require.NoError(t, err)

	ca := newTestCA(t)
	pcr0 := bytesRepeat(0x11, 48)
	userData := []byte(crypto.EncodeKey(srv.BoxPublicKey()) + "|" + srv.SigningPublicKeyHex())
	doc := ca.signDoc(t, userData, map[uint][]byte{0: pcr0})
	srv.Attest = func([]byte) ([]byte, error) { return doc, nil }

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.Handle(conn)
		}
	}()

	host, err := New(Config{
		RootCertPEM:  ca.rootPEM,
		ExpectedPCRs: map[int]string{0: pcrHex(pcr0)},
		Timeout:      5 * time.Second,
		Dial: func(context.Context) (net.Conn, error) {
			return net.Dial("tcp", ln.Addr().String())
		},
	})
	require.NoError(t, err)

	id, err := host.Identity(context.Background())
	require.NoError(t, err)
	require.Equal(t, "nitro", id.Provider)
	require.Equal(t, crypto.EncodeKey(srv.BoxPublicKey()), id.BoxPublicKey)
	require.Equal(t, srv.SigningPublicKeyHex(), id.SigningPublicKey)
	require.NotEmpty(t, id.Attestation)

	cycle, contrib := uuid.New(), uuid.New()
	plain, err := payload.Build(domain.LevelBasic, cycle, "0123456789abcdef0123456789abcdef",
		payload.Basic{CultureCode: "soybean", AreaHa: 50, TotalCostBRL: 250_000})
	require.NoError(t, err)
	cipher, err := crypto.Seal(plain, srv.BoxPublicKey())
	require.NoError(t, err)

	v, err := host.Validate(context.Background(), port.ValidateRequest{
		ContributionID: contrib, CycleID: cycle, Level: domain.LevelBasic,
		CommitHash: payload.Hash(plain), Ciphertext: cipher,
		References: []port.ReferenceRange{{Metric: payload.MetricTotalCostHa, Min: 3000, Max: 8000}},
	})
	require.NoError(t, err)
	require.True(t, v.Accepted, "checks: %+v", v.Checks)
	require.Equal(t, id.SigningPublicKey, v.SignerPubKey)
}

func TestIdentityRejectsMismatchedUserData(t *testing.T) {
	srv, err := NewServer()
	require.NoError(t, err)
	ca := newTestCA(t)
	bad := ca.signDoc(t, []byte("chaves-erradas"), nil)
	srv.Attest = func([]byte) ([]byte, error) { return bad, nil }

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.Handle(conn)
		}
	}()

	host, err := New(Config{
		RootCertPEM: ca.rootPEM,
		Dial: func(context.Context) (net.Conn, error) {
			return net.Dial("tcp", ln.Addr().String())
		},
	})
	require.NoError(t, err)

	_, err = host.Identity(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "user_data")
}

func TestNewRequiresRootCert(t *testing.T) {
	_, err := New(Config{})
	require.Error(t, err)
}
