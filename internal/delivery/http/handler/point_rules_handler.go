package handler

import (
	"time"

	"backoffice-api/internal/delivery/http/dto"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	domainearnrule "crm-core/domain/earnrule"
	usecasepoints "crm-core/usecase/points"

	"github.com/gofiber/fiber/v2"
)

// PointRulesHandler serves the earn rules, the point policies and the
// calculator (plan §4). Every rule lives in crm-core; this only maps HTTP.
type PointRulesHandler struct {
	points usecasepoints.Usecase
}

// NewPointRulesHandler builds the handler.
func NewPointRulesHandler(points usecasepoints.Usecase) *PointRulesHandler {
	return &PointRulesHandler{points: points}
}

// ListEarnRules serves GET /earn-rules?type=.
func (h *PointRulesHandler) ListEarnRules(c *fiber.Ctx) error {
	rules, err := h.points.ListEarnRules(c.UserContext(), domainearnrule.Query{Type: c.Query("type")})
	if err != nil {
		return writeError(c, err)
	}
	out := make([]dto.EarnRuleResponse, 0, len(rules))
	for _, r := range rules {
		out = append(out, dto.ToEarnRuleResponse(r))
	}
	return response.OK(c, out)
}

// CreateEarnRule serves POST /earn-rules.
func (h *PointRulesHandler) CreateEarnRule(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.EarnRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	rule, err := h.points.CreateEarnRule(c.UserContext(), actor, toEarnRuleInput(req))
	if err != nil {
		return writeError(c, err)
	}
	return response.Created(c, dto.ToEarnRuleResponse(*rule), "Earn rule created")
}

// UpdateEarnRule serves PATCH /earn-rules/:id.
func (h *PointRulesHandler) UpdateEarnRule(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.EarnRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	rule, err := h.points.UpdateEarnRule(c.UserContext(), actor, id, toEarnRuleInput(req))
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToEarnRuleResponse(*rule), "Earn rule updated")
}

func toEarnRuleInput(r dto.EarnRuleRequest) usecasepoints.EarnRuleInput {
	return usecasepoints.EarnRuleInput{
		Type: r.Type, TierID: r.TierID, Multiplier: r.Multiplier, SKUIDs: r.SKUIDs,
		Trigger: r.Trigger, Points: r.Points, StartAt: r.StartAt, EndAt: r.EndAt,
		Stackable: r.Stackable, Priority: r.Priority, IsActive: r.IsActive,
	}
}

// RedeemPolicy serves GET /settings/redeem-policy.
func (h *PointRulesHandler) RedeemPolicy(c *fiber.Ctx) error {
	p, err := h.points.RedeemPolicy(c.UserContext())
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, p)
}

// PutRedeemPolicy serves PUT /settings/redeem-policy.
func (h *PointRulesHandler) PutRedeemPolicy(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req usecasepoints.RedeemPolicy
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	p, err := h.points.PutRedeemPolicy(c.UserContext(), actor, req)
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, p, "Redeem policy updated")
}

// AdjustPolicy serves GET /settings/point-adjust-policy.
func (h *PointRulesHandler) AdjustPolicy(c *fiber.Ctx) error {
	p, err := h.points.Policy(c.UserContext())
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, p)
}

// PutAdjustPolicy serves PUT /settings/point-adjust-policy.
func (h *PointRulesHandler) PutAdjustPolicy(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req usecasepoints.Policy
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	p, err := h.points.PutAdjustPolicy(c.UserContext(), actor, req)
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, p, "Point adjustment policy updated")
}

// ExpiryPolicy serves GET /settings/point-expiry.
func (h *PointRulesHandler) ExpiryPolicy(c *fiber.Ctx) error {
	p, err := h.points.ExpiryPolicy(c.UserContext())
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, p)
}

// PutExpiryPolicy serves PUT /settings/point-expiry.
func (h *PointRulesHandler) PutExpiryPolicy(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req usecasepoints.ExpiryPolicy
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	p, err := h.points.PutExpiryPolicy(c.UserContext(), actor, req)
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, p, "Point expiry updated")
}

// Simulate serves POST /point-rules/simulate. The web calculator never
// computes points itself.
func (h *PointRulesHandler) Simulate(c *fiber.Ctx) error {
	var req dto.SimulateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	at := time.Now()
	if req.At != nil {
		at = *req.At
	}
	res, err := h.points.Calculate(c.UserContext(), usecasepoints.CalculateInput{
		TierID: req.TierID, SKUID: req.SKUID, BasePoints: req.BasePoints, At: at,
	})
	if err != nil {
		return writeError(c, err)
	}
	if res.Rules == nil {
		res.Rules = []usecasepoints.AppliedRule{}
	}
	return response.OK(c, res)
}
