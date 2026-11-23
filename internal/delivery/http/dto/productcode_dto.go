package dto

import (
	"time"

	domainproductcode "crm-core/domain/productcode"
)

// ProductCodeBatchRequest creates or edits a draft batch.
type ProductCodeBatchRequest struct {
	ERPSKUID          uint     `json:"erpSkuId"`
	SKUCode           string   `json:"skuCode"`
	SKUName           string   `json:"skuName"`
	SKUPrice          float64  `json:"skuPrice"`
	Quantity          int      `json:"quantity"`
	PointsPerCode     int64    `json:"pointsPerCode"`
	SpendValuePerCode *float64 `json:"spendValuePerCode"`
	LotNumber         string   `json:"lotNumber"`
	LotExpiryDate     string   `json:"lotExpiryDate"` // YYYY-MM-DD
	CodeExpiresAt     string   `json:"codeExpiresAt"` // YYYY-MM-DD (end of that day, Bangkok)
	IsTest            bool     `json:"isTest"`
	Note              string   `json:"note"`
}

// VoidBatchRequest voids a batch.
type VoidBatchRequest struct {
	Reason string `json:"reason"`
}

// ProductCodeStats counts codes by status.
type ProductCodeStats struct {
	Total  int64 `json:"total"`
	Active int64 `json:"active"`
	Used   int64 `json:"used"`
	Void   int64 `json:"void"`
}

// ProductCodeBatchResponse is a batch with its code counts.
type ProductCodeBatchResponse struct {
	ID                uint             `json:"id"`
	BatchNo           string           `json:"batchNo"`
	ERPSKUID          uint             `json:"erpSkuId"`
	SKUCode           string           `json:"skuCode"`
	SKUName           string           `json:"skuName"`
	SKUPrice          float64          `json:"skuPrice"`
	Quantity          int              `json:"quantity"`
	PointsPerCode     int64            `json:"pointsPerCode"`
	SpendValuePerCode float64          `json:"spendValuePerCode"`
	LotNumber         string           `json:"lotNumber"`
	LotExpiryDate     *time.Time       `json:"lotExpiryDate,omitempty"`
	CodeExpiresAt     *time.Time       `json:"codeExpiresAt,omitempty"`
	IsTest            bool             `json:"isTest"`
	Status            string           `json:"status"`
	Note              string           `json:"note"`
	ExportedAt        *time.Time       `json:"exportedAt,omitempty"`
	ActivatedAt       *time.Time       `json:"activatedAt,omitempty"`
	VoidedAt          *time.Time       `json:"voidedAt,omitempty"`
	VoidReason        string           `json:"voidReason,omitempty"`
	CreatedByEmail    string           `json:"createdByEmail"`
	CreatedAt         time.Time        `json:"createdAt"`
	Stats             ProductCodeStats `json:"stats"`
}

// ToProductCodeBatchResponse maps a batch.
func ToProductCodeBatchResponse(b domainproductcode.Batch, s domainproductcode.Stats) ProductCodeBatchResponse {
	return ProductCodeBatchResponse{
		ID: b.ID, BatchNo: b.BatchNo, ERPSKUID: b.SKUID, SKUCode: b.SKUCode, SKUName: b.SKUNameSnapshot,
		SKUPrice: b.SKUPriceSnapshot, Quantity: b.Quantity, PointsPerCode: b.PointsPerCode,
		SpendValuePerCode: b.SpendValuePerCode, LotNumber: b.LotNumber, LotExpiryDate: b.LotExpiryDate,
		CodeExpiresAt: b.CodeExpiresAt, IsTest: b.IsTest, Status: b.Status, Note: b.Note,
		ExportedAt: b.ExportedAt, ActivatedAt: b.ActivatedAt, VoidedAt: b.VoidedAt, VoidReason: b.VoidReason,
		CreatedByEmail: b.CreatedByEmail, CreatedAt: b.CreatedAt,
		Stats: ProductCodeStats{Total: s.Total, Active: s.Active, Used: s.Used, Void: s.Void},
	}
}
