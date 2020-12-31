package jwt

import (
	"context"

	"github.com/dgrijalva/jwt-go"
)

type claimKeyType string

const claimKey claimKeyType = "claimKey"

// List of claim keys.
const (
	UserIDClaim         = "https://clients.pilot.traefik.io/uuid"
	OrganizationIDClaim = "https://clients.pilot.traefik.io/organizationId"
)

// SetClaims puts the claims into the context.
func SetClaims(ctx context.Context, claims jwt.MapClaims) context.Context {
	return context.WithValue(ctx, claimKey, claims)
}
