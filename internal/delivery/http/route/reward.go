package route

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// registerReward mounts the reward catalogue and redemptions (plan §5).
//
//	GET    /api/v1/crm/reward-categories               rewards.view
//	POST   /api/v1/crm/reward-categories               rewards.manage
//	PATCH  /api/v1/crm/reward-categories/:id           rewards.manage
//	DELETE /api/v1/crm/reward-categories/:id           rewards.manage (empty only)
//	GET    /api/v1/crm/rewards                         rewards.view (?search&categoryId&status&lowStock)
//	POST   /api/v1/crm/rewards                         rewards.manage
//	GET    /api/v1/crm/rewards/:id                     rewards.view
//	PATCH  /api/v1/crm/rewards/:id                     rewards.manage
//	POST   /api/v1/crm/rewards/:id/status              rewards.manage {status}
//	GET    /api/v1/crm/erp/skus                        rewards.manage (ERP proxy)
//	GET    /api/v1/crm/redemptions                     redemptions.fulfill (?format=xlsx)
//	GET    /api/v1/crm/redemptions/:id                 redemptions.fulfill
//	POST   /api/v1/crm/redemptions/:id/approve|pack|ship|deliver|cancel   redemptions.fulfill
//	GET    /api/v1/crm/members/:id/redemptions         members.view
func registerReward(crm fiber.Router, h Handlers) {
	if h.Reward == nil {
		return
	}
	view := middleware.RequirePermission(authz.RewardsView)
	manage := middleware.RequirePermission(authz.RewardsManage)
	fulfill := middleware.RequirePermission(authz.RedemptionsFulfill)

	crm.Get("/reward-categories", view, h.Reward.Categories)
	crm.Post("/reward-categories", manage, h.Reward.CreateCategory)
	crm.Patch("/reward-categories/:id", manage, h.Reward.UpdateCategory)
	crm.Delete("/reward-categories/:id", manage, h.Reward.DeleteCategory)

	crm.Get("/rewards", view, h.Reward.List)
	crm.Post("/rewards", manage, h.Reward.Create)
	crm.Get("/rewards/:id", view, h.Reward.Get)
	crm.Patch("/rewards/:id", manage, h.Reward.Update)
	crm.Post("/rewards/:id/status", manage, h.Reward.SetStatus)
	crm.Get("/erp/skus", manage, h.Reward.SearchERPSKUs)

	crm.Get("/redemptions", fulfill, h.Reward.Redemptions)
	crm.Get("/redemptions/:id", fulfill, h.Reward.GetRedemption)
	for _, action := range []string{"approve", "pack", "ship", "deliver", "cancel"} {
		crm.Post("/redemptions/:id/"+action, fulfill, h.Reward.Transition(action))
	}
	crm.Get("/members/:id/redemptions", middleware.RequirePermission(authz.MembersView), h.Reward.MemberRedemptions)
}
