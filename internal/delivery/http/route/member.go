package route

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// registerMember mounts the member module.
//
//	GET    /api/v1/crm/tiers                      members.view
//	GET    /api/v1/crm/members                    members.view (?format=xlsx needs members.export)
//	GET    /api/v1/crm/members/:id                members.view
//	PATCH  /api/v1/crm/members/:id                members.manage
//	DELETE /api/v1/crm/members/:id/line           members.manage
//	GET    /api/v1/crm/members/:id/tags           members.view
//	POST   /api/v1/crm/members/:id/tags           members.manage
//	DELETE /api/v1/crm/members/:id/tags/:tag      members.manage
//	GET    /api/v1/crm/members/:id/notes          members.notes
//	POST   /api/v1/crm/members/:id/notes          members.notes
//	GET    /api/v1/crm/members/:id/points         points.view
//	POST   /api/v1/crm/members/:id/points/adjust  points.adjust
func registerMember(crm fiber.Router, h Handlers) {
	crm.Get("/tiers", middleware.RequirePermission(authz.MembersView), h.Member.Tiers)
	crm.Get("/members", middleware.RequirePermission(authz.MembersView), h.Member.List)
	crm.Get("/members/:id", middleware.RequirePermission(authz.MembersView), h.Member.Get)
	crm.Patch("/members/:id", middleware.RequirePermission(authz.MembersManage), h.Member.Update)
	crm.Delete("/members/:id/line", middleware.RequirePermission(authz.MembersManage), h.Member.UnlinkLine)
	crm.Get("/members/:id/tags", middleware.RequirePermission(authz.MembersView), h.Member.Tags)
	crm.Post("/members/:id/tags", middleware.RequirePermission(authz.MembersManage), h.Member.AddTag)
	crm.Delete("/members/:id/tags/:tag", middleware.RequirePermission(authz.MembersManage), h.Member.RemoveTag)
	crm.Get("/members/:id/notes", middleware.RequirePermission(authz.MembersNotes), h.Member.Notes)
	crm.Post("/members/:id/notes", middleware.RequirePermission(authz.MembersNotes), h.Member.AddNote)

	if h.Points != nil {
		crm.Get("/members/:id/points", middleware.RequirePermission(authz.PointsView), h.Points.Ledger)
		crm.Post("/members/:id/points/adjust", middleware.RequirePermission(authz.PointsAdjust), h.Points.Adjust)
	}
}
