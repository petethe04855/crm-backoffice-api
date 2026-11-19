package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"backoffice-api/internal/client/erp"
	"backoffice-api/internal/delivery/http/dto"
	"backoffice-api/internal/delivery/http/export"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	"crm-core/domain"
	domainreward "crm-core/domain/reward"
	usecasereward "crm-core/usecase/reward"

	"github.com/gofiber/fiber/v2"
)

// SKUSearcher is the ERP SKU search the reward form uses (the ERP client).
type SKUSearcher interface {
	SearchSKUs(ctx context.Context, search string, warehouseID uint, page, limit int) ([]erp.SKU, int64, error)
}

// RewardHandler serves the reward catalogue and redemptions (plan §5).
type RewardHandler struct {
	rewards usecasereward.Usecase
	skus    SKUSearcher
}

// NewRewardHandler builds the handler. skus may be nil (no ERP search).
func NewRewardHandler(rewards usecasereward.Usecase, skus SKUSearcher) *RewardHandler {
	return &RewardHandler{rewards: rewards, skus: skus}
}

// ---- categories ----

// Categories serves GET /reward-categories.
func (h *RewardHandler) Categories(c *fiber.Ctx) error {
	cats, err := h.rewards.Categories(c.UserContext())
	if err != nil {
		return writeError(c, err)
	}
	out := make([]dto.RewardCategoryResponse, 0, len(cats))
	for _, cat := range cats {
		out = append(out, dto.ToRewardCategoryResponse(cat))
	}
	return response.OK(c, out)
}

// CreateCategory serves POST /reward-categories.
func (h *RewardHandler) CreateCategory(c *fiber.Ctx) error {
	var req dto.RewardCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	cat, err := h.rewards.CreateCategory(c.UserContext(), req.Name, req.SortOrder)
	if err != nil {
		return writeError(c, err)
	}
	return response.Created(c, dto.ToRewardCategoryResponse(*cat), "Category created")
}

// UpdateCategory serves PATCH /reward-categories/:id.
func (h *RewardHandler) UpdateCategory(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	var req dto.RewardCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	cat, err := h.rewards.UpdateCategory(c.UserContext(), id, req.Name, req.SortOrder, active)
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToRewardCategoryResponse(*cat), "Category updated")
}

// DeleteCategory serves DELETE /reward-categories/:id (empty categories only).
func (h *RewardHandler) DeleteCategory(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	if err := h.rewards.DeleteCategory(c.UserContext(), id); err != nil {
		return writeError(c, err)
	}
	return response.OK(c, fiber.Map{"id": id}, "Category deleted")
}

// ---- rewards ----

// List serves GET /rewards?search&categoryId&status&lowStock&page&limit.
func (h *RewardHandler) List(c *fiber.Ctx) error {
	page, err := parseOptionalInt(c.Query("page"), "page")
	if err != nil {
		return writeError(c, err)
	}
	limit, err := parseOptionalInt(c.Query("limit"), "limit")
	if err != nil {
		return writeError(c, err)
	}
	q := domainreward.Query{Search: c.Query("search"), Status: c.Query("status"), Page: page, Limit: limit,
		LowStock: c.Query("lowStock") == "true"}
	if v := c.Query("categoryId"); v != "" {
		if q.CategoryID, err = parseID(v, "categoryId"); err != nil {
			return writeError(c, err)
		}
	}
	rows, total, err := h.rewards.List(c.UserContext(), q)
	if err != nil {
		return writeError(c, err)
	}
	out := make([]dto.RewardResponse, 0, len(rows))
	for _, v := range rows {
		out = append(out, dto.ToRewardResponse(v))
	}
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

// Get serves GET /rewards/:id.
func (h *RewardHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	v, err := h.rewards.Get(c.UserContext(), id)
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToRewardResponse(*v))
}

// Create serves POST /rewards (always a draft).
func (h *RewardHandler) Create(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.RewardRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	v, err := h.rewards.Create(c.UserContext(), actor, req.ToInput())
	if err != nil {
		return writeError(c, err)
	}
	return response.Created(c, dto.ToRewardResponse(*v), "Reward created")
}

// Update serves PATCH /rewards/:id.
func (h *RewardHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.RewardRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	v, err := h.rewards.Update(c.UserContext(), actor, id, req.ToInput())
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToRewardResponse(*v), "Reward updated")
}

// SetStatus serves POST /rewards/:id/status {status}.
func (h *RewardHandler) SetStatus(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.RewardStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	v, err := h.rewards.SetStatus(c.UserContext(), actor, id, req.Status)
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToRewardResponse(*v), "Reward status updated")
}

// SearchERPSKUs serves GET /erp/skus?search=&page=&limit= (proxy to the ERP).
func (h *RewardHandler) SearchERPSKUs(c *fiber.Ctx) error {
	if h.skus == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "ERP_UNAVAILABLE", "ERP integration is not configured")
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	skus, total, err := h.skus.SearchSKUs(c.UserContext(), c.Query("search"), 0, page, limit)
	if err != nil {
		if errors.Is(err, erp.ErrNotConfigured) {
			return response.Error(c, fiber.StatusServiceUnavailable, "ERP_UNAVAILABLE", "ERP integration token is not configured")
		}
		log.Printf("[WARN] ERP SKU search failed: %v", err)
		return response.Error(c, fiber.StatusServiceUnavailable, "ERP_UNAVAILABLE", "ERP is unavailable, try again later")
	}
	out := make([]dto.ERPSKUResponse, 0, len(skus))
	for _, s := range skus {
		out = append(out, dto.ERPSKUResponse{ID: s.ID, Code: s.Code, Name: s.Name, ImageURL: s.ImageURL,
			Category: s.Category, Price: s.Price, AvailableQty: s.AvailableQty})
	}
	return response.List(c, out, page, limit, total)
}

// ---- redemptions ----

func parseRedemptionQuery(c *fiber.Ctx) (domainreward.RedemptionQuery, error) {
	q := domainreward.RedemptionQuery{Status: c.Query("status"), Search: c.Query("search")}
	var err error
	if q.Page, err = parseOptionalInt(c.Query("page"), "page"); err != nil {
		return q, err
	}
	if q.Limit, err = parseOptionalInt(c.Query("limit"), "limit"); err != nil {
		return q, err
	}
	if v := c.Query("rewardId"); v != "" {
		if q.RewardID, err = parseID(v, "rewardId"); err != nil {
			return q, err
		}
	}
	if q.From, err = parseDate(c.Query("dateFrom"), "dateFrom", false); err != nil {
		return q, err
	}
	if q.To, err = parseDate(c.Query("dateTo"), "dateTo", true); err != nil {
		return q, err
	}
	return q, nil
}

func (h *RewardHandler) redemptionList(c *fiber.Ctx, q domainreward.RedemptionQuery) error {
	rows, total, err := h.rewards.Redemptions(c.UserContext(), q)
	if err != nil {
		return writeError(c, err)
	}
	out := make([]dto.RedemptionResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.ToRedemptionResponse(r))
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

// Redemptions serves GET /redemptions (?format=xlsx, masked unless owner).
func (h *RewardHandler) Redemptions(c *fiber.Ctx) error {
	q, err := parseRedemptionQuery(c)
	if err != nil {
		return writeError(c, err)
	}
	if c.Query("format") == "xlsx" {
		return h.exportRedemptions(c, q)
	}
	return h.redemptionList(c, q)
}

func (h *RewardHandler) exportRedemptions(c *fiber.Ctx, q domainreward.RedemptionQuery) error {
	actor, _ := middleware.ActorFrom(c)
	q.Page, q.Limit = 1, usecasereward.ExportLimit()
	rows, _, err := h.rewards.Redemptions(c.UserContext(), q)
	if err != nil {
		return writeError(c, err)
	}
	file, err := export.RedemptionsXLSX(rows, actor.IsOwner(), bangkok)
	if err != nil {
		return writeError(c, err)
	}
	log.Printf("[AUDIT] redemptions export by erp_user_id=%d rows=%d status=%q unmasked=%t",
		actor.ERPUserID, len(rows), q.Status, actor.IsOwner())
	name := fmt.Sprintf("redemptions-%s.xlsx", time.Now().In(bangkok).Format("20060102-1504"))
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+name+`"`)
	return c.Send(file)
}

// MemberRedemptions serves GET /members/:id/redemptions.
func (h *RewardHandler) MemberRedemptions(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	q, err := parseRedemptionQuery(c)
	if err != nil {
		return writeError(c, err)
	}
	q.MemberID = id
	return h.redemptionList(c, q)
}

// GetRedemption serves GET /redemptions/:id.
func (h *RewardHandler) GetRedemption(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	rd, err := h.rewards.Redemption(c.UserContext(), id)
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToRedemptionResponse(domainreward.RedemptionRow{Redemption: *rd}))
}

// Transition serves POST /redemptions/:id/approve|pack|ship|deliver|cancel.
func (h *RewardHandler) Transition(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := parseID(c.Params("id"), "id")
		if err != nil {
			return writeError(c, err)
		}
		actor, ok := middleware.ActorFrom(c)
		if !ok {
			return response.Unauthorized(c, "Unauthorized")
		}
		ctx := c.UserContext()
		var rd *domainreward.Redemption
		switch action {
		case "approve":
			rd, err = h.rewards.Approve(ctx, actor, id)
		case "pack":
			rd, err = h.rewards.Pack(ctx, actor, id)
		case "deliver":
			rd, err = h.rewards.Deliver(ctx, actor, id)
		case "ship":
			var req dto.ShipRequest
			if err := c.BodyParser(&req); err != nil {
				return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
			}
			rd, err = h.rewards.Ship(ctx, actor, id, req.TrackingNo)
		case "cancel":
			var req dto.CancelRedemptionRequest
			if err := c.BodyParser(&req); err != nil {
				return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
			}
			rd, err = h.rewards.Cancel(ctx, actor, id, req.Reason)
		default:
			return writeError(c, domain.ErrNotFound)
		}
		if err != nil {
			return writeError(c, err)
		}
		return response.OK(c, dto.ToRedemptionResponse(domainreward.RedemptionRow{Redemption: *rd}), "Redemption "+action+" done")
	}
}
