package jwt

import (
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

const secret = "test-secret-at-least-16"

func sign(t *testing.T, method gojwt.SigningMethod, key []byte, c ERPClaims) string {
	t.Helper()
	s, err := gojwt.NewWithClaims(method, c).SignedString(key)
	require.NoError(t, err)
	return s
}

func claimsFor(userID uint, exp time.Time) ERPClaims {
	c := ERPClaims{UserID: userID, Email: "a@example.com", Role: "owner"}
	if !exp.IsZero() {
		c.ExpiresAt = gojwt.NewNumericDate(exp)
	}
	return c
}

func TestValidateERPToken(t *testing.T) {
	good := sign(t, gojwt.SigningMethodHS256, []byte(secret), claimsFor(7, time.Now().Add(time.Hour)))
	c, err := ValidateERPToken(good, secret)
	require.NoError(t, err)
	require.EqualValues(t, 7, c.UserID)
	require.Equal(t, "owner", c.Role)
}

func TestValidateERPTokenRejects(t *testing.T) {
	hour := time.Now().Add(time.Hour)
	cases := map[string]string{
		"wrong secret":    sign(t, gojwt.SigningMethodHS256, []byte("another-secret-xxxxx"), claimsFor(7, hour)),
		"expired":         sign(t, gojwt.SigningMethodHS256, []byte(secret), claimsFor(7, time.Now().Add(-time.Minute))),
		"no expiry":       sign(t, gojwt.SigningMethodHS256, []byte(secret), claimsFor(7, time.Time{})),
		"no user id":      sign(t, gojwt.SigningMethodHS256, []byte(secret), claimsFor(0, hour)),
		"hs512 not hs256": sign(t, gojwt.SigningMethodHS512, []byte(secret), claimsFor(7, hour)),
		"garbage":         "not-a-jwt",
	}
	for name, tok := range cases {
		_, err := ValidateERPToken(tok, secret)
		require.ErrorIs(t, err, ErrInvalidToken, name)
	}
}
