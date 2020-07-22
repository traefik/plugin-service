package functions

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/dgrijalva/jwt-go"
	jwtreq "github.com/dgrijalva/jwt-go/request"
)

const authorizationHeader = "Authorization"

type check struct {
	header string
	value  string
}

// JWTHandler holds JWT.
type JWTHandler struct {
	certValue string
	audience  string
	iss       string
	claims    map[string]check

	extractor jwtreq.Extractor

	next http.Handler
}

func newJWTHandler(cert, audience, iss string, claims map[string]check, next http.Handler) JWTHandler {
	return JWTHandler{
		certValue: cert,
		audience:  audience,
		iss:       iss,
		claims:    claims,
		extractor: newJWTExtractor(),
		next:      next,
	}
}

// ServeHTTP checks the token and call next.
func (h JWTHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if err := h.check(req); err != nil {
		log.Error().Msg(err.Error())
		jsonError(rw, http.StatusUnauthorized, "unauthorized")
		return
	}

	h.next.ServeHTTP(rw, req)
}

func (h JWTHandler) check(req *http.Request) error {
	parser := &jwt.Parser{UseJSONNumber: true}

	tok, err := jwtreq.ParseFromRequest(req, h.extractor, h.keyFunc, jwtreq.WithParser(parser))
	if err != nil {
		return fmt.Errorf("unable to parse JWT: %w", err)
	}

	mapClaims := tok.Claims.(jwt.MapClaims)

	// Verify 'aud' claim
	checkAud := mapClaims.VerifyAudience(h.audience, false)
	if !checkAud {
		return errors.New("invalid audience")
	}

	// Verify 'iss' claim
	checkIss := mapClaims.VerifyIssuer(h.iss, true)
	if !checkIss {
		return errors.New("invalid issuer")
	}

	// Verify 'exp, iat, nbf'
	valid := mapClaims.Valid()
	if valid != nil {
		return fmt.Errorf("invalid token: %w", valid)
	}

	// Verify custom claims
	err = h.customValidation(req, mapClaims)
	if err != nil {
		return err
	}

	req.Header.Del(authorizationHeader)

	return nil
}

func (h JWTHandler) customValidation(req *http.Request, mapClaims jwt.MapClaims) error {
	for k, check := range h.claims {
		claimVal, ok := mapClaims[k]
		if !ok {
			return errors.New("claims: invalid JWT")
		}

		if check.value != "" && check.value != claimVal {
			return errors.New("claims: invalid JWT")
		}

		if check.header != "" {
			req.Header.Set(check.header, claimVal.(string))
		}
	}

	return nil
}

// keyFunc returns the correct key to validate the given JWT's signature.
func (h JWTHandler) keyFunc(_ *jwt.Token) (key interface{}, err error) {
	cert, err := base64.StdEncoding.DecodeString(h.certValue)
	if err != nil {
		return nil, err
	}

	result, err := jwt.ParseRSAPublicKeyFromPEM(cert)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// newJWTExtractor Extracts a JWT from an HTTP request.
// It first looks in the "Authorization" header then in a query parameter "jwt".
// It returns an error if no JWT was found.
func newJWTExtractor() jwtreq.Extractor {
	return &jwtreq.PostExtractionFilter{
		Extractor: jwtreq.HeaderExtractor{authorizationHeader},
		Filter: func(raw string) (s string, err error) {
			if !strings.HasPrefix(raw, "Bearer ") {
				return "", errors.New("no JWT found in request")
			}

			rawJWT := strings.TrimPrefix(raw, "Bearer ")
			if rawJWT == "" {
				return "", errors.New("no JWT found in request")
			}

			return rawJWT, nil
		},
	}
}
