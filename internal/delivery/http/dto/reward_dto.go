package dto

import (
	"strings"
	"time"

	domainreward "crm-core/domain/reward"
	usecasereward "crm-core/usecase/reward"
)

// RewardCategoryResponse is one category.
type RewardCategoryResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	IsActive  bool   `json:"isActive"`
}

// ToRewardCategoryResponse maps a category.
func ToRewardCategoryResponse(c domainreward.Category) RewardCategoryResponse {
	return RewardCategoryResponse{ID: c.ID, Name: c.Name, SortOrder: c.SortOrder, IsActive: c.IsActive}
}

// RewardCategoryRequest creates or updates a category.
type RewardCategoryRequest struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	IsActive  *bool  `json:"isActive"`
}

// RewardResponse is a reward with its ERP stock (plan §5.2–5.3).
type RewardResponse struct {
	ID               uint       `json:"id"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	ImageURL         string     `json:"imageUrl"`
	CategoryID       *uint      `json:"categoryId"`
	Type             string     `json:"type"`
	ERPSKUID         *uint      `json:"erpSkuId"`
	SKUCode          string     `json:"skuCode"`
	SKUName          string     `json:"skuName"`
	ERPWarehouseID   uint       `json:"erpWarehouseId"`
	QtyPerRedemption int        `json:"qtyPerRedemption"`
	PointsCost       int64      `json:"pointsCost"`
	StockLimit       *int       `json:"stockLimit"`
	LimitPerMember   *int       `json:"limitPerMember"`
	MinTierID        *uint      `json:"minTierId"`
	AvailableFrom    *time.Time `json:"availableFrom"`
	AvailableTo      *time.Time `json:"availableTo"`
	Status           string     `json:"status"`
	RedeemedCount    int        `json:"redeemedCount"`
	SortOrder        int        `json:"sortOrder"`
	ERPAvailableQty  *int       `json:"erpAvailableQty"`
	StockSyncedAt    *time.Time `json:"stockSyncedAt"`
	StockStale       bool       `json:"stockStale"`
	RemainingQuota   int        `json:"remainingQuota"`
	UpdatedByEmail   string     `json:"updatedByEmail"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// ToRewardResponse maps a reward view.
func ToRewardResponse(v usecasereward.View) RewardResponse {
	r := v.Reward
	return RewardResponse{
		ID: r.ID, Name: r.Name, Description: r.Description, ImageURL: r.ImageURL, CategoryID: r.CategoryID,
		Type: r.Type, ERPSKUID: r.ERPSKUID, SKUCode: r.SKUCodeSnapshot, SKUName: r.SKUNameSnapshot,
		ERPWarehouseID: r.ERPWarehouseID, QtyPerRedemption: r.QtyPerRedemption, PointsCost: r.PointsCost,
		StockLimit: r.StockLimit, LimitPerMember: r.LimitPerMember, MinTierID: r.MinTierID,
		AvailableFrom: r.AvailableFrom, AvailableTo: r.AvailableTo, Status: r.Status,
		RedeemedCount: r.RedeemedCount, SortOrder: r.SortOrder,
		ERPAvailableQty: v.ERPAvailableQty, StockSyncedAt: v.StockSyncedAt, StockStale: v.StockStale,
		RemainingQuota: v.RemainingQuota, UpdatedByEmail: r.UpdatedByEmail, UpdatedAt: r.UpdatedAt,
	}
}

// RewardRequest is the body of POST /rewards and PATCH /rewards/:id.
type RewardRequest struct {
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	ImageURL         string     `json:"imageUrl"`
	CategoryID       *uint      `json:"categoryId"`
	Type             string     `json:"type"`
	ERPSKUID         *uint      `json:"erpSkuId"`
	SKUCode          string     `json:"skuCode"`
	SKUName          string     `json:"skuName"`
	ERPWarehouseID   uint       `json:"erpWarehouseId"`
	QtyPerRedemption int        `json:"qtyPerRedemption"`
	PointsCost       int64      `json:"pointsCost"`
	StockLimit       *int       `json:"stockLimit"`
	LimitPerMember   *int       `json:"limitPerMember"`
	MinTierID        *uint      `json:"minTierId"`
	AvailableFrom    *time.Time `json:"availableFrom"`
	AvailableTo      *time.Time `json:"availableTo"`
	SortOrder        int        `json:"sortOrder"`
}

// ToInput maps the request.
func (r RewardRequest) ToInput() usecasereward.Input {
	return usecasereward.Input{
		Name: r.Name, Description: r.Description, ImageURL: r.ImageURL, CategoryID: r.CategoryID, Type: r.Type,
		ERPSKUID: r.ERPSKUID, SKUCode: r.SKUCode, SKUName: r.SKUName, ERPWarehouseID: r.ERPWarehouseID,
		QtyPerRedemption: r.QtyPerRedemption, PointsCost: r.PointsCost, StockLimit: r.StockLimit,
		LimitPerMember: r.LimitPerMember, MinTierID: r.MinTierID, AvailableFrom: r.AvailableFrom,
		AvailableTo: r.AvailableTo, SortOrder: r.SortOrder,
	}
}

// RewardStatusRequest is the body of POST /rewards/:id/status.
type RewardStatusRequest struct {
	Status string `json:"status"`
}

// ERPSKUResponse is one ERP SKU offered in the reward form.
type ERPSKUResponse struct {
	ID           uint    `json:"id"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	ImageURL     string  `json:"imageUrl"`
	Category     string  `json:"category"`
	Price        float64 `json:"price"`
	AvailableQty int     `json:"availableQty"`
}

// RedemptionResponse is one redemption.
type RedemptionResponse struct {
	ID            uint       `json:"id"`
	RedemptionNo  string     `json:"redemptionNo"`
	MemberID      uint       `json:"memberId"`
	MemberCode    string     `json:"memberCode"`
	MemberName    string     `json:"memberName"`
	RewardID      uint       `json:"rewardId"`
	RewardName    string     `json:"rewardName"`
	SKUCode       string     `json:"skuCode"`
	Qty           int        `json:"qty"`
	Points        int64      `json:"points"`
	Status        string     `json:"status"`
	ShipName      string     `json:"shipName"`
	ShipPhone     string     `json:"shipPhone"`
	ShipAddress   string     `json:"shipAddress"`
	ShipProvince  string     `json:"shipProvince"`
	ShipPostcode  string     `json:"shipPostcode"`
	MemberNote    string     `json:"memberNote"`
	TrackingNo    string     `json:"trackingNo"`
	StockIssueRef string     `json:"stockIssueRef"`
	CancelReason  string     `json:"cancelReason"`
	CreatedAt     time.Time  `json:"createdAt"`
	ApprovedAt    *time.Time `json:"approvedAt"`
	PackedAt      *time.Time `json:"packedAt"`
	ShippedAt     *time.Time `json:"shippedAt"`
	DeliveredAt   *time.Time `json:"deliveredAt"`
	CancelledAt   *time.Time `json:"cancelledAt"`
	UpdatedBy     string     `json:"updatedByEmail"`
}

// ToRedemptionResponse maps a row (member fields may be empty for a bare redemption).
func ToRedemptionResponse(r domainreward.RedemptionRow) RedemptionResponse {
	rd := r.Redemption
	return RedemptionResponse{
		ID: rd.ID, RedemptionNo: rd.RedemptionNo, MemberID: rd.MemberID, MemberCode: r.MemberCode,
		MemberName: strings.TrimSpace(r.FirstName + " " + r.LastName), RewardID: rd.RewardID,
		RewardName: rd.RewardName, SKUCode: rd.SKUCode, Qty: rd.Qty, Points: rd.Points, Status: rd.Status,
		ShipName: rd.ShipName, ShipPhone: rd.ShipPhone, ShipAddress: rd.ShipAddress, ShipProvince: rd.ShipProvince,
		ShipPostcode: rd.ShipPostcode, MemberNote: rd.MemberNote, TrackingNo: rd.TrackingNo,
		StockIssueRef: rd.StockIssueRef, CancelReason: rd.CancelReason, CreatedAt: rd.CreatedAt,
		ApprovedAt: rd.ApprovedAt, PackedAt: rd.PackedAt, ShippedAt: rd.ShippedAt, DeliveredAt: rd.DeliveredAt,
		CancelledAt: rd.CancelledAt, UpdatedBy: rd.LastActorEmail,
	}
}

// ShipRequest is the body of POST /redemptions/:id/ship.
type ShipRequest struct {
	TrackingNo string `json:"trackingNo"`
}

// CancelRedemptionRequest is the body of POST /redemptions/:id/cancel.
type CancelRedemptionRequest struct {
	Reason string `json:"reason"`
}
