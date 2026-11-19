package route

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// registerCoupon mounts coupon management (plan §8).
//
//	GET    /api/v1/crm/coupons                       coupons.view (?month&status&channel&search)
//	POST   /api/v1/crm/coupons                       coupons.manage
//	GET    /api/v1/crm/coupons/codes/:code           coupons.use    (store lookup)
//	POST   /api/v1/crm/coupons/codes/:code/use       coupons.use    {usedRef}
//	GET    /api/v1/crm/coupons/:id                   coupons.view
//	PATCH  /api/v1/crm/coupons/:id                   coupons.manage
//	DELETE /api/v1/crm/coupons/:id                   coupons.manage (unclaimed only)
//	POST   /api/v1/crm/coupons/:id/activate          coupons.manage
//	POST   /api/v1/crm/coupons/:id/pause             coupons.manage
//	POST   /api/v1/crm/coupons/:id/end               coupons.manage
//	GET    /api/v1/crm/coupons/:id/holders           coupons.view (?format=xlsx, masked unless owner)
//	POST   /api/v1/crm/member-coupons/:id/cancel     coupons.manage {reason}
//	GET    /api/v1/crm/members/:id/coupons           members.view
//	POST   /api/v1/crm/members/:id/coupons           coupons.manage {couponId} (free coupons only)
func registerCoupon(crm fiber.Router, h Handlers) {
	if h.Coupon == nil {
		return
	}
	view := middleware.RequirePermission(authz.CouponsView)
	manage := middleware.RequirePermission(authz.CouponsManage)
	use := middleware.RequirePermission(authz.CouponsUse)

	// The code routes go first so "codes" is never parsed as an :id.
	crm.Get("/coupons/codes/:code", use, h.Coupon.Lookup)
	crm.Post("/coupons/codes/:code/use", use, h.Coupon.Use)

	crm.Get("/coupons", view, h.Coupon.List)
	crm.Post("/coupons", manage, h.Coupon.Create)
	crm.Get("/coupons/:id", view, h.Coupon.Get)
	crm.Patch("/coupons/:id", manage, h.Coupon.Update)
	crm.Delete("/coupons/:id", manage, h.Coupon.Delete)
	crm.Post("/coupons/:id/activate", manage, h.Coupon.Transition("activate"))
	crm.Post("/coupons/:id/pause", manage, h.Coupon.Transition("pause"))
	crm.Post("/coupons/:id/end", manage, h.Coupon.Transition("end"))
	crm.Get("/coupons/:id/holders", view, h.Coupon.Holders)

	crm.Post("/member-coupons/:id/cancel", manage, h.Coupon.Cancel)
	crm.Get("/members/:id/coupons", middleware.RequirePermission(authz.MembersView), h.Coupon.MemberCoupons)
	crm.Post("/members/:id/coupons", manage, h.Coupon.Grant)
}
