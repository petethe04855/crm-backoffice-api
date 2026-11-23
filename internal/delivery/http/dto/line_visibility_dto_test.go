package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	domaincoupon "crm-core/domain/coupon"
	domainpromotion "crm-core/domain/promotion"
	domainreward "crm-core/domain/reward"
	usecasereward "crm-core/usecase/reward"
)

func TestResponsesCarryLineVisibility(t *testing.T) {
	now := time.Date(2026, 11, 22, 20, 0, 0, 0, time.UTC)
	clock = func() time.Time { return now }
	t.Cleanup(func() { clock = time.Now })

	p := ToPromotionResponse(domainpromotion.Promotion{
		Status: domainpromotion.StatusPublished, DisplayStartAt: now.Add(time.Hour), DisplayEndAt: now.Add(2 * time.Hour),
	}, now, nil)
	if p.LineVisibility.Visible || p.LineVisibility.Reason != "not_started" || p.LineVisibility.From == nil {
		t.Errorf("promotion visibility = %+v, want hidden not_started with from", p.LineVisibility)
	}

	c := ToCouponResponse(domaincoupon.Coupon{Status: domaincoupon.StatusDraft})
	if c.LineVisibility.Visible || c.LineVisibility.Reason != "draft" {
		t.Errorf("coupon visibility = %+v, want hidden draft", c.LineVisibility)
	}

	r := ToRewardResponse(usecasereward.View{Reward: domainreward.Reward{Status: domainreward.StatusActive}, RemainingQuota: 3})
	if !r.LineVisibility.Visible || r.LineVisibility.Reason != "live" {
		t.Errorf("reward visibility = %+v, want visible live", r.LineVisibility)
	}

	raw, _ := json.Marshal(c)
	if !strings.Contains(string(raw), `"lineVisibility":{"visible":false,"reason":"draft"}`) {
		t.Errorf("coupon JSON = %s, want a lineVisibility object", raw)
	}
}
