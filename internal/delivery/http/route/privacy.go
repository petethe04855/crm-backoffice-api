package route

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// registerPrivacy mounts the PDPA deletion request queue (L7).
//
//	GET  /api/v1/crm/data-requests?status=          members.manage
//	POST /api/v1/crm/data-requests/:id/complete     members.manage {note}
//	POST /api/v1/crm/data-requests/:id/reject       members.manage {note}
func registerPrivacy(crm fiber.Router, h Handlers) {
	if h.Privacy == nil {
		return
	}
	manage := middleware.RequirePermission(authz.MembersManage)
	crm.Get("/data-requests", manage, h.Privacy.List)
	crm.Post("/data-requests/:id/complete", manage, h.Privacy.Complete)
	crm.Post("/data-requests/:id/reject", manage, h.Privacy.Reject)
}
