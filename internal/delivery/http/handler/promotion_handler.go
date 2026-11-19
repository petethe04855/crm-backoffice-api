package handler

import (
	"time"

	"backoffice-api/internal/delivery/http/dto"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	domainpromotion "crm-core/domain/promotion"
	usecasepromotion "crm-core/usecase/promotion"

	"github.com/gofiber/fiber/v2"
)

// PromotionHandler serves the marketing promotion screens and the
// draft/publish/archive lifecycle.
type PromotionHandler struct {
	promotions usecasepromotion.Usecase
	// now drives DisplayStatus so every row in one response agrees.
	now func() time.Time
}

// NewPromotionHandler builds the handler.
func NewPromotionHandler(promotions usecasepromotion.Usecase) *PromotionHandler {
	return &PromotionHandler{promotions: promotions, now: time.Now}
}

// List serves GET /promotions.
//
//	?month=2026-11  display window overlapping that Thai month
//	?status=        draft | published | archived (stored status)
//	?placement=     home_banner | promotion_list | popup
//	?search=        title contains
func (h *PromotionHandler) List(c *fiber.Ctx) error {
	page, err := parseOptionalInt(c.Query("page"), "page")
	if err != nil {
		return writeError(c, err)
	}
	limit, err := parseOptionalInt(c.Query("limit"), "limit")
	if err != nil {
		return writeError(c, err)
	}
	q := usecasepromotion.ListQuery{
		Month:     c.Query("month"),
		Status:    c.Query("status"),
		Placement: c.Query("placement"),
		Search:    c.Query("search"),
		Page:      page,
		Limit:     limit,
	}

	items, total, err := h.promotions.List(c.UserContext(), q)
	if err != nil {
		return writeError(c, err)
	}
	normalized := usecasepromotion.NormalizeListQuery(q)
	now := h.now()
	out := make([]dto.PromotionResponse, 0, len(items))
	for _, p := range items {
		out = append(out, dto.ToPromotionResponse(p, now, nil))
	}
	return response.List(c, out, normalized.Page, normalized.Limit, total)
}

// Get serves GET /promotions/:id.
func (h *PromotionHandler) Get(c *fiber.Ctx) error {
	p, err := h.promotions.Get(c.UserContext(), c.Params("id"))
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToPromotionResponse(*p, h.now(), nil))
}

// Create serves POST /promotions. New promotions always start as drafts.
func (h *PromotionHandler) Create(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.PromotionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	p, warnings, err := h.promotions.Create(c.UserContext(), actor, toPromotionInput(req))
	if err != nil {
		return writeError(c, err)
	}
	return response.Created(c, dto.ToPromotionResponse(*p, h.now(), warnings), "Promotion created")
}

// Update serves PATCH /promotions/:id. Publishing and archiving have their own
// endpoints; this only edits the fields.
func (h *PromotionHandler) Update(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.PromotionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	p, warnings, err := h.promotions.Update(c.UserContext(), actor, c.Params("id"), toPromotionInput(req))
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToPromotionResponse(*p, h.now(), warnings), "Promotion updated")
}

// Delete serves DELETE /promotions/:id (drafts only).
func (h *PromotionHandler) Delete(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	id := c.Params("id")
	if err := h.promotions.Delete(c.UserContext(), actor, id); err != nil {
		return writeError(c, err)
	}
	return response.OK(c, fiber.Map{"id": id}, "Promotion deleted")
}

// Publish serves POST /promotions/:id/publish. Non-blocking warnings are
// returned with the published row.
func (h *PromotionHandler) Publish(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	p, warnings, err := h.promotions.Publish(c.UserContext(), actor, c.Params("id"))
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToPromotionResponse(*p, h.now(), warnings), "Promotion published")
}

// Archive serves POST /promotions/:id/archive.
func (h *PromotionHandler) Archive(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	p, err := h.promotions.Archive(c.UserContext(), actor, c.Params("id"))
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToPromotionResponse(*p, h.now(), nil), "Promotion archived")
}

// Duplicate serves POST /promotions/:id/duplicate: a draft copy for next month.
func (h *PromotionHandler) Duplicate(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	p, err := h.promotions.Duplicate(c.UserContext(), actor, c.Params("id"))
	if err != nil {
		return writeError(c, err)
	}
	return response.Created(c, dto.ToPromotionResponse(*p, h.now(), nil), "Promotion duplicated")
}

func toPromotionInput(req dto.PromotionRequest) usecasepromotion.Input {
	in := usecasepromotion.Input{
		Title:          req.Title,
		Description:    req.Description,
		ImageURL:       req.ImageURL,
		Placement:      req.Placement,
		DisplayStartAt: req.DisplayStartAt,
		DisplayEndAt:   req.DisplayEndAt,
		CTAType:        req.CTAType,
		CTATarget:      req.CTATarget,
		Priority:       req.Priority,
		Audience: domainpromotion.Audience{
			Tiers:         req.Audience.Tiers,
			NewMemberDays: req.Audience.NewMemberDays,
		},
	}
	return in
}
