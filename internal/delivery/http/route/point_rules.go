package route

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// registerPointRules mounts the earn rules, point policies and calculator
// (plan §4). Reads need settings.view; every write needs settings.manage
// (owner only), and the usecase re-checks the owner for the policies.
//
//	GET    /api/v1/crm/earn-rules                       settings.view
//	POST   /api/v1/crm/earn-rules                       settings.manage
//	PATCH  /api/v1/crm/earn-rules/:id                   settings.manage
//	GET    /api/v1/crm/settings/redeem-policy           settings.view
//	PUT    /api/v1/crm/settings/redeem-policy           settings.manage
//	GET    /api/v1/crm/settings/point-adjust-policy     settings.view
//	PUT    /api/v1/crm/settings/point-adjust-policy     settings.manage
//	GET    /api/v1/crm/settings/point-expiry            settings.view
//	PUT    /api/v1/crm/settings/point-expiry            settings.manage
//	POST   /api/v1/crm/point-rules/simulate             settings.view
func registerPointRules(crm fiber.Router, h Handlers) {
	if h.PointRules == nil {
		return
	}
	view := middleware.RequirePermission(authz.SettingsView)
	manage := middleware.RequirePermission(authz.SettingsManage)

	crm.Get("/earn-rules", view, h.PointRules.ListEarnRules)
	crm.Post("/earn-rules", manage, h.PointRules.CreateEarnRule)
	crm.Patch("/earn-rules/:id", manage, h.PointRules.UpdateEarnRule)

	crm.Get("/settings/redeem-policy", view, h.PointRules.RedeemPolicy)
	crm.Put("/settings/redeem-policy", manage, h.PointRules.PutRedeemPolicy)
	crm.Get("/settings/point-adjust-policy", view, h.PointRules.AdjustPolicy)
	crm.Put("/settings/point-adjust-policy", manage, h.PointRules.PutAdjustPolicy)
	crm.Get("/settings/point-expiry", view, h.PointRules.ExpiryPolicy)
	crm.Put("/settings/point-expiry", manage, h.PointRules.PutExpiryPolicy)

	crm.Post("/point-rules/simulate", view, h.PointRules.Simulate)
}
