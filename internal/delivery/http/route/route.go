// Package route wires HTTP routes. All endpoints live under /api/v1.
//
// Every module owns one route file with a registerXxx(crm, h) function and its
// endpoint list in the file header (health.go, member.go, staff_role.go, consent.go, product_code.go, privacy.go,
// sales_dashboard.go, upload.go, promotion.go, point_rules.go, coupon.go, reward.go, report.go). A new module adds:
//
//  1. one field to Handlers,
//  2. one registerXxx call in Register,
//  3. its own route file.
//
// Nothing else in this file changes.
package route

import (
	"backoffice-api/internal/delivery/http/handler"

	"github.com/gofiber/fiber/v2"
)

// Handlers groups every handler the router needs.
type Handlers struct {
	Health         *handler.HealthHandler
	Member         *handler.MemberHandler
	Points         *handler.PointsHandler
	StaffRole      *handler.StaffRoleHandler
	SalesDashboard *handler.SalesDashboardHandler
	Upload         *handler.UploadHandler
	Promotion      *handler.PromotionHandler
	PointRules     *handler.PointRulesHandler
	Coupon         *handler.CouponHandler
	Reward         *handler.RewardHandler
	MemberReport   *handler.MemberReportHandler
	Consent        *handler.ConsentHandler
	ProductCode    *handler.ProductCodeHandler
	Privacy        *handler.PrivacyHandler
}

// Register mounts all routes on app.
//
// The per-module endpoint lists live in each registerXxx file.
func Register(app *fiber.App, h Handlers, staffAuth fiber.Handler) {
	api := app.Group("/api/v1")
	registerHealth(api, h)

	crm := api.Group("/crm", staffAuth)
	crm.Get("/me", handler.Me)

	registerMember(crm, h)
	registerStaffRole(crm, h)
	registerSalesDashboard(crm, h)
	registerUploads(crm, h)
	registerPromotion(crm, h)
	registerPointRules(crm, h)
	registerCoupon(crm, h)
	registerReward(crm, h)
	registerReport(crm, h)
	registerConsent(crm, h)
	registerProductCode(crm, h)
	registerPrivacy(crm, h)

	registerFiles(api, h)
}
