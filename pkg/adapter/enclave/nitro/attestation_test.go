package nitro

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
	"github.com/veraison/go-cose"
)

type testCA struct {
	rootPEM  []byte
	rootCert *x509.Certificate
	leafKey  *ecdsa.PrivateKey
	leafDER  []byte
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	rootKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-nitro-root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	require.NoError(t, err)
	rootCert, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)

	leafKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-nitro-leaf"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, rootCert, &leafKey.PublicKey, rootKey)
	require.NoError(t, err)

	return &testCA{
		rootPEM:  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}),
		rootCert: rootCert,
		leafKey:  leafKey,
		leafDER:  leafDER,
	}
}

func (ca *testCA) signDoc(t *testing.T, userData []byte, pcrs map[uint][]byte) []byte {
	t.Helper()
	doc := AttestationDocument{
		ModuleID:    "i-test-enclave",
		Digest:      "SHA384",
		Timestamp:   uint64(time.Now().UnixMilli()),
		PCRs:        pcrs,
		Certificate: ca.leafDER,
		UserData:    userData,
	}
	payload, err := cbor.Marshal(doc)
	require.NoError(t, err)

	msg := cose.NewSign1Message()
	msg.Payload = payload
	msg.Headers.Protected.SetAlgorithm(cose.AlgorithmES384)
	signer, err := cose.NewSigner(cose.AlgorithmES384, ca.leafKey)
	require.NoError(t, err)
	require.NoError(t, msg.Sign(rand.Reader, nil, signer))
	raw, err := msg.MarshalCBOR()
	require.NoError(t, err)
	return raw
}

func TestVerifyAttestationAcceptsValidDoc(t *testing.T) {
	ca := newTestCA(t)
	pcr0 := bytesRepeat(0xab, 48)
	userData := []byte("boxpub|signpub")
	raw := ca.signDoc(t, userData, map[uint][]byte{0: pcr0})

	doc, err := VerifyAttestation(raw, ca.rootPEM, map[int]string{0: pcrHex(pcr0)}, time.Now())
	require.NoError(t, err)
	require.Equal(t, userData, doc.UserData)
	require.Equal(t, "i-test-enclave", doc.ModuleID)
}

func TestVerifyAttestationRejectsBadPCRAndEmpty(t *testing.T) {
	ca := newTestCA(t)
	pcr0 := bytesRepeat(0xab, 48)
	raw := ca.signDoc(t, []byte("ud"), map[uint][]byte{0: pcr0})

	_, err := VerifyAttestation(raw, ca.rootPEM, map[int]string{0: pcrHex(bytesRepeat(0xcd, 48))}, time.Now())
	require.Error(t, err)
	require.Contains(t, err.Error(), "PCR0")

	_, err = VerifyAttestation(nil, ca.rootPEM, nil, time.Now())
	require.Error(t, err)
}

func TestParseRootPEM(t *testing.T) {
	ca := newTestCA(t)
	require.NoError(t, ParseRootPEM(ca.rootPEM))
	require.Error(t, ParseRootPEM([]byte("not-pem")))
}

func bytesRepeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
