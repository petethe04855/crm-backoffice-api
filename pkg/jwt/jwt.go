// Package jwt verifies staff tokens issued by erp-api-v2. backoffice-api never
// issues staff tokens itself; staff log in through the ERP.
package jwt

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

// ERPClaims mirrors the claims erp-api-v2 puts in its tokens.
type ERPClaims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// ErrInvalidToken is returned for any token that fails verification.
var ErrInvalidToken = errors.New("invalid token")

// ValidateERPToken verifies an HS256 token signed with the ERP secret.
// Expiry is enforced by the jwt library; tokens without exp are rejected.
func ValidateERPToken(tokenString, secret string) (*ERPClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ERPClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	}, jwt.WithExpirationRequired(), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*ERPClaims)
	if !ok || !token.Valid || claims.UserID == 0 {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
