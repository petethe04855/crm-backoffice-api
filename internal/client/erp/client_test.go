package erp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/me" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Header.Get("Authorization") {
		case "Bearer good":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":7,"email":"a@example.com","role":"sales","isActive":true,"mustResetPassword":false}}`))
		case "Bearer revoked":
			w.WriteHeader(http.StatusUnauthorized)
		case "Bearer broken":
			_, _ = w.Write([]byte(`{"success":true,"data":{}}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c := New(srv.URL+"/api/v1/", "", time.Second) // trailing slash is trimmed
	ctx := context.Background()

	u, err := c.Me(ctx, "good")
	require.NoError(t, err)
	require.EqualValues(t, 7, u.ID)
	require.Equal(t, "sales", u.Role)
	require.True(t, u.IsActive)

	_, err = c.Me(ctx, "revoked")
	require.ErrorIs(t, err, ErrUnauthorized)

	_, err = c.Me(ctx, "broken")
	require.ErrorIs(t, err, ErrUnavailable)

	_, err = c.Me(ctx, "server-error")
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestMeUnreachable(t *testing.T) {
	c := New("http://127.0.0.1:1/api/v1", "", 200*time.Millisecond)
	_, err := c.Me(context.Background(), "x")
	require.ErrorIs(t, err, ErrUnavailable)
}
