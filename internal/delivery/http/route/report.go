package route

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// registerReport mounts CRM reports.
//
//	GET /api/v1/crm/reports/members   reports.view (?dateFrom&dateTo, inclusive)
func registerReport(crm fiber.Router, h Handlers) {
	if h.MemberReport == nil {
		return
	}
	crm.Get("/reports/members", middleware.RequirePermission(authz.ReportsView), h.MemberReport.Members)
}
