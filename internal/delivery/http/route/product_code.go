package route

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// registerProductCode mounts packaging-code batches (L4B). The SKU dropdown
// uses GET /crm/erp/skus from reward.go.
//
//	GET   /api/v1/crm/product-code-batches                product_codes.manage
//	POST  /api/v1/crm/product-code-batches                product_codes.manage
//	GET   /api/v1/crm/product-code-batches/:id            product_codes.manage
//	PATCH /api/v1/crm/product-code-batches/:id            product_codes.manage (draft)
//	POST  /api/v1/crm/product-code-batches/:id/export     product_codes.export (once; ?format=xlsx|csv)
//	POST  /api/v1/crm/product-code-batches/:id/activate   product_codes.manage
//	POST  /api/v1/crm/product-code-batches/:id/void       product_codes.manage {reason}
//	GET   /api/v1/crm/product-code-policy                 product_codes.manage
//	PUT   /api/v1/crm/product-code-policy                 product_codes.manage (owner, checked in crm-core)
//	GET   /api/v1/crm/product-code-flags                  product_codes.manage
//	GET   /api/v1/crm/test-members                        product_codes.manage
//	PUT   /api/v1/crm/test-members/:memberId              product_codes.manage
//	DELETE /api/v1/crm/test-members/:memberId             product_codes.manage
func registerProductCode(crm fiber.Router, h Handlers) {
	if h.ProductCode == nil {
		return
	}
	manage := middleware.RequirePermission(authz.ProductCodesManage)
	exportPerm := middleware.RequirePermission(authz.ProductCodesExport)

	batches := crm.Group("/product-code-batches")
	batches.Get("/", manage, h.ProductCode.List)
	batches.Post("/", manage, h.ProductCode.Create)
	batches.Get("/:id", manage, h.ProductCode.Get)
	batches.Patch("/:id", manage, h.ProductCode.Update)
	batches.Post("/:id/export", exportPerm, h.ProductCode.Export)
	batches.Post("/:id/activate", manage, h.ProductCode.Activate)
	batches.Post("/:id/void", manage, h.ProductCode.Void)

	if h.ProductCode.HasMembers() {
		crm.Get("/product-code-policy", manage, h.ProductCode.Policy)
		crm.Put("/product-code-policy", manage, h.ProductCode.PutPolicy)
		crm.Get("/product-code-flags", manage, h.ProductCode.Flagged)
		crm.Get("/test-members", manage, h.ProductCode.TestMembers)
		crm.Put("/test-members/:memberId", manage, h.ProductCode.AddTestMember)
		crm.Delete("/test-members/:memberId", manage, h.ProductCode.RemoveTestMember)
	}
}
