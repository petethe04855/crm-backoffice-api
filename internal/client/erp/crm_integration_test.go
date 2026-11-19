package erp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const svcToken = "crm-service-token-for-tests-0123456789"

func integrationServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+svcToken {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":"INVALID_TOKEN","message":"Invalid service token"}}`))
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestIntegrationNeedsAToken(t *testing.T) {
	c := New("http://127.0.0.1:1/api/v1", "", time.Second)
	_, _, err := c.SearchSKUs(context.Background(), "x", 0, 1, 20)
	require.ErrorIs(t, err, ErrNotConfigured)

	srv := integrationServer(t, func(w http.ResponseWriter, r *http.Request) {})
	_, _, err = New(srv.URL+"/api/v1", "wrong", time.Second).SearchSKUs(context.Background(), "x", 0, 1, 20)
	require.ErrorIs(t, err, ErrUnauthorized)
}

func TestSearchSKUs(t *testing.T) {
	srv := integrationServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/integrations/crm/skus", r.URL.Path)
		require.Equal(t, "ไก่", r.URL.Query().Get("search"))
		require.Equal(t, "2", r.URL.Query().Get("warehouseId"))
		_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1,"code":"CHK-01","name":"ขนมไก่","imageUrl":"https://img/1.jpg","availableQty":8}],"meta":{"page":1,"limit":20,"total":1}}`))
	})
	skus, total, err := New(srv.URL+"/api/v1", svcToken, time.Second).SearchSKUs(context.Background(), " ไก่ ", 2, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "CHK-01", skus[0].Code)
	require.Equal(t, 8, skus[0].AvailableQty)
}

func TestStockSplitsLargeRequests(t *testing.T) {
	var calls int32
	srv := integrationServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		ids := strings.Split(r.URL.Query().Get("skuIds"), ",")
		require.LessOrEqual(t, len(ids), MaxStockSKUs)
		out := make([]StockLevel, 0, len(ids))
		for range ids {
			out = append(out, StockLevel{SKUID: 1, WarehouseID: 1, AvailableQty: 3})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": out})
	})
	ids := make([]uint, 450)
	for i := range ids {
		ids[i] = uint(i + 1)
	}
	levels, err := New(srv.URL+"/api/v1", svcToken, time.Second).Stock(context.Background(), ids, 0)
	require.NoError(t, err)
	require.Len(t, levels, 450)
	require.EqualValues(t, 3, atomic.LoadInt32(&calls))
}

func TestCreateGoodsIssueOutcomes(t *testing.T) {
	var mode atomic.Value
	srv := integrationServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		var body GoodsIssueRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "RD2611-000001", body.IdempotencyKey)
		switch mode.Load() {
		case "short":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":"INSUFFICIENT_STOCK","message":"insufficient stock"},"data":{"shortages":[{"skuId":1,"skuCode":"CHK-01","requested":9,"available":8}]}}`))
		case "bad":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":"SKU_NOT_FOUND","message":"SKU 42 not found"}}`))
		case "down":
			w.WriteHeader(http.StatusBadGateway)
		case "slow":
			time.Sleep(300 * time.Millisecond)
		default:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true,"data":{"giRef":"CRMGI-000001","status":"issued","replayed":false}}`))
		}
	})
	c := New(srv.URL+"/api/v1", svcToken, 150*time.Millisecond)
	req := GoodsIssueRequest{IdempotencyKey: "RD2611-000001", Items: []GoodsIssueItem{{SKUID: 1, Qty: 1}}, Reference: "RD2611-000001"}
	ctx := context.Background()

	res, err := c.CreateGoodsIssue(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "CRMGI-000001", res.GIRef)

	mode.Store("short")
	_, err = c.CreateGoodsIssue(ctx, req)
	var short *InsufficientStockError
	require.True(t, errors.As(err, &short))
	require.Equal(t, 8, short.Shortages[0].Available)

	mode.Store("bad")
	_, err = c.CreateGoodsIssue(ctx, req)
	var reqErr *RequestError
	require.True(t, errors.As(err, &reqErr))
	require.Equal(t, "SKU_NOT_FOUND", reqErr.Code)

	mode.Store("down")
	_, err = c.CreateGoodsIssue(ctx, req)
	require.ErrorIs(t, err, ErrUnavailable)

	mode.Store("slow")
	_, err = c.CreateGoodsIssue(ctx, req)
	require.ErrorIs(t, err, ErrUnavailable, "a timeout is retryable thanks to the idempotency key")
}
