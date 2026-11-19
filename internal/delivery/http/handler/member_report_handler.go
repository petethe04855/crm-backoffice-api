package handler

import (
	"time"

	"backoffice-api/pkg/response"

	"crm-core/domain"
	"crm-core/usecase/memberreport"

	"github.com/gofiber/fiber/v2"
)

// MemberReportHandler serves the CRM member part of the dashboard (plan §6.3 B).
type MemberReportHandler struct {
	uc memberreport.Usecase
}

// NewMemberReportHandler builds the handler.
func NewMemberReportHandler(uc memberreport.Usecase) *MemberReportHandler {
	return &MemberReportHandler{uc: uc}
}

type memberReportMonth struct {
	Month      string `json:"month"`
	NewMembers int64  `json:"newMembers"`
	Earned     int64  `json:"pointsEarned"`
	Redeemed   int64  `json:"pointsRedeemed"`
	Expired    int64  `json:"pointsExpired"`
}

type memberReportResponse struct {
	DateFrom          string              `json:"dateFrom"`
	DateTo            string              `json:"dateTo"`
	TotalMembers      int64               `json:"totalMembers"`
	NewMembers        int64               `json:"newMembers"`
	ActiveMembers     int64               `json:"activeMembers"`
	PointsEarned      int64               `json:"pointsEarned"`
	PointsRedeemed    int64               `json:"pointsRedeemed"`
	PointsExpired     int64               `json:"pointsExpired"`
	PointsAdjusted    int64               `json:"pointsAdjusted"`
	PointsRefunded    int64               `json:"pointsRefunded"`
	OutstandingPoints int64               `json:"outstandingPoints"`
	Months            []memberReportMonth `json:"months"`
}

// Members serves GET /reports/members?dateFrom=YYYY-MM-DD&dateTo=YYYY-MM-DD
// (inclusive Bangkok days; default the last 90 days).
func (h *MemberReportHandler) Members(c *fiber.Ctx) error {
	from, err := parseDate(c.Query("dateFrom"), "dateFrom", false)
	if err != nil {
		return writeError(c, err)
	}
	to, err := parseDate(c.Query("dateTo"), "dateTo", true)
	if err != nil {
		return writeError(c, err)
	}
	now := time.Now().In(bangkok)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, bangkok)
	if to == nil {
		end := today.AddDate(0, 0, 1)
		to = &end
	}
	if from == nil {
		start := to.AddDate(0, 0, -90)
		from = &start
	}
	if !from.Before(*to) {
		return writeError(c, domain.NewValidationError("dateFrom", "must not be after dateTo"))
	}
	r, err := h.uc.Build(c.UserContext(), *from, *to)
	if err != nil {
		return writeError(c, err)
	}
	out := memberReportResponse{
		DateFrom: from.In(bangkok).Format("2006-01-02"), DateTo: to.In(bangkok).AddDate(0, 0, -1).Format("2006-01-02"),
		TotalMembers: r.TotalMembers, NewMembers: r.NewMembers, ActiveMembers: r.ActiveMembers,
		PointsEarned: r.PointsEarned, PointsRedeemed: r.PointsRedeemed, PointsExpired: r.PointsExpired,
		PointsAdjusted: r.PointsAdjusted, PointsRefunded: r.PointsRefunded, OutstandingPoints: r.OutstandingPoint,
		Months: make([]memberReportMonth, 0, len(r.Months)),
	}
	for _, m := range r.Months {
		out.Months = append(out.Months, memberReportMonth{Month: m.Month, NewMembers: m.NewMembers,
			Earned: m.Earned, Redeemed: m.Redeemed, Expired: m.Expired})
	}
	return response.OK(c, out)
}
