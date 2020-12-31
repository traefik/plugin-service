package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

type apiError struct {
	Message string `json:"error"`
}

// PanicHandler handles panics.
func PanicHandler(rw http.ResponseWriter, req *http.Request, err interface{}) {
	log.Error().Str("method", req.Method).Interface("url", req.URL).Interface("err", err).
		Msg("Panic error executing request")
	JSONError(rw, http.StatusInternalServerError, "panic")
}

// JSONInternalServerError handles an JSON InternalServerError.
func JSONInternalServerError(rw http.ResponseWriter) {
	JSONError(rw, http.StatusInternalServerError, "internal server error")
}

// JSONErrorf handles an JSON error.
func JSONErrorf(rw http.ResponseWriter, code int, errMsg string, args ...interface{}) {
	JSONError(rw, code, fmt.Sprintf(errMsg, args...))
}

// JSONError handles an JSON error.
func JSONError(rw http.ResponseWriter, code int, errMsg string) {
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
