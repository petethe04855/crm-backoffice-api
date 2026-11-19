package handler

import (
	"backoffice-api/internal/delivery/http/dto"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	domainpoints "crm-core/domain/points"
	usecasepoints "crm-core/usecase/points"

	"github.com/gofiber/fiber/v2"
)

// PointsHandler serves the member point ledger and manual adjustments.
type PointsHandler struct {
	points usecasepoints.Usecase
}

// NewPointsHandler builds the points handler.
func NewPointsHandler(points usecasepoints.Usecase) *PointsHandler {
	return &PointsHandler{points: points}
}

// Ledger serves GET /members/:id/points.
func (h *PointsHandler) Ledger(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	page, err := parseOptionalInt(c.Query("page"), "page")
	if err != nil {
		return writeError(c, err)
	}
	limit, err := parseOptionalInt(c.Query("limit"), "limit")
	if err != nil {
		return writeError(c, err)
	}
	q := domainpoints.Query{Type: c.Query("type"), Page: page, Limit: limit}
	entries, total, err := h.points.Ledger(c.UserContext(), id, q)
	if err != nil {
		return writeError(c, err)
	}
	out := make([]dto.PointLedgerResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, dto.ToPointLedgerResponse(e))
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	return response.List(c, out, page, limit, total)
}

// Adjust serves POST /members/:id/points/adjust.
func (h *PointsHandler) Adjust(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.AdjustPointsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	entry, err := h.points.Adjust(c.UserContext(), actor, id, usecasepoints.AdjustRequest{
		Points: req.Points,
		Reason: req.Reason,
	})
	if err != nil {
		return writeError(c, err)
	}
	return response.Created(c, dto.ToPointLedgerResponse(*entry), "Points adjusted")
}
