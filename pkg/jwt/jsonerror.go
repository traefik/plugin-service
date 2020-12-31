package jwt

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

type apiError struct {
	Message string `json:"error"`
}

func jsonError(rw http.ResponseWriter, code int, errMsg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("X-Content-Type-Options", "nosniff")
	rw.WriteHeader(code)

	msg := apiError{
		Message: errMsg,
	}

	content, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Str("func_error", errMsg).Msg("failed to process error")

		_, _ = fmt.Fprintln(rw, `{"error": "internal error"}`)
		return
	}

	_, _ = fmt.Fprintln(rw, string(content))
}
