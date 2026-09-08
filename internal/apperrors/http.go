package apperrors

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func WriteHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	status := http.StatusInternalServerError
	resp := errorResponse{Code: ErrInternal, Message: Catalog[ErrInternal].Message}

	var appErr *AppError
	if errors.As(err, &appErr) {
		if def, ok := Catalog[appErr.Code]; ok {
			status = def.HTTPStatus
			resp.Code = def.Code
			resp.Message = def.Message
		}
		resp.Detail = appErr.Detail

		log := slog.Warn
		if status >= 500 {
			log = slog.Error
		}
		log("app error",
			"code", appErr.Code, "op", appErr.Op, "layer", appErr.Layer,
			"err", appErr.Err, "path", r.URL.Path, "request_id", r.Header.Get("X-Request-Id"),
		)
	} else {
		slog.Error("unexpected error", "err", err, "path", r.URL.Path)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
