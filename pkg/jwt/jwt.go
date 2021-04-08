package jwt

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	jwtreq "github.com/dgrijalva/jwt-go/request"
	"github.com/rs/zerolog/log"
)

// Issuer SSO issuer.
const Issuer = "https://sso.traefik.io/"

// Audiences.
const (
	ServicesAudience = "https://services.pilot.traefik.io/"
	ClientsAudience  = "https://clients.pilot.traefik.io/"
)

const authorizationHeader = "Authorization"

// Check a check definition.
type Check struct {
	Header string
	Value  string
}

// Handler holds JWT.
type Handler struct {
	certValue string
	audience  string
	iss       string
	claims    map[string]Check

	extractor jwtreq.Extractor

	next http.Handler
}

// NewHandler creates a JWT handler.
func NewHandler(cert, audience, iss string, claims map[string]Check, next http.Handler) Handler {
	return Handler{
		certValue: cert,
		audience:  audience,
		iss:       iss,
		claims:    claims,
		extractor: newJWTExtractor(),
		next:      next,
	}
}

// ServeHTTP checks the token and call next.
func (h Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	claims, err := h.check(req)
	if err != nil {
		log.Error().Err(err).Msg("Impossible to check the JWT Token")
		jsonError(rw, http.StatusUnauthorized, "unauthorized")
		return
	}

	h.next.ServeHTTP(rw, req.WithContext(SetClaims(req.Context(), claims)))
}

func (h Handler) check(req *http.Request) (jwt.MapClaims, error) {
	parser := &jwt.Parser{UseJSONNumber: true}

	tok, err := jwtreq.ParseFromRequest(req, h.extractor, h.keyFunc, jwtreq.WithParser(parser))
	if err != nil {
		return nil, fmt.Errorf("unable to parse JWT: %w", err)
	}

	mapClaims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("unable to check claims type")
	}

	// Verify 'aud' claim
	checkAud := mapClaims.VerifyAudience(h.audience, false)
	if !checkAud {
		return nil, errors.New("invalid audience")
	}

	// Verify 'iss' claim
	checkIss := mapClaims.VerifyIssuer(h.iss, true)
	if !checkIss {
		return nil, errors.New("invalid issuer")
	}

	// Verify 'exp, iat, nbf'
	if valid := mapClaims.Valid(); valid != nil {
		return nil, fmt.Errorf("invalid token: %w", valid)
	}

	// Verify custom claims
	err = h.customValidation(req, mapClaims)
	if err != nil {
		return nil, err
	}

	req.Header.Del(authorizationHeader)

	return mapClaims, nil
}

func (h Handler) customValidation(req *http.Request, mapClaims jwt.MapClaims) error {
	for k, check := range h.claims {
		claimVal, ok := mapClaims[k]
		if !ok {
			return errors.New("claims: invalid JWT")
		}

		if check.Value != "" && check.Value != claimVal {
			return errors.New("claims: invalid JWT")
		}

		if check.Header != "" {
			value, ok := claimVal.(string)
			if !ok {
				return errors.New("claims: invalid JWT")
			}

			req.Header.Set(check.Header, value)
		}
	}

	return nil
}

// keyFunc returns the correct key to validate the given JWT's signature.
func (h Handler) keyFunc(_ *jwt.Token) (key interface{}, err error) {
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
