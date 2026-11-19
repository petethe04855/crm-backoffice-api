// Package dto defines request/response contracts. Domain entities are never
// serialized directly.
package dto

import (
	"time"

	domainloyalty "crm-core/domain/loyalty"
	domainmember "crm-core/domain/member"
	usecasemember "crm-core/usecase/member"
)

// TierResponse is a membership tier.
type TierResponse struct {
	ID             uint    `json:"id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	MinSpend       float64 `json:"minSpend"`
	EarnMultiplier float64 `json:"earnMultiplier"`
	SortOrder      int     `json:"sortOrder"`
	IsActive       bool    `json:"isActive"`
}

// ToTierResponse maps a tier.
func ToTierResponse(t domainloyalty.Tier) TierResponse {
	return TierResponse{
		ID: t.ID, Code: t.Code, Name: t.Name, MinSpend: t.MinSpend,
		EarnMultiplier: t.EarnMultiplier, SortOrder: t.SortOrder, IsActive: t.IsActive,
	}
}

// MemberListItem is a row in the member list.
type MemberListItem struct {
	ID            uint       `json:"id"`
	MemberCode    string     `json:"memberCode"`
	FirstName     string     `json:"firstName"`
	LastName      string     `json:"lastName"`
	Email         string     `json:"email"`
	Phone         *string    `json:"phone"`
	TierCode      string     `json:"tierCode"`
	TierName      string     `json:"tierName"`
	PointBalance  int64      `json:"pointBalance"`
	LifetimeSpend float64    `json:"lifetimeSpend"`
	Status        string     `json:"status"`
	LastActiveAt  *time.Time `json:"lastActiveAt"`
	RegisteredAt  time.Time  `json:"registeredAt"`
}

// ToMemberListItem maps a usecase list item.
func ToMemberListItem(i usecasemember.ListItem) MemberListItem {
	m := i.Member
	return MemberListItem{
		ID: m.ID, MemberCode: m.MemberCode, FirstName: m.FirstName, LastName: m.LastName,
		Email: m.Email, Phone: m.PhoneE164, TierCode: i.TierCode, TierName: i.TierName,
		PointBalance: m.PointBalance, LifetimeSpend: m.LifetimeSpend, Status: m.Status,
		LastActiveAt: m.LastActiveAt, RegisteredAt: m.CreatedAt,
	}
}

// LineAccountResponse is the member's linked LINE account.
type LineAccountResponse struct {
	LineUserID  string    `json:"lineUserId"`
	DisplayName string    `json:"displayName"`
	PictureURL  string    `json:"pictureUrl"`
	IsFriend    bool      `json:"isFriend"`
	LinkedAt    time.Time `json:"linkedAt"`
}

// MemberDetailResponse is the member detail view.
type MemberDetailResponse struct {
	ID              uint                 `json:"id"`
	MemberCode      string               `json:"memberCode"`
	FirstName       string               `json:"firstName"`
	LastName        string               `json:"lastName"`
	Email           string               `json:"email"`
	EmailVerifiedAt *time.Time           `json:"emailVerifiedAt"`
	Phone           *string              `json:"phone"`
	BirthDate       *string              `json:"birthDate"` // YYYY-MM-DD
	Gender          string               `json:"gender"`
	Tier            *TierResponse        `json:"tier"`
	PointBalance    int64                `json:"pointBalance"`
	LifetimeSpend   float64              `json:"lifetimeSpend"`
	ERPCustomerID   *uint                `json:"erpCustomerId"`
	Status          string               `json:"status"`
	Line            *LineAccountResponse `json:"line"`
	Tags            []MemberTagResponse  `json:"tags"`
	LastActiveAt    *time.Time           `json:"lastActiveAt"`
	RegisteredAt    time.Time            `json:"registeredAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
}

// ToMemberDetail maps a usecase detail.
func ToMemberDetail(d *usecasemember.Detail) MemberDetailResponse {
	m := d.Member
	out := MemberDetailResponse{
		ID: m.ID, MemberCode: m.MemberCode, FirstName: m.FirstName, LastName: m.LastName,
		Email: m.Email, EmailVerifiedAt: m.EmailVerifiedAt, Phone: m.PhoneE164,
		Gender: m.Gender, PointBalance: m.PointBalance, LifetimeSpend: m.LifetimeSpend,
		ERPCustomerID: m.CustomerID, Status: m.Status, RegisteredAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		LastActiveAt: m.LastActiveAt, Tags: ToMemberTagResponses(d.Tags),
	}
	if m.BirthDate != nil {
		s := m.BirthDate.Format("2006-01-02")
		out.BirthDate = &s
	}
	if d.Tier != nil {
		t := ToTierResponse(*d.Tier)
		out.Tier = &t
	}
	if d.Line != nil {
		out.Line = toLineAccount(*d.Line)
	}
	return out
}

func toLineAccount(a domainmember.MemberLineAccount) *LineAccountResponse {
	return &LineAccountResponse{
		LineUserID: a.LineUserID, DisplayName: a.DisplayName, PictureURL: a.PictureURL,
		IsFriend: a.IsFriend, LinkedAt: a.LinkedAt,
	}
}
