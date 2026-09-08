package httpx

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/AgroBench/backend/internal/apperrors"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

// Validate roda as tags `validate` de um struct e converte a primeira falha em AppError
// com detail legível (ex.: "email: required").
func Validate(v any) error {
	const op = "httpx.Validate"
	err := validate.Struct(v)
	if err == nil {
		return nil
	}

	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		parts := make([]string, 0, len(verrs))
		for _, fe := range verrs {
			field := strings.ToLower(fe.Field())
			if fe.Param() != "" {
				parts = append(parts, fmt.Sprintf("%s: %s=%s", field, fe.Tag(), fe.Param()))
			} else {
				parts = append(parts, fmt.Sprintf("%s: %s", field, fe.Tag()))
			}
		}
		return apperrors.Validation(op, err).WithLayer("handler").WithDetail(strings.Join(parts, "; "))
	}
	return apperrors.Validation(op, err).WithLayer("handler")
}

// Validator expõe a instância para registrar validações customizadas (ex.: cpf, car).
func Validator() *validator.Validate { return validate }
