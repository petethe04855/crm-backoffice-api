package dto

import (
	"time"

	domainmember "crm-core/domain/member"
	domainpoints "crm-core/domain/points"
)

// PointLedgerResponse is one point movement in a member statement.
type PointLedgerResponse struct {
	ID             uint      `json:"id"`
	Type           string    `json:"type"`
	Points         int64     `json:"points"`
	BalanceAfter   int64     `json:"balanceAfter"`
	SourceType     string    `json:"sourceType"`
	SourceID       string    `json:"sourceId"`
	Note           string    `json:"note"`
	CreatedByEmail string    `json:"createdByEmail"`
	CreatedAt      time.Time `json:"createdAt"`
}

// ToPointLedgerResponse maps a ledger row.
func ToPointLedgerResponse(e domainpoints.PointLedger) PointLedgerResponse {
	return PointLedgerResponse{
		ID: e.ID, Type: e.Type, Points: e.Points, BalanceAfter: e.BalanceAfter,
		SourceType: e.SourceType, SourceID: e.SourceID, Note: e.Note,
		CreatedByEmail: e.CreatedByEmail, CreatedAt: e.CreatedAt,
	}
}

// AdjustPointsRequest is the body of POST /members/:id/points/adjust.
// Points is signed: positive grants, negative deducts.
type AdjustPointsRequest struct {
	Points int64  `json:"points"`
	Reason string `json:"reason"`
}

// MemberTagResponse is one backoffice tag.
type MemberTagResponse struct {
	Tag       string    `json:"tag"`
	CreatedAt time.Time `json:"createdAt"`
}

// ToMemberTagResponses maps tags.
func ToMemberTagResponses(tags []domainmember.MemberTag) []MemberTagResponse {
	out := make([]MemberTagResponse, 0, len(tags))
	for _, t := range tags {
		out = append(out, MemberTagResponse{Tag: t.Tag, CreatedAt: t.CreatedAt})
	}
	return out
}

// MemberNoteResponse is an internal note.
type MemberNoteResponse struct {
	ID             uint      `json:"id"`
	Body           string    `json:"body"`
	CreatedBy      uint      `json:"createdBy"`
	CreatedByEmail string    `json:"createdByEmail"`
	CreatedAt      time.Time `json:"createdAt"`
}

// ToMemberNoteResponse maps a note.
func ToMemberNoteResponse(n domainmember.MemberNote) MemberNoteResponse {
	return MemberNoteResponse{
		ID: n.ID, Body: n.Body, CreatedBy: n.CreatedBy,
		CreatedByEmail: n.CreatedByEmail, CreatedAt: n.CreatedAt,
	}
}

// UpdateMemberRequest is the body of PATCH /members/:id. Only status is
// mutable from the backoffice for now.
type UpdateMemberRequest struct {
	Status *string `json:"status"`
}

// AddTagRequest is the body of POST /members/:id/tags.
type AddTagRequest struct {
	Tag string `json:"tag"`
}

// CreateNoteRequest is the body of POST /members/:id/notes.
type CreateNoteRequest struct {
	Body string `json:"body"`
}
