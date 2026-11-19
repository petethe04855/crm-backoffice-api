package route

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// registerStaffRole mounts the CRM role grants (owner only).
//
//	GET    /api/v1/crm/staff-roles                staff_roles.manage
//	PUT    /api/v1/crm/staff-roles/:erpUserId     staff_roles.manage
//	DELETE /api/v1/crm/staff-roles/:erpUserId     staff_roles.manage
func registerStaffRole(crm fiber.Router, h Handlers) {
	staff := crm.Group("/staff-roles", middleware.RequirePermission(authz.StaffRolesManage))
	staff.Get("/", h.StaffRole.List)
	staff.Put("/:erpUserId", h.StaffRole.Grant)
	staff.Delete("/:erpUserId", h.StaffRole.Revoke)
}
