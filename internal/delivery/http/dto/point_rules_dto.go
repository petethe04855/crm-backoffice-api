package dto

import (
	"time"

	domainearnrule "crm-core/domain/earnrule"
)

// EarnRuleResponse is one earn rule (plan §4.1).
type EarnRuleResponse struct {
	ID             uint       `json:"id"`
	Type           string     `json:"type"`
	TierID         *uint      `json:"tierId"`
	Multiplier     float64    `json:"multiplier"`
	SKUIDs         []string   `json:"skuIds"`
	Trigger        string     `json:"trigger"`
	Points         int64      `json:"points"`
	StartAt        *time.Time `json:"startAt"`
	EndAt          *time.Time `json:"endAt"`
	Stackable      bool       `json:"stackable"`
	Priority       int        `json:"priority"`
	IsActive       bool       `json:"isActive"`
	UpdatedByEmail string     `json:"updatedByEmail"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// ToEarnRuleResponse maps a rule; skuIds is always an array.
func ToEarnRuleResponse(r domainearnrule.EarnRule) EarnRuleResponse {
	skus := r.SKUIDs
	if skus == nil {
		skus = []string{}
	}
	return EarnRuleResponse{
		ID: r.ID, Type: r.Type, TierID: r.TierID, Multiplier: r.Multiplier, SKUIDs: skus,
		Trigger: r.Trigger, Points: r.Points, StartAt: r.StartAt, EndAt: r.EndAt,
		Stackable: r.Stackable, Priority: r.Priority, IsActive: r.IsActive,
		UpdatedByEmail: r.UpdatedByEmail, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// EarnRuleRequest is the body of POST /earn-rules and PATCH /earn-rules/:id.
// PATCH replaces every editable field; an omitted isActive keeps the state.
type EarnRuleRequest struct {
	Type       string     `json:"type"`
	TierID     *uint      `json:"tierId"`
	Multiplier float64    `json:"multiplier"`
	SKUIDs     []string   `json:"skuIds"`
	Trigger    string     `json:"trigger"`
	Points     int64      `json:"points"`
	StartAt    *time.Time `json:"startAt"`
	EndAt      *time.Time `json:"endAt"`
	Stackable  bool       `json:"stackable"`
	Priority   int        `json:"priority"`
	IsActive   *bool      `json:"isActive"`
}

// SimulateRequest is the body of POST /point-rules/simulate. At defaults to now.
type SimulateRequest struct {
	TierID     uint       `json:"tierId"`
	SKUID      string     `json:"skuId"`
	BasePoints int64      `json:"basePoints"`
	At         *time.Time `json:"at"`
}
