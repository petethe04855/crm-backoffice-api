package erp

import (
	"context"
	"errors"
	"fmt"

	domainreward "crm-core/domain/reward"
)

// RewardERP adapts the HTTP client to the reward module's ERP port, turning
// client errors into the domain errors crm-core understands.
type RewardERP struct {
	c *Client
}

// NewRewardERP builds the adapter.
func NewRewardERP(c *Client) *RewardERP { return &RewardERP{c: c} }

var _ domainreward.ERP = (*RewardERP)(nil)

func toRewardError(err error) error {
	var short *InsufficientStockError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &short):
		return fmt.Errorf("%w: %v", domainreward.ErrInsufficientERP, err)
	case errors.Is(err, ErrUnavailable), errors.Is(err, ErrNotConfigured), errors.Is(err, ErrUnauthorized):
		return fmt.Errorf("%w: %v", domainreward.ErrERPUnavailable, err)
	default:
		// 4xx the ERP explained (unknown SKU, validation): not retryable, but the
		// shipment still cannot proceed; surface it as unavailable with detail.
		return fmt.Errorf("%w: %v", domainreward.ErrERPUnavailable, err)
	}
}

// Stock reads available quantities for the stock cache.
func (a *RewardERP) Stock(ctx context.Context, skuIDs []uint, warehouseID uint) ([]domainreward.StockLevel, error) {
	levels, err := a.c.Stock(ctx, skuIDs, warehouseID)
	if err != nil {
		return nil, toRewardError(err)
	}
	out := make([]domainreward.StockLevel, 0, len(levels))
	for _, l := range levels {
		out = append(out, domainreward.StockLevel{SKUID: l.SKUID, WarehouseID: l.WarehouseID, AvailableQty: l.AvailableQty})
	}
	return out, nil
}

// IssueGoods deducts stock when a reward ships (idempotent per key).
func (a *RewardERP) IssueGoods(ctx context.Context, gi domainreward.GoodsIssue) (string, error) {
	res, err := a.c.CreateGoodsIssue(ctx, GoodsIssueRequest{
		IdempotencyKey: gi.IdempotencyKey, WarehouseID: gi.WarehouseID,
		Items: []GoodsIssueItem{{SKUID: gi.SKUID, Qty: gi.Qty}}, Reference: gi.Reference, Note: gi.Note,
	})
	if err != nil {
		return "", toRewardError(err)
	}
	return res.GIRef, nil
}
