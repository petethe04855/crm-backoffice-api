package route

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// registerSalesDashboard mounts the marketplace sales dashboard. Every path
// needs sales_dashboard.view; buyer-level paths need sales_dashboard.customers
// on top (the handler enforces that part so aggregates stay visible).
//
//	GET /api/v1/crm/sales-dashboard                          sales_dashboard.view
//	GET /api/v1/crm/sales-dashboard/summary                  sales_dashboard.view
//	GET /api/v1/crm/sales-dashboard/products                 sales_dashboard.view
//	GET /api/v1/crm/sales-dashboard/provinces                sales_dashboard.view
//	GET /api/v1/crm/sales-dashboard/customers                sales_dashboard.view + .customers
//	GET /api/v1/crm/sales-dashboard/customers/:platform/:buyer  sales_dashboard.view + .customers
//	GET /api/v1/crm/sales-dashboard/timing                   sales_dashboard.view
//	GET /api/v1/crm/sales-dashboard/repurchase               sales_dashboard.view
func registerSalesDashboard(crm fiber.Router, h Handlers) {
	if h.SalesDashboard == nil {
		return
	}
	dashboard := crm.Group("/sales-dashboard", middleware.RequirePermission(authz.SalesDashboardView))
	dashboard.Get("", h.SalesDashboard.Report)
	dashboard.Get("/", h.SalesDashboard.Report)
	dashboard.Get("/summary", h.SalesDashboard.Summary)
	dashboard.Get("/products", h.SalesDashboard.Products)
	dashboard.Get("/provinces", h.SalesDashboard.Provinces)
	dashboard.Get("/customers", h.SalesDashboard.Customers)
	dashboard.Get("/customers/:platform/:buyer", h.SalesDashboard.CustomerDetail)
	dashboard.Get("/timing", h.SalesDashboard.Timing)
	dashboard.Get("/repurchase", h.SalesDashboard.Repurchase)
}
