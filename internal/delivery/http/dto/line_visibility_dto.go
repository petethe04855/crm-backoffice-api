package dto

import (
	"time"

	"crm-core/domain/linevis"
)

// clock is the time used for LINE OA visibility on coupon / reward responses
// (promotions pass their own now, shared with DisplayStatus).
var clock = time.Now

// LineVisibilityResponse tells staff whether an item is shown to members in
// the LINE OA right now and why not (plan: backoffice "แสดงใน LINE OA").
type LineVisibilityResponse struct {
	Visible bool       `json:"visible"`
	Reason  string     `json:"reason"`
	From    *time.Time `json:"from,omitempty"`
	Until   *time.Time `json:"until,omitempty"`
}

// ToLineVisibility maps a visibility result.
func ToLineVisibility(v linevis.Visibility) LineVisibilityResponse {
	return LineVisibilityResponse{Visible: v.Visible, Reason: v.Reason, From: v.From, Until: v.Until}
}
