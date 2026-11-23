package line

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	domainlinemsg "crm-core/domain/linemsg"

	"github.com/stretchr/testify/require"
)

func TestPushAndMenus(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		calls = append(calls, r.Method+" "+r.URL.Path+" "+string(body))
		switch r.URL.Path {
		case "/v2/bot/message/push":
			if string(body) == `{"messages":[{"type":"text","text":"x"}],"to":"Ublocked"}` {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message":"not a friend"}`))
				return
			}
		case "/v2/bot/user/U1/richmenu":
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := New("tok", "richmenu-member").WithBaseURLs(srv.URL, srv.URL)
	ctx := context.Background()
	msgs := json.RawMessage(`[{"type":"text","text":"x"}]`)

	require.NoError(t, c.Push(ctx, "U1", msgs))
	err := c.Push(ctx, "Ublocked", msgs)
	require.ErrorIs(t, err, domainlinemsg.ErrPermanent, "4xx is not retried")
	require.NoError(t, c.LinkMemberMenu(ctx, "U1"))
	require.NoError(t, c.UnlinkMenu(ctx, "U1"), "404 = already on the default menu")
	require.Contains(t, calls[2], "POST /v2/bot/user/U1/richmenu/richmenu-member")

	require.ErrorIs(t, New("tok", "").WithBaseURLs(srv.URL, srv.URL).LinkMemberMenu(ctx, "U1"), domainlinemsg.ErrPermanent)
}

func TestRetryableErrors(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
		err := New("tok", "m").WithBaseURLs(srv.URL, srv.URL).Push(context.Background(), "U1", json.RawMessage(`[]`))
		srv.Close()
		require.Error(t, err)
		require.False(t, errors.Is(err, domainlinemsg.ErrPermanent), "status %d is retried", status)
	}
}
