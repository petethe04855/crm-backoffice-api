package handler

import (
	"fmt"
	"log"
	"time"

	"backoffice-api/internal/delivery/http/dto"
	"backoffice-api/internal/delivery/http/export"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	domaincoupon "crm-core/domain/coupon"
	usecasecoupon "crm-core/usecase/coupon"

	"github.com/gofiber/fiber/v2"
)

// CouponHandler serves coupon management (plan §8). Every rule lives in
// crm-core; this maps HTTP only.
type CouponHandler struct {
	coupons usecasecoupon.Usecase
}

// NewCouponHandler builds the handler.
func NewCouponHandler(coupons usecasecoupon.Usecase) *CouponHandler {
	return &CouponHandler{coupons: coupons}
}

// List serves GET /coupons?month=YYYY-MM&status&channel&search&page&limit.
func (h *CouponHandler) List(c *fiber.Ctx) error {
	page, err := parseOptionalInt(c.Query("page"), "page")
	if err != nil {
		return writeError(c, err)
	}
	limit, err := parseOptionalInt(c.Query("limit"), "limit")
	if err != nil {
		return writeError(c, err)
	}
	q := usecasecoupon.NormalizeQuery(domaincoupon.Query{
		Search: c.Query("search"), Status: c.Query("status"), Channel: c.Query("channel"), Page: page, Limit: limit,
	})
	rows, total, err := h.coupons.List(c.UserContext(), q, c.Query("month"))
	if err != nil {
		return writeError(c, err)
	}
	out := make([]dto.CouponResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.ToCouponResponse(r))
	}
	return response.List(c, out, q.Page, q.Limit, total)
}

// Get serves GET /coupons/:id.
func (h *CouponHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	cp, err := h.coupons.Get(c.UserContext(), id)
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToCouponResponse(*cp))
}

// Create serves POST /coupons (always a draft).
func (h *CouponHandler) Create(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.CouponRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	cp, err := h.coupons.Create(c.UserContext(), actor, req.ToInput())
	if err != nil {
		return writeError(c, err)
	}
	return response.Created(c, dto.ToCouponResponse(*cp), "Coupon created")
}

// Update serves PATCH /coupons/:id.
func (h *CouponHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.CouponRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	cp, err := h.coupons.Update(c.UserContext(), actor, id, req.ToInput())
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToCouponResponse(*cp), "Coupon updated")
}

// Delete serves DELETE /coupons/:id (only before anyone claimed it).
func (h *CouponHandler) Delete(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	if err := h.coupons.Delete(c.UserContext(), id); err != nil {
		return writeError(c, err)
	}
	return response.OK(c, fiber.Map{"id": id}, "Coupon deleted")
}

// Transition serves POST /coupons/:id/activate|pause|end.
func (h *CouponHandler) Transition(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := parseID(c.Params("id"), "id")
		if err != nil {
			return writeError(c, err)
		}
		actor, ok := middleware.ActorFrom(c)
		if !ok {
			return response.Unauthorized(c, "Unauthorized")
		}
		var cp *domaincoupon.Coupon
		switch action {
		case "activate":
			cp, err = h.coupons.Activate(c.UserContext(), actor, id)
		case "pause":
			cp, err = h.coupons.Pause(c.UserContext(), actor, id)
		default:
			cp, err = h.coupons.End(c.UserContext(), actor, id)
		}
		if err != nil {
			return writeError(c, err)
		}
		return response.OK(c, dto.ToCouponResponse(*cp), "Coupon "+action+"d")
	}
}

// Holders serves GET /coupons/:id/holders (?format=xlsx; masked unless owner).
func (h *CouponHandler) Holders(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	if c.Query("format") == "xlsx" {
		return h.exportHolders(c, id)
	}
	page, err := parseOptionalInt(c.Query("page"), "page")
	if err != nil {
		return writeError(c, err)
	}
	limit, err := parseOptionalInt(c.Query("limit"), "limit")
	if err != nil {
		return writeError(c, err)
	}
	if _, err := h.coupons.Get(c.UserContext(), id); err != nil {
		return writeError(c, err)
	}
	return h.holderList(c, domaincoupon.HolderQuery{CouponID: id, Status: c.Query("status"), Page: page, Limit: limit})
}

func (h *CouponHandler) holderList(c *fiber.Ctx, q domaincoupon.HolderQuery) error {
	rows, total, err := h.coupons.Holders(c.UserContext(), q)
	if err != nil {
		return writeError(c, err)
	}
	out := make([]dto.MemberCouponResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.ToHolderResponse(r))
	}
	page, limit := q.Page, q.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return response.List(c, out, page, limit, total)
}

func (h *CouponHandler) exportHolders(c *fiber.Ctx, id uint) error {
	actor, _ := middleware.ActorFrom(c)
	rows, err := h.coupons.ExportHolders(c.UserContext(), id)
	if err != nil {
		return writeError(c, err)
	}
	file, err := export.CouponHoldersXLSX(rows, actor.IsOwner(), bangkok)
	if err != nil {
		return writeError(c, err)
	}
	log.Printf("[AUDIT] coupon holders export coupon_id=%d by erp_user_id=%d rows=%d unmasked=%t",
		id, actor.ERPUserID, len(rows), actor.IsOwner())
	name := fmt.Sprintf("coupon-%d-holders-%s.xlsx", id, time.Now().In(bangkok).Format("20060102-1504"))
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+name+`"`)
	return c.Send(file)
}

// MemberCoupons serves GET /members/:id/coupons.
func (h *CouponHandler) MemberCoupons(c *fiber.Ctx) error {
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
	return h.holderList(c, domaincoupon.HolderQuery{MemberID: id, Status: c.Query("status"), Page: page, Limit: limit})
}

// Grant serves POST /members/:id/coupons — gives a free coupon to a member.
func (h *CouponHandler) Grant(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.GrantCouponRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	mc, err := h.coupons.Claim(c.UserContext(), usecasecoupon.ClaimRequest{
		MemberID: id, CouponID: req.CouponID, Grant: true, Actor: actor,
	})
	if err != nil {
		return writeError(c, err)
	}
	return response.Created(c, dto.ToMemberCouponResponse(*mc), "Coupon granted")
}

// Cancel serves POST /member-coupons/:id/cancel {reason}.
func (h *CouponHandler) Cancel(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.CancelCouponRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	mc, err := h.coupons.Cancel(c.UserContext(), actor, id, req.Reason, time.Time{})
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToMemberCouponResponse(*mc), "Coupon cancelled")
}

// Lookup serves GET /coupons/codes/:code.
func (h *CouponHandler) Lookup(c *fiber.Ctx) error {
	view, err := h.coupons.Lookup(c.UserContext(), c.Params("code"), time.Time{})
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToCodeViewResponse(view))
}

// Use serves POST /coupons/codes/:code/use {usedRef}.
func (h *CouponHandler) Use(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.UseCouponRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	view, err := h.coupons.Use(c.UserContext(), actor, c.Params("code"), req.UsedRef, time.Time{})
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToCodeViewResponse(view), "Coupon used")
}
