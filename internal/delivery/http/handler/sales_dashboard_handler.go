package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"backoffice-api/internal/authz"
	"backoffice-api/internal/client/erp"
	"backoffice-api/internal/delivery/http/dto"
	"backoffice-api/internal/delivery/http/export"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	"crm-core/domain"
	domainsales "crm-core/domain/sales"
	usecasesales "crm-core/usecase/sales"

	"github.com/gofiber/fiber/v2"
)

const (
	// SalesDefaultDays is the default range of the dashboard (plan §6.2).
	SalesDefaultDays = 90
	// SalesMaxDays caps a single request to one year of facts.
	SalesMaxDays = 366
	// DefaultSalesCacheTTL keeps ERP load low; the numbers lag by at most this.
	DefaultSalesCacheTTL = 10 * time.Minute
)

// SalesFetcher loads order facts for one range from the ERP.
type SalesFetcher interface {
	SalesOrders(ctx context.Context, bearerToken string, from, to time.Time, channel, region string) ([]domainsales.SalesOrder, error)
}

type salesCacheEntry struct {
	orders    []domainsales.SalesOrder
	fetchedAt time.Time
}

// SalesDashboardHandler serves the marketplace dashboard. Every number is
// computed in crm-core; this handler only parses filters, caches ERP facts and
// maps the report to DTOs.
type SalesDashboardHandler struct {
	erp   SalesFetcher
	ttl   time.Duration
	now   func() time.Time
	mu    sync.Mutex
	cache map[string]salesCacheEntry
}

// NewSalesDashboardHandler builds the dashboard handler.
func NewSalesDashboardHandler(fetcher SalesFetcher) *SalesDashboardHandler {
	return &SalesDashboardHandler{
		erp:   fetcher,
		ttl:   DefaultSalesCacheTTL,
		now:   time.Now,
		cache: map[string]salesCacheEntry{},
	}
}

// salesQuery is the parsed, validated filter of one request.
type salesQuery struct {
	from, to time.Time
	days     int
	channel  string
	region   string
}

// errNoActor means the route ran without staff authentication.
var errNoActor = errors.New("sales dashboard: no authenticated actor")

// errBuyerData means the caller lacks sales_dashboard.customers.
var errBuyerData = errors.New("sales dashboard: buyer-level data requires permission")

// fail maps a build error to the standard envelope exactly once. Handlers must
// not write a second response after this.
func (h *SalesDashboardHandler) fail(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, errNoActor):
		return response.Unauthorized(c, "Unauthorized")
	case errors.Is(err, errBuyerData):
		return response.Forbidden(c, "You do not have permission to see buyer-level data", "FORBIDDEN")
	default:
		return h.erpError(c, err)
	}
}

// Report serves GET /sales-dashboard: the whole payload in one call.
func (h *SalesDashboardHandler) Report(c *fiber.Ctx) error {
	report, q, err := h.build(c, false)
	if err != nil {
		return h.fail(c, err)
	}
	return response.OK(c, dto.ToSalesReportResponse(report, q.days))
}

// Summary serves GET /sales-dashboard/summary.
func (h *SalesDashboardHandler) Summary(c *fiber.Ctx) error {
	report, q, err := h.build(c, false)
	if err != nil {
		return h.fail(c, err)
	}
	if c.Query("format") == "xlsx" {
		return h.sendXLSX(c, report, export.SalesSummary)
	}
	full := dto.ToSalesReportResponse(report, q.days)
	return response.OK(c, fiber.Map{
		"range":    full.Range,
		"summary":  full.Summary,
		"previous": full.Previous,
		"channels": full.Channels,
		"trend":    full.Trend,
	})
}

// Products serves GET /sales-dashboard/products.
func (h *SalesDashboardHandler) Products(c *fiber.Ctx) error {
	report, _, err := h.build(c, false)
	if err != nil {
		return h.fail(c, err)
	}
	if c.Query("format") == "xlsx" {
		return h.sendXLSX(c, report, export.SalesProducts)
	}
	full := dto.ToSalesReportResponse(report, 0)
	return response.OK(c, fiber.Map{
		"flavors":     full.Flavors,
		"products":    full.Products,
		"sets":        full.Sets,
		"accessories": full.Accessories,
	})
}

// Provinces serves GET /sales-dashboard/provinces.
func (h *SalesDashboardHandler) Provinces(c *fiber.Ctx) error {
	report, _, err := h.build(c, false)
	if err != nil {
		return h.fail(c, err)
	}
	if c.Query("format") == "xlsx" {
		return h.sendXLSX(c, report, export.SalesProvinces)
	}
	full := dto.ToSalesReportResponse(report, 0)
	return response.OK(c, fiber.Map{
		"provinces": full.Provinces,
		"regions":   full.Regions,
	})
}

// Customers serves GET /sales-dashboard/customers. accountant holds
// sales_dashboard.view but not sales_dashboard.customers, so buyer rows are
// refused rather than silently masked (plan §6.4, §13 #5).
func (h *SalesDashboardHandler) Customers(c *fiber.Ctx) error {
	report, _, err := h.build(c, true)
	if err != nil {
		return h.fail(c, err)
	}
	if c.Query("format") == "xlsx" {
		return h.sendXLSX(c, report, export.SalesCustomers)
	}
	full := dto.ToSalesReportResponse(report, 0)
	return response.OK(c, fiber.Map{
		"customers":   full.Customers,
		"frequencies": full.Frequencies,
	})
}

// CustomerDetail serves GET /sales-dashboard/customers/:platform/:buyer.
func (h *SalesDashboardHandler) CustomerDetail(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	if !authz.Allowed(actor.Roles, authz.SalesDashboardCustomers) {
		return response.Forbidden(c, "You do not have permission to see buyer-level data", "FORBIDDEN")
	}
	platform := strings.ToLower(strings.TrimSpace(c.Params("platform")))
	if platform != domainsales.PlatformShopee && platform != domainsales.PlatformTikTok {
		return response.BadRequest(c, "platform must be shopee or tiktok", "VALIDATION_ERROR")
	}
	buyer := strings.TrimSpace(c.Params("buyer"))
	if buyer == "" {
		return response.BadRequest(c, "buyer is required", "VALIDATION_ERROR")
	}

	q, err := parseSalesQuery(c)
	if err != nil {
		return writeError(c, err)
	}
	orders, err := h.fetch(c, q)
	if err != nil {
		return h.erpError(c, err)
	}
	detail, err := usecasesales.Detail(orders, platform, buyer, true)
	if err != nil {
		return writeError(c, err)
	}

	out := dto.SalesCustomerDetailResponse{
		Customer: dto.ToSalesReportResponse(domainsales.Report{
			Customers: []domainsales.CustomerRow{detail.Customer},
		}, 0).Customers[0],
		Orders: make([]dto.SalesCustomerOrderResponse, 0, len(detail.Orders)),
	}
	for _, o := range detail.Orders {
		out.Orders = append(out.Orders, dto.SalesCustomerOrderResponse{
			Platform: o.Platform, OrderNo: o.OrderNo, Date: o.Date, Amount: o.Amount,
			Channel: o.ChannelLabel, Province: o.Province, Items: o.Units,
		})
	}
	return response.OK(c, out)
}

// Timing serves GET /sales-dashboard/timing.
func (h *SalesDashboardHandler) Timing(c *fiber.Ctx) error {
	report, _, err := h.build(c, false)
	if err != nil {
		return h.fail(c, err)
	}
	if c.Query("format") == "xlsx" {
		return h.sendXLSX(c, report, export.SalesTiming)
	}
	return response.OK(c, dto.ToSalesReportResponse(report, 0).Timing)
}

// Repurchase serves GET /sales-dashboard/repurchase.
func (h *SalesDashboardHandler) Repurchase(c *fiber.Ctx) error {
	report, _, err := h.build(c, false)
	if err != nil {
		return h.fail(c, err)
	}
	if c.Query("format") == "xlsx" {
		return h.sendXLSX(c, report, export.SalesRepurchase)
	}
	return response.OK(c, dto.ToSalesReportResponse(report, 0).Repurchase)
}

// build parses the filter, fetches both periods and computes the report. It
// returns typed errors and never writes the response itself, so a handler can
// map the failure exactly once.
func (h *SalesDashboardHandler) build(c *fiber.Ctx, requireCustomers bool) (domainsales.Report, salesQuery, error) {
	var q salesQuery
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return domainsales.Report{}, q, errNoActor
	}
	canSeeCustomers := authz.Allowed(actor.Roles, authz.SalesDashboardCustomers)
	if requireCustomers && !canSeeCustomers {
		return domainsales.Report{}, q, errBuyerData
	}

	q, err := parseSalesQuery(c)
	if err != nil {
		return domainsales.Report{}, q, err
	}
	orders, err := h.fetch(c, q)
	if err != nil {
		return domainsales.Report{}, q, err
	}

	prevFrom, prevTo := usecasesales.PreviousRange(q.from, q.to)
	previous, err := h.fetchRange(c, prevFrom, prevTo, q.channel, q.region)
	if err != nil {
		return domainsales.Report{}, q, err
	}

	report := usecasesales.Build(orders, previous, usecasesales.Options{
		From:     q.from,
		To:       q.to,
		Channel:  q.channel,
		Region:   q.region,
		Unmasked: canSeeCustomers,
	})
	if !canSeeCustomers {
		// accountant sees aggregates only.
		report.Customers = nil
		report.Frequencies = nil
	}
	return report, q, nil
}

func (h *SalesDashboardHandler) fetch(c *fiber.Ctx, q salesQuery) ([]domainsales.SalesOrder, error) {
	return h.fetchRange(c, q.from, q.to, q.channel, q.region)
}

// fetchRange returns cached ERP facts when they are fresh enough.
func (h *SalesDashboardHandler) fetchRange(c *fiber.Ctx, from, to time.Time, channel, region string) ([]domainsales.SalesOrder, error) {
	key := strings.Join([]string{
		from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339), channel, region,
	}, "|")

	h.mu.Lock()
	entry, ok := h.cache[key]
	h.mu.Unlock()
	if ok && h.now().Sub(entry.fetchedAt) < h.ttl {
		return entry.orders, nil
	}

	token := bearerToken(c)
	orders, err := h.erp.SalesOrders(c.UserContext(), token, from, to, channel, region)
	if err != nil {
		return nil, err
	}

	h.mu.Lock()
	h.cache[key] = salesCacheEntry{orders: orders, fetchedAt: h.now()}
	h.mu.Unlock()
	return orders, nil
}

// erpError maps ERP failures to a clear envelope instead of a bare 500.
func (h *SalesDashboardHandler) erpError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, erp.ErrUnauthorized):
		return response.Forbidden(c, "The ERP rejected this session for the sales report", "ERP_FORBIDDEN")
	case errors.Is(err, erp.ErrUnavailable):
		log.Printf("[WARN] sales analytics unavailable: %v", err)
		return response.Error(c, fiber.StatusServiceUnavailable,
			"ERP_UNAVAILABLE", "Sales data is unavailable right now. Try again in a moment.")
	default:
		return writeError(c, err)
	}
}

func bearerToken(c *fiber.Ctx) string {
	raw := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
	if len(raw) > 7 && strings.EqualFold(raw[:7], "bearer ") {
		return strings.TrimSpace(raw[7:])
	}
	return raw
}

// parseSalesQuery validates the date range, channel and region filters.
func parseSalesQuery(c *fiber.Ctx) (salesQuery, error) {
	q := salesQuery{
		channel: strings.ToLower(strings.TrimSpace(c.Query("channel"))),
		region:  strings.TrimSpace(c.Query("region")),
	}
	if q.channel == "all" {
		q.channel = ""
	}
	switch q.channel {
	case "", domainsales.ChannelShopee, domainsales.ChannelTikTok, domainsales.ChannelTikTokLive,
		domainsales.ChannelTikTokProduct, domainsales.ChannelTikTokVideo:
	default:
		return q, domain.NewValidationError("channel", "is not a known sales channel")
	}
	if q.region != "" && !knownRegion(q.region) {
		return q, domain.NewValidationError("region", "is not a known region")
	}

	to := startOfToday()
	if v := c.Query("dateTo"); v != "" {
		t, perr := parseDate(v, "dateTo", true)
		if perr != nil {
			return q, perr
		}
		to = *t
	}
	from := to.AddDate(0, 0, -SalesDefaultDays)
	if v := c.Query("dateFrom"); v != "" {
		t, perr := parseDate(v, "dateFrom", false)
		if perr != nil {
			return q, perr
		}
		from = *t
	}
	if !from.Before(to) {
		return q, domain.NewValidationError("dateFrom", "must be before dateTo")
	}
	days := int(to.Sub(from).Hours()/24 + 0.5)
	if days > SalesMaxDays {
		return q, domain.NewValidationError("dateFrom", "the range may not exceed 366 days")
	}
	q.from, q.to, q.days = from, to, days
	return q, nil
}

func startOfToday() time.Time {
	now := time.Now().In(bangkok)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, bangkok).AddDate(0, 0, 1)
}

func knownRegion(name string) bool {
	for _, r := range domainsales.Regions() {
		if r == name {
			return true
		}
	}
	return false
}

// sendXLSX writes one dashboard tab as .xlsx with the filters of the screen.
// Buyer usernames are masked unless the caller is the owner (PDPA); the
// customer tab itself is already refused for roles without
// sales_dashboard.customers.
func (h *SalesDashboardHandler) sendXLSX(c *fiber.Ctx, report domainsales.Report, section string) error {
	actor, _ := middleware.ActorFrom(c)
	var mask func(string) string
	if !actor.IsOwner() {
		mask = usecasesales.MaskBuyer
	}
	file, err := export.SalesDashboardXLSX(report, section, mask)
	if err != nil {
		return writeError(c, err)
	}
	log.Printf("[AUDIT] sales dashboard export section=%s by erp_user_id=%d from=%s to=%s channel=%q unmasked=%t",
		section, actor.ERPUserID, report.From, report.To, report.Channel, mask == nil)
	name := fmt.Sprintf("sales-%s-%s-%s.xlsx", section, report.From, report.To)
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+name+`"`)
	return c.Send(file)
}
