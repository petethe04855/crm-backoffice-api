package dto

import (
	"time"

	domainpromotion "crm-core/domain/promotion"
)

// PromotionAudienceResponse is the promotion's target audience. An empty tiers
// list with no newMemberDays means everyone.
type PromotionAudienceResponse struct {
	Tiers         []string `json:"tiers"`
	NewMemberDays *int     `json:"newMemberDays"`
}

// PromotionResponse is one promotion as the backoffice sees it.
//
// DisplayStatus is computed from the display window at request time (plan
// §3.2); the stored Status is the staff-set lifecycle state. Warnings are
// non-blocking notices produced by the save/publish action (e.g. six home
// banners at once) and are always an array so the UI never has to null-check.
type PromotionResponse struct {
	ID             string                    `json:"id"`
	Title          string                    `json:"title"`
	Description    string                    `json:"description"`
	ImageURL       string                    `json:"imageUrl"`
	Placement      string                    `json:"placement"`
	DisplayStartAt *time.Time                `json:"displayStartAt"`
	DisplayEndAt   *time.Time                `json:"displayEndAt"`
	CTAType        string                    `json:"ctaType"`
	CTATarget      string                    `json:"ctaTarget"`
	Audience       PromotionAudienceResponse `json:"audience"`
	Priority       int                       `json:"priority"`
	Status         string                    `json:"status"`
	DisplayStatus  string                    `json:"displayStatus"`
	Warnings       []string                  `json:"warnings"`
	CreatedBy      uint                      `json:"createdBy"`
	CreatedByEmail string                    `json:"createdByEmail"`
	UpdatedBy      uint                      `json:"updatedBy"`
	UpdatedByEmail string                    `json:"updatedByEmail"`
	CreatedAt      time.Time                 `json:"createdAt"`
	UpdatedAt      time.Time                 `json:"updatedAt"`
}

// ToPromotionResponse maps a promotion at the given clock. warnings may be nil.
func ToPromotionResponse(p domainpromotion.Promotion, now time.Time, warnings []string) PromotionResponse {
	tiers := p.Audience.Tiers
	if tiers == nil {
		tiers = []string{}
	}
	if warnings == nil {
		warnings = []string{}
	}
	return PromotionResponse{
		ID:             p.ID,
		Title:          p.Title,
		Description:    p.Description,
		ImageURL:       p.ImageURL,
		Placement:      p.Placement,
		DisplayStartAt: timeOrNil(p.DisplayStartAt),
		DisplayEndAt:   timeOrNil(p.DisplayEndAt),
		CTAType:        p.CTAType,
		CTATarget:      p.CTATarget,
		Audience:       PromotionAudienceResponse{Tiers: tiers, NewMemberDays: p.Audience.NewMemberDays},
		Priority:       p.Priority,
		Status:         p.Status,
		DisplayStatus:  domainpromotion.DisplayStatus(p, now),
		Warnings:       warnings,
		CreatedBy:      p.CreatedBy,
		CreatedByEmail: p.CreatedByEmail,
		UpdatedBy:      p.UpdatedBy,
		UpdatedByEmail: p.UpdatedByEmail,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

// timeOrNil renders a zero timestamp as null so the UI shows an empty field
// instead of 0001-01-01.
func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// PromotionAudienceRequest is the editable audience. An absent object or empty
// fields mean everyone.
type PromotionAudienceRequest struct {
	Tiers         []string `json:"tiers"`
	NewMemberDays *int     `json:"newMemberDays"`
}

// PromotionRequest is the create/update body (the form submits every field).
//
// DisplayStartAt/DisplayEndAt are pointers so null clears the window, which a
// draft may not have yet. PATCH replaces all editable fields, matching what the
// form sends.
type PromotionRequest struct {
	Title          string                   `json:"title"`
	Description    string                   `json:"description"`
	ImageURL       string                   `json:"imageUrl"`
	Placement      string                   `json:"placement"`
	DisplayStartAt *time.Time               `json:"displayStartAt"`
	DisplayEndAt   *time.Time               `json:"displayEndAt"`
	CTAType        string                   `json:"ctaType"`
	CTATarget      string                   `json:"ctaTarget"`
	Audience       PromotionAudienceRequest `json:"audience"`
	Priority       int                      `json:"priority"`
}
