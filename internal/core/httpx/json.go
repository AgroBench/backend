// Package httpx concentra helpers HTTP compartilhados por todos os handlers:
// decode/encode JSON, validação de input e middlewares.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/AgroBench/backend/internal/apperrors"
)

const maxBodyBytes = 2 << 20 // 2 MiB — payloads cifrados cabem folgado

// Decode lê o body JSON em dst, rejeita campos desconhecidos e valida as tags `validate`.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	const op = "httpx.Decode"

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &syntaxErr):
			return apperrors.Validation(op, err).WithLayer("handler").WithDetail(fmt.Sprintf("JSON malformado na posição %d", syntaxErr.Offset))
		case errors.As(err, &typeErr):
			return apperrors.Validation(op, err).WithLayer("handler").WithDetail(fmt.Sprintf("campo %q com tipo inválido", typeErr.Field))
		case errors.Is(err, io.EOF):
			return apperrors.Validation(op, err).WithLayer("handler").WithDetail("body vazio")
		case errors.As(err, &maxErr):
			return apperrors.Validation(op, err).WithLayer("handler").WithDetail("body excede o limite")
		default:
			return apperrors.Validation(op, err).WithLayer("handler").WithDetail(err.Error())
		}
	}
	if dec.More() {
		return apperrors.Validation(op, errors.New("multiple json values")).WithLayer("handler").WithDetail("body deve conter um único objeto JSON")
	}

	return Validate(dst)
}

// JSON escreve v como JSON com o status informado.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// Error é atalho para apperrors.WriteHTTPError.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	apperrors.WriteHTTPError(w, r, err)
}
