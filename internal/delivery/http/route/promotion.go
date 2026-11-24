package route

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// registerPromotion mounts the marketing promotion module.
//
//	GET    /api/v1/crm/promotions                  promotions.view
//	GET    /api/v1/crm/promotions/:id              promotions.view
//	POST   /api/v1/crm/promotions                  promotions.manage
//	PATCH  /api/v1/crm/promotions/:id              promotions.manage
//	PATCH  /api/v1/crm/promotions/:id/priority     promotions.manage
//	DELETE /api/v1/crm/promotions/:id              promotions.manage (drafts only)
//	POST   /api/v1/crm/promotions/:id/publish      promotions.manage
//	POST   /api/v1/crm/promotions/:id/unpublish    promotions.manage
//	POST   /api/v1/crm/promotions/:id/archive      promotions.manage
//	POST   /api/v1/crm/promotions/:id/duplicate    promotions.manage
func registerPromotion(crm fiber.Router, h Handlers) {
	promotions := crm.Group("/promotions")
	promotions.Get("/", middleware.RequirePermission(authz.PromotionsView), h.Promotion.List)
	promotions.Get("/:id", middleware.RequirePermission(authz.PromotionsView), h.Promotion.Get)
	promotions.Post("/", middleware.RequirePermission(authz.PromotionsManage), h.Promotion.Create)
	promotions.Patch("/:id", middleware.RequirePermission(authz.PromotionsManage), h.Promotion.Update)
	promotions.Patch("/:id/priority", middleware.RequirePermission(authz.PromotionsManage), h.Promotion.SetPriority)
	promotions.Delete("/:id", middleware.RequirePermission(authz.PromotionsManage), h.Promotion.Delete)
	promotions.Post("/:id/publish", middleware.RequirePermission(authz.PromotionsManage), h.Promotion.Publish)
	promotions.Post("/:id/unpublish", middleware.RequirePermission(authz.PromotionsManage), h.Promotion.Unpublish)
	promotions.Post("/:id/archive", middleware.RequirePermission(authz.PromotionsManage), h.Promotion.Archive)
	promotions.Post("/:id/duplicate", middleware.RequirePermission(authz.PromotionsManage), h.Promotion.Duplicate)
}
