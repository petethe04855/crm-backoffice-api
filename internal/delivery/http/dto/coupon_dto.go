package dto

import (
	"strings"
	"time"

	domaincoupon "crm-core/domain/coupon"
	usecasecoupon "crm-core/usecase/coupon"
)

// CouponResponse is a coupon template (plan §8.1).
type CouponResponse struct {
	ID             uint       `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	ImageURL       string     `json:"imageUrl"`
	Terms          string     `json:"terms"`
	BenefitType    string     `json:"benefitType"`
	BenefitValue   float64    `json:"benefitValue"`
	MaxDiscount    *float64   `json:"maxDiscount"`
	MinSpend       float64    `json:"minSpend"`
	PointsCost     int64      `json:"pointsCost"`
	ClaimStartAt   *time.Time `json:"claimStartAt"`
	ClaimEndAt     *time.Time `json:"claimEndAt"`
	ExpiryMode     string     `json:"expiryMode"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	ValidDays      int        `json:"validDays"`
	TotalQuota     int        `json:"totalQuota"`
	ClaimedCount   int        `json:"claimedCount"`
	UsedCount      int        `json:"usedCount"`
	RemainingQuota *int       `json:"remainingQuota"` // null = unlimited
	LimitPerMember int        `json:"limitPerMember"`
	MinTierID      *uint      `json:"minTierId"`
	Channel        string     `json:"channel"`
	CodeType       string     `json:"codeType"`
	SharedCode     string     `json:"sharedCode"`
	Status         string     `json:"status"`
	UpdatedByEmail string     `json:"updatedByEmail"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// ToCouponResponse maps a template.
func ToCouponResponse(c domaincoupon.Coupon) CouponResponse {
	out := CouponResponse{
		ID: c.ID, Name: c.Name, Description: c.Description, ImageURL: c.ImageURL, Terms: c.Terms,
		BenefitType: c.BenefitType, BenefitValue: c.BenefitValue, MaxDiscount: c.MaxDiscount, MinSpend: c.MinSpend,
		PointsCost: c.PointsCost, ClaimStartAt: c.ClaimStartAt, ClaimEndAt: c.ClaimEndAt,
		ExpiryMode: c.ExpiryMode, ExpiresAt: c.ExpiresAt, ValidDays: c.ValidDays,
		TotalQuota: c.TotalQuota, ClaimedCount: c.ClaimedCount, UsedCount: c.UsedCount,
		LimitPerMember: c.LimitPerMember, MinTierID: c.MinTierID, Channel: c.Channel,
		CodeType: c.CodeType, SharedCode: c.SharedCode, Status: c.Status,
		UpdatedByEmail: c.UpdatedByEmail, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	if c.TotalQuota > 0 {
		left := c.TotalQuota - c.ClaimedCount
		if left < 0 {
			left = 0
		}
		out.RemainingQuota = &left
	}
	return out
}

// CouponRequest is the body of POST /coupons and PATCH /coupons/:id.
type CouponRequest struct {
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	ImageURL       string     `json:"imageUrl"`
	Terms          string     `json:"terms"`
	BenefitType    string     `json:"benefitType"`
	BenefitValue   float64    `json:"benefitValue"`
	MaxDiscount    *float64   `json:"maxDiscount"`
	MinSpend       float64    `json:"minSpend"`
	PointsCost     int64      `json:"pointsCost"`
	ClaimStartAt   *time.Time `json:"claimStartAt"`
	ClaimEndAt     *time.Time `json:"claimEndAt"`
	ExpiryMode     string     `json:"expiryMode"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	ValidDays      int        `json:"validDays"`
	TotalQuota     int        `json:"totalQuota"`
	LimitPerMember int        `json:"limitPerMember"`
	MinTierID      *uint      `json:"minTierId"`
	Channel        string     `json:"channel"`
	CodeType       string     `json:"codeType"`
	SharedCode     string     `json:"sharedCode"`
}

// ToInput maps the request to the usecase input.
func (r CouponRequest) ToInput() usecasecoupon.Input {
	return usecasecoupon.Input{
		Name: r.Name, Description: r.Description, ImageURL: r.ImageURL, Terms: r.Terms,
		BenefitType: r.BenefitType, BenefitValue: r.BenefitValue, MaxDiscount: r.MaxDiscount, MinSpend: r.MinSpend,
		PointsCost: r.PointsCost, ClaimStartAt: r.ClaimStartAt, ClaimEndAt: r.ClaimEndAt,
		ExpiryMode: r.ExpiryMode, ExpiresAt: r.ExpiresAt, ValidDays: r.ValidDays,
		TotalQuota: r.TotalQuota, LimitPerMember: r.LimitPerMember, MinTierID: r.MinTierID,
		Channel: r.Channel, CodeType: r.CodeType, SharedCode: r.SharedCode,
	}
}

// MemberCouponResponse is one coupon a member holds.
type MemberCouponResponse struct {
	ID           uint       `json:"id"`
	CouponID     uint       `json:"couponId"`
	CouponName   string     `json:"couponName"`
	BenefitType  string     `json:"benefitType"`
	MemberID     uint       `json:"memberId"`
	MemberCode   string     `json:"memberCode"`
	MemberName   string     `json:"memberName"`
	Email        string     `json:"email"`
	Code         string     `json:"code"`
	SharedCode   string     `json:"sharedCode"`
	Status       string     `json:"status"`
	PointsSpent  int64      `json:"pointsSpent"`
	ClaimedAt    time.Time  `json:"claimedAt"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	UsedAt       *time.Time `json:"usedAt"`
	UsedRef      string     `json:"usedRef"`
	UsedByEmail  string     `json:"usedByEmail"`
	CancelledAt  *time.Time `json:"cancelledAt"`
	CancelReason string     `json:"cancelReason"`
}

// ToHolderResponse maps a holder row.
func ToHolderResponse(h domaincoupon.Holder) MemberCouponResponse {
	return MemberCouponResponse{
		ID: h.ID, CouponID: h.CouponID, CouponName: h.CouponName, BenefitType: h.BenefitType,
		MemberID: h.MemberID, MemberCode: h.MemberCode, MemberName: strings.TrimSpace(h.FirstName + " " + h.LastName),
		Email: h.Email, Code: h.Code, SharedCode: h.SharedCode, Status: h.Status, PointsSpent: h.PointsSpent,
		ClaimedAt: h.ClaimedAt, ExpiresAt: h.ExpiresAt, UsedAt: h.UsedAt, UsedRef: h.UsedRef,
		UsedByEmail: h.UsedByEmail, CancelledAt: h.CancelledAt, CancelReason: h.CancelReason,
	}
}

// ToMemberCouponResponse maps a bare member coupon (no joined names).
func ToMemberCouponResponse(mc domaincoupon.MemberCoupon) MemberCouponResponse {
	return ToHolderResponse(domaincoupon.Holder{MemberCoupon: mc})
}

// CodeViewResponse is what the store screen shows for a code.
type CodeViewResponse struct {
	Code      string     `json:"code"`
	Status    string     `json:"status"`
	Usable    bool       `json:"usable"`
	Reason    string     `json:"reason"`
	ClaimedAt time.Time  `json:"claimedAt"`
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt"`
	UsedRef   string     `json:"usedRef"`
	Coupon    struct {
		ID           uint     `json:"id"`
		Name         string   `json:"name"`
		BenefitType  string   `json:"benefitType"`
		BenefitValue float64  `json:"benefitValue"`
		MaxDiscount  *float64 `json:"maxDiscount"`
		MinSpend     float64  `json:"minSpend"`
		Channel      string   `json:"channel"`
		Terms        string   `json:"terms"`
	} `json:"coupon"`
	Member struct {
		ID         uint   `json:"id"`
		MemberCode string `json:"memberCode"`
		Name       string `json:"name"`
	} `json:"member"`
}

// ToCodeViewResponse maps a lookup / use result.
func ToCodeViewResponse(v *usecasecoupon.CodeView) CodeViewResponse {
	var out CodeViewResponse
	out.Code, out.Status, out.Usable, out.Reason = v.Holder.Code, v.Holder.Status, v.Usable, v.Reason
	out.ClaimedAt, out.ExpiresAt, out.UsedAt, out.UsedRef = v.Holder.ClaimedAt, v.Holder.ExpiresAt, v.Holder.UsedAt, v.Holder.UsedRef
	out.Coupon.ID, out.Coupon.Name, out.Coupon.BenefitType = v.Coupon.ID, v.Coupon.Name, v.Coupon.BenefitType
	out.Coupon.BenefitValue, out.Coupon.MaxDiscount, out.Coupon.MinSpend = v.Coupon.BenefitValue, v.Coupon.MaxDiscount, v.Coupon.MinSpend
	out.Coupon.Channel, out.Coupon.Terms = v.Coupon.Channel, v.Coupon.Terms
	out.Member.ID, out.Member.MemberCode = v.Member.ID, v.Member.MemberCode
	out.Member.Name = strings.TrimSpace(v.Member.FirstName + " " + v.Member.LastName)
	return out
}

// UseCouponRequest is the body of POST /coupons/codes/:code/use.
type UseCouponRequest struct {
	UsedRef string `json:"usedRef"`
}

// CancelCouponRequest is the body of POST /member-coupons/:id/cancel.
type CancelCouponRequest struct {
	Reason string `json:"reason"`
}

// GrantCouponRequest is the body of POST /members/:id/coupons.
type GrantCouponRequest struct {
	CouponID uint `json:"couponId"`
}
