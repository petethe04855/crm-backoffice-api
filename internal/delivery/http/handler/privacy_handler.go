package handler

import (
	"errors"
	"log"
	"strconv"
	"time"

	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	domainprivacy "crm-core/domain/privacy"
	usecaseprivacy "crm-core/usecase/privacy"

	"github.com/gofiber/fiber/v2"
)

// PrivacyHandler serves the PDPA deletion request queue (L7).
type PrivacyHandler struct {
	privacy usecaseprivacy.Usecase
}

// NewPrivacyHandler builds the handler.
func NewPrivacyHandler(p usecaseprivacy.Usecase) *PrivacyHandler { return &PrivacyHandler{privacy: p} }

type dataRequestDTO struct {
	ID          uint       `json:"id"`
	MemberID    uint       `json:"memberId"`
	MemberCode  string     `json:"memberCode"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Reason      string     `json:"reason"`
	StaffNote   string     `json:"staffNote"`
	ProcessedBy uint       `json:"processedBy"`
	ProcessedAt *time.Time `json:"processedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

func (h *PrivacyHandler) writeError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domainprivacy.ErrOpenRedemptions):
		return response.Error(c, fiber.StatusConflict, "OPEN_REDEMPTIONS", "The member has redemptions not shipped or cancelled yet; finish or cancel them first")
	case errors.Is(err, domainprivacy.ErrRequestClosed):
		return response.Error(c, fiber.StatusConflict, "REQUEST_CLOSED", "The request was already processed")
	}
	return writeError(c, err)
}

// List serves GET /data-requests?status=&page=&limit=.
func (h *PrivacyHandler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	rows, total, err := h.privacy.Requests(c.UserContext(), domainprivacy.RequestQuery{Status: c.Query("status"), Page: page, Limit: limit})
	if err != nil {
		return h.writeError(c, err)
	}
	out := make([]dataRequestDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, dataRequestDTO{
			ID: r.ID, MemberID: r.MemberID, MemberCode: r.MemberCode, Name: r.FirstName + " " + r.LastName, Email: r.Email,
			Type: r.Type, Status: r.Status, Reason: r.Reason, StaffNote: r.StaffNote, ProcessedBy: r.ProcessedBy,
			ProcessedAt: r.ProcessedAt, CreatedAt: r.CreatedAt,
		})
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return response.List(c, out, page, limit, total)
}

type noteRequest struct {
	Note string `json:"note"`
}

// Complete serves POST /data-requests/:id/complete {note}: anonymizes the member.
func (h *PrivacyHandler) Complete(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return h.writeError(c, err)
	}
	var req noteRequest
	_ = c.BodyParser(&req)
	actor, _ := middleware.ActorFrom(c)
	if err := h.privacy.CompleteDeletion(c.UserContext(), actor, id, req.Note); err != nil {
		return h.writeError(c, err)
	}
	log.Printf("[AUDIT] PDPA deletion completed request=%d by erp_user_id=%d", id, actor.ERPUserID)
	return response.OK(c, fiber.Map{"id": id}, "Member anonymized")
}

// Reject serves POST /data-requests/:id/reject {note}.
func (h *PrivacyHandler) Reject(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return h.writeError(c, err)
	}
	var req noteRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	actor, _ := middleware.ActorFrom(c)
	if err := h.privacy.RejectDeletion(c.UserContext(), actor, id, req.Note); err != nil {
		return h.writeError(c, err)
	}
	log.Printf("[AUDIT] PDPA deletion rejected request=%d by erp_user_id=%d", id, actor.ERPUserID)
	return response.OK(c, fiber.Map{"id": id}, "Request rejected")
}
