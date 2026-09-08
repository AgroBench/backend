// Package crypto concentra as primitivas criptográficas do protocolo:
//   - sealed box NaCl (x25519 + XSalsa20-Poly1305) para o payload do reveal;
//   - ed25519 para a assinatura do veredito do enclave;
//   - HMAC-SHA256 com pepper para CPF/CAR/CNPJ (índice irreversível);
//   - argon2id para senha.
//
// Compatível com tweetnacl / libsodium (crypto_box_seal) no app do produtor.
package crypto

import (
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/nacl/box"
)

const KeySize = 32

var ErrDecrypt = errors.New("sealed box: falha ao decifrar (chave errada ou payload corrompido)")

// GenerateBoxKeyPair gera um par x25519 para o ambiente de validação.
func GenerateBoxKeyPair() (public, private *[KeySize]byte, err error) {
	return box.GenerateKey(rand.Reader)
}

// Seal cifra message para recipientPub com uma chave efêmera (anônima).
// Saída = ephemeral_pub(32) || box. É o que o app faz antes do reveal.
func Seal(message []byte, recipientPub *[KeySize]byte) ([]byte, error) {
	return box.SealAnonymous(nil, message, recipientPub, rand.Reader)
}

// Open decifra um sealed box com o par do destinatário. Só o enclave chama isto.
func Open(ciphertext []byte, pub, priv *[KeySize]byte) ([]byte, error) {
	plain, ok := box.OpenAnonymous(nil, ciphertext, pub, priv)
	if !ok {
		return nil, ErrDecrypt
	}
	return plain, nil
}
