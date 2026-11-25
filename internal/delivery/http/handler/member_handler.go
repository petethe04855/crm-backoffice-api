package handler

import (
	"fmt"
	"log"
	"strconv"
	"time"
	_ "time/tzdata" // Asia/Bangkok must resolve on hosts without tz files

	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/dto"
	"backoffice-api/internal/delivery/http/export"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	"crm-core/domain"
	domainmember "crm-core/domain/member"
	usecasemember "crm-core/usecase/member"
	usecasetier "crm-core/usecase/tier"

	"github.com/gofiber/fiber/v2"
)

var bangkok = mustLoadLocation("Asia/Bangkok")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// MemberHandler serves member endpoints.
type MemberHandler struct {
	members  usecasemember.Usecase
	exporter usecasemember.Exporter
	tiers    usecasetier.Usecase
}

// NewMemberHandler builds the member handler.
func NewMemberHandler(members usecasemember.Usecase, exporter usecasemember.Exporter, tiers usecasetier.Usecase) *MemberHandler {
	return &MemberHandler{members: members, exporter: exporter, tiers: tiers}
}

// List serves GET /members. ?format=xlsx downloads the filtered list
// (requires members.export; personal data is masked for non-owners).
func (h *MemberHandler) List(c *fiber.Ctx) error {
	q, err := parseMemberQuery(c)
	if err != nil {
		return writeError(c, err)
	}

	if c.Query("format") == "xlsx" {
		return h.exportXLSX(c, q)
	}

	items, total, err := h.members.List(c.UserContext(), q)
	if err != nil {
		return writeError(c, err)
	}

	actor, _ := middleware.ActorFrom(c)
	unmasked := actor.IsOwner()

	out := make([]dto.MemberListItem, 0, len(items))
	for _, i := range items {
		item := dto.ToMemberListItem(i)
		if !unmasked {
			item.Email = domainmember.MaskEmail(item.Email)
			if item.Phone != nil {
				masked := domainmember.MaskPhone(*item.Phone)
				item.Phone = &masked
			}
		}
		out = append(out, item)
	}
	normalized, _ := usecasemember.NormalizeQuery(q)
	return response.List(c, out, normalized.Page, normalized.Limit, total)
}

func (h *MemberHandler) exportXLSX(c *fiber.Ctx, q domainmember.Query) error {
	actor, _ := middleware.ActorFrom(c)
	if !authz.Allowed(actor.Roles, authz.MembersExport) {
		return response.Forbidden(c, "You do not have permission to export members", "FORBIDDEN")
	}
	rows, err := h.exporter.Export(c.UserContext(), actor, q)
	if err != nil {
		return writeError(c, err)
	}
	file, err := export.MembersXLSX(rows)
	if err != nil {
		return writeError(c, err)
	}
	// Audit trail (PDPA): who exported what. Persisted audit log arrives with P2.
	log.Printf("[AUDIT] members export by erp_user_id=%d rows=%d search=%q status=%q tier=%d",
		actor.ERPUserID, len(rows), q.Search, q.Status, q.TierID)

	name := fmt.Sprintf("members-%s.xlsx", time.Now().In(bangkok).Format("20060102-1504"))
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+name+`"`)
	return c.Send(file)
}

// Get serves GET /members/:id.
func (h *MemberHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	d, err := h.members.Get(c.UserContext(), id)
	if err != nil {
		return writeError(c, err)
	}
	res := dto.ToMemberDetail(d)
	actor, _ := middleware.ActorFrom(c)
	if !actor.IsOwner() {
		res.Email = domainmember.MaskEmail(res.Email)
		if res.Phone != nil {
			masked := domainmember.MaskPhone(*res.Phone)
			res.Phone = &masked
		}
		res.BirthDate = nil
	}
	return response.OK(c, res)
}

// Tiers serves GET /tiers.
func (h *MemberHandler) Tiers(c *fiber.Ctx) error {
	tiers, err := h.tiers.List(c.UserContext())
	if err != nil {
		return writeError(c, err)
	}
	out := make([]dto.TierResponse, 0, len(tiers))
	for _, t := range tiers {
		out = append(out, dto.ToTierResponse(t))
	}
	return response.OK(c, out)
}

func parseMemberQuery(c *fiber.Ctx) (domainmember.Query, error) {
	q := domainmember.Query{
		Search: c.Query("search"),
		Status: c.Query("status"),
	}
	var err error
	if q.Page, err = parseOptionalInt(c.Query("page"), "page"); err != nil {
		return q, err
	}
	if q.Limit, err = parseOptionalInt(c.Query("limit"), "limit"); err != nil {
		return q, err
	}
	if v := c.Query("tierId"); v != "" {
		if q.TierID, err = parseID(v, "tierId"); err != nil {
			return q, err
		}
	}
	if q.RegisteredFrom, err = parseDate(c.Query("registeredFrom"), "registeredFrom", false); err != nil {
		return q, err
	}
	// registeredTo is an inclusive date; the repository bound is exclusive.
	if q.RegisteredTo, err = parseDate(c.Query("registeredTo"), "registeredTo", true); err != nil {
		return q, err
	}
	if q.MinPoints, err = parseOptionalPoints(c.Query("minPoints"), "minPoints"); err != nil {
		return q, err
	}
	if q.MaxPoints, err = parseOptionalPoints(c.Query("maxPoints"), "maxPoints"); err != nil {
		return q, err
	}
	if q.MinPoints != nil && q.MaxPoints != nil && *q.MinPoints > *q.MaxPoints {
		return q, domain.NewValidationError("minPoints", "must not exceed maxPoints")
	}
	if v := c.Query("inactiveDays"); v != "" {
		days, err := strconv.Atoi(v)
		if err != nil || days < 1 || days > 3650 {
			return q, domain.NewValidationError("inactiveDays", "must be an integer between 1 and 3650")
		}
		before := time.Now().AddDate(0, 0, -days)
		q.InactiveBefore = &before
	}
	return q, nil
}

// parseOptionalPoints reads a non-negative point bound; empty means no bound.
func parseOptionalPoints(v, field string) (*int64, error) {
	if v == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return nil, domain.NewValidationError(field, "must be a non-negative integer")
	}
	return &n, nil
}

func parseOptionalInt(v, field string) (int, error) {
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, domain.NewValidationError(field, "must be a positive integer")
	}
	return n, nil
}

func parseID(v, field string) (uint, error) {
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil || n == 0 {
		return 0, domain.NewValidationError(field, "must be a positive integer")
	}
	return uint(n), nil
}

// parseDate parses YYYY-MM-DD in Bangkok time. endOfDay returns the start of
// the next day so the date itself is included by an exclusive bound.
func parseDate(v, field string, endOfDay bool) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", v, bangkok)
	if err != nil {
		return nil, domain.NewValidationError(field, "must be a date in YYYY-MM-DD format")
	}
	if endOfDay {
		t = t.AddDate(0, 0, 1)
	}
	return &t, nil
}
