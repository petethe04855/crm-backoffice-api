package erp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrNotConfigured means ERP_API_TOKEN is empty, so the service-token calls
// to /integrations/crm cannot be made.
var ErrNotConfigured = errors.New("erp: CRM integration token is not configured")

// HasServiceToken reports whether ERP_API_TOKEN is set.
func (c *Client) HasServiceToken() bool { return c != nil && c.serviceToken != "" }

// MaxStockSKUs mirrors the ERP limit per /integrations/crm/stock request.
const MaxStockSKUs = 200

// SKU is one ERP SKU offered as a reward.
type SKU struct {
	ID           uint    `json:"id"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	ImageURL     string  `json:"imageUrl"`
	Category     string  `json:"category"`
	Price        float64 `json:"price"`
	AvailableQty int     `json:"availableQty"`
}

// StockLevel is the available quantity of one SKU in one warehouse.
type StockLevel struct {
	SKUID        uint      `json:"skuId"`
	WarehouseID  uint      `json:"warehouseId"`
	AvailableQty int       `json:"availableQty"`
	AsOf         time.Time `json:"asOf"`
}

// GoodsIssueItem is one SKU quantity to issue.
type GoodsIssueItem struct {
	SKUID uint `json:"skuId"`
	Qty   int  `json:"qty"`
}

// GoodsIssueRequest asks the ERP to deduct stock for a reward shipment. Use the
// redemption number as IdempotencyKey: a retry returns the first giRef.
type GoodsIssueRequest struct {
	IdempotencyKey string           `json:"idempotencyKey"`
	WarehouseID    uint             `json:"warehouseId,omitempty"`
	Items          []GoodsIssueItem `json:"items"`
	Reference      string           `json:"reference"`
	Note           string           `json:"note"`
}

// GoodsIssueResult is the ERP answer; Replayed is true when the key was used before.
type GoodsIssueResult struct {
	GIRef    string `json:"giRef"`
	Status   string `json:"status"`
	Replayed bool   `json:"replayed"`
}

// Shortage is one SKU the ERP could not issue.
type Shortage struct {
	SKUID     uint   `json:"skuId"`
	SKUCode   string `json:"skuCode"`
	Requested int    `json:"requested"`
	Available int    `json:"available"`
}

// InsufficientStockError is the ERP's 409: nothing was deducted.
type InsufficientStockError struct {
	Shortages []Shortage
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("erp: insufficient stock for %d SKU(s)", len(e.Shortages))
}

// RequestError is a 4xx the ERP explained (validation, SKU not found).
type RequestError struct {
	Status  int
	Code    string
	Message string
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("erp: %d %s: %s", e.Status, e.Code, e.Message)
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Data json.RawMessage `json:"data"`
}

type listEnvelope[T any] struct {
	Data []T `json:"data"`
	Meta struct {
		Total int64 `json:"total"`
	} `json:"meta"`
}

// integration sends a service-token request to /integrations/crm and decodes a
// 2xx body into out.
func (c *Client) integration(ctx context.Context, method, path string, body any, out any) error {
	if c.serviceToken == "" {
		return ErrNotConfigured
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/integrations/crm"+path, reader)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.serviceToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("%w: read: %v", ErrUnavailable, err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return ErrUnauthorized
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusServiceUnavailable:
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	case resp.StatusCode >= 400:
		var e errorEnvelope
		_ = json.Unmarshal(raw, &e)
		if resp.StatusCode == http.StatusConflict && e.Error.Code == "INSUFFICIENT_STOCK" {
			var detail struct {
				Shortages []Shortage `json:"shortages"`
			}
			_ = json.Unmarshal(e.Data, &detail)
			return &InsufficientStockError{Shortages: detail.Shortages}
		}
		return &RequestError{Status: resp.StatusCode, Code: e.Error.Code, Message: e.Error.Message}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: decode: %v", ErrUnavailable, err)
	}
	return nil
}

// SearchSKUs lists active ERP SKUs for the reward form.
func (c *Client) SearchSKUs(ctx context.Context, search string, warehouseID uint, page, limit int) ([]SKU, int64, error) {
	q := url.Values{}
	if s := strings.TrimSpace(search); s != "" {
		q.Set("search", s)
	}
	if warehouseID > 0 {
		q.Set("warehouseId", strconv.FormatUint(uint64(warehouseID), 10))
	}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := "/skus"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var body listEnvelope[SKU]
	if err := c.integration(ctx, http.MethodGet, path, nil, &body); err != nil {
		return nil, 0, err
	}
	return body.Data, body.Meta.Total, nil
}

// Stock reads available quantities, splitting into requests of MaxStockSKUs.
func (c *Client) Stock(ctx context.Context, skuIDs []uint, warehouseID uint) ([]StockLevel, error) {
	var out []StockLevel
	for start := 0; start < len(skuIDs); start += MaxStockSKUs {
		end := start + MaxStockSKUs
		if end > len(skuIDs) {
			end = len(skuIDs)
		}
		ids := make([]string, 0, end-start)
		for _, id := range skuIDs[start:end] {
			ids = append(ids, strconv.FormatUint(uint64(id), 10))
		}
		q := url.Values{"skuIds": {strings.Join(ids, ",")}}
		if warehouseID > 0 {
			q.Set("warehouseId", strconv.FormatUint(uint64(warehouseID), 10))
		}
		var body envelope[[]StockLevel]
		if err := c.integration(ctx, http.MethodGet, "/stock?"+q.Encode(), nil, &body); err != nil {
			return nil, err
		}
		out = append(out, body.Data...)
	}
	return out, nil
}

// CreateGoodsIssue deducts stock for a reward shipment. Safe to retry with the
// same IdempotencyKey after a timeout: the ERP never deducts twice.
func (c *Client) CreateGoodsIssue(ctx context.Context, in GoodsIssueRequest) (*GoodsIssueResult, error) {
	var body envelope[GoodsIssueResult]
	if err := c.integration(ctx, http.MethodPost, "/goods-issues", in, &body); err != nil {
		return nil, err
	}
	if body.Data.GIRef == "" {
		return nil, fmt.Errorf("%w: goods issue without giRef", ErrUnavailable)
	}
	return &body.Data, nil
}
