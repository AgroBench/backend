package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/argon2"
)

// SHA256Hex é o hash do commit: sha256 do JSON canônico do payload.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// HMACHex calcula HMAC-SHA256(pepper, value) em hex. Usado para CPF, CAR e CNPJ:
// determinístico (permite unique index e lookup) e irreversível sem o pepper.
// value é normalizado para só dígitos/maiúsculas antes do hash.
func HMACHex(pepper, value string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(NormalizeIdentifier(value)))
	return hex.EncodeToString(mac.Sum(nil))
}

var nonAlnum = regexp.MustCompile(`[^A-Z0-9]`)

// NormalizeIdentifier remove pontuação e espaços e coloca em maiúsculas
// ("123.456.789-01" → "12345678901"; "RS-4312005-ABC…" → "RS4312005ABC…").
func NormalizeIdentifier(v string) string {
	return nonAlnum.ReplaceAllString(strings.ToUpper(v), "")
}

// --- argon2id (senha) ---

const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

// HashPassword devolve o hash no formato PHC: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := randRead(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

var ErrPasswordMismatch = errors.New("senha incorreta")

// VerifyPassword compara password com um hash PHC gerado por HashPassword.
func VerifyPassword(password, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return errors.New("hash de senha em formato inválido")
	}
	var mem, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &t, &p); err != nil {
		return fmt.Errorf("parâmetros do hash inválidos: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return err
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return err
	}
	got := argon2.IDKey([]byte(password), salt, t, mem, p, uint32(len(expected)))
	if subtle.ConstantTimeCompare(got, expected) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}
