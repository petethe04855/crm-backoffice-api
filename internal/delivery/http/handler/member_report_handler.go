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

// memberReportGender is one slice of the gender split of the members who
// registered inside the range.
type memberReportGender struct {
	Gender       string  `json:"gender"`
	Members      int64   `json:"members"`
	SharePercent float64 `json:"sharePercent"`
}

// memberReportAge is one age band of the members who registered inside the
// range; the "unknown" band holds the members with no birth date.
type memberReportAge struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	Members      int64   `json:"members"`
	SharePercent float64 `json:"sharePercent"`
}

// memberReportReward is one reward redeemed in the range (cancelled excluded).
type memberReportReward struct {
	RewardID     uint    `json:"rewardId"`
	RewardName   string  `json:"rewardName"`
	Redemptions  int64   `json:"redemptions"`
	Qty          int64   `json:"qty"`
	Points       int64   `json:"points"`
	SharePercent float64 `json:"sharePercent"`
}

// memberReportFirstReward is the reward a member redeemed first, counted once
// per member.
type memberReportFirstReward struct {
	RewardID     uint    `json:"rewardId"`
	RewardName   string  `json:"rewardName"`
	Members      int64   `json:"members"`
	SharePercent float64 `json:"sharePercent"`
}

// memberReportFirstSKU is the SKU a member typed a product code for first,
// counted once per member.
type memberReportFirstSKU struct {
	SKUCode      string  `json:"skuCode"`
	SKUName      string  `json:"skuName"`
	Members      int64   `json:"members"`
	SharePercent float64 `json:"sharePercent"`
}

type memberReportResponse struct {
	DateFrom          string                 `json:"dateFrom"`
	DateTo            string                 `json:"dateTo"`
	TotalMembers      int64                  `json:"totalMembers"`
	NewMembers        int64                  `json:"newMembers"`
	ActiveMembers     int64                  `json:"activeMembers"`
	PointsEarned      int64                  `json:"pointsEarned"`
	PointsRedeemed    int64                  `json:"pointsRedeemed"`
	PointsExpired     int64                  `json:"pointsExpired"`
	PointsAdjusted    int64                  `json:"pointsAdjusted"`
	PointsRefunded    int64                  `json:"pointsRefunded"`
	OutstandingPoints int64                  `json:"outstandingPoints"`
	Months            []memberReportMonth    `json:"months"`
	Genders           []memberReportGender   `json:"genders"`
	AgeBuckets        []memberReportAge      `json:"ageBuckets"`
	FirstSKUs         []memberReportFirstSKU   `json:"firstSkus"`
	FirstRewards      []memberReportFirstReward `json:"firstRewards"`
	Redemptions       int64                  `json:"redemptions"`
	RedemptionMembers int64                  `json:"redemptionMembers"`
	Rewards           []memberReportReward   `json:"rewards"`
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
	out.Genders = make([]memberReportGender, 0, len(r.Genders))
	for _, g := range r.Genders {
		out.Genders = append(out.Genders, memberReportGender{Gender: g.Gender, Members: g.Members,
			SharePercent: g.SharePercent})
	}
	out.AgeBuckets = make([]memberReportAge, 0, len(r.AgeBuckets))
	for _, a := range r.AgeBuckets {
		out.AgeBuckets = append(out.AgeBuckets, memberReportAge{Key: a.Key, Label: a.Label,
			Members: a.Members, SharePercent: a.SharePercent})
	}
	out.FirstRewards = make([]memberReportFirstReward, 0, len(r.FirstRewards))
	for _, fr := range r.FirstRewards {
		out.FirstRewards = append(out.FirstRewards, memberReportFirstReward{RewardID: fr.RewardID,
			RewardName: fr.RewardName, Members: fr.Members, SharePercent: fr.SharePercent})
	}
	out.FirstSKUs = make([]memberReportFirstSKU, 0, len(r.FirstSKUs))
	for _, s := range r.FirstSKUs {
		out.FirstSKUs = append(out.FirstSKUs, memberReportFirstSKU{SKUCode: s.SKUCode, SKUName: s.SKUName,
			Members: s.Members, SharePercent: s.SharePercent})
	}
	out.Redemptions, out.RedemptionMembers = r.Redemptions, r.RedemptionMembers
	out.Rewards = make([]memberReportReward, 0, len(r.Rewards))
	for _, rw := range r.Rewards {
		out.Rewards = append(out.Rewards, memberReportReward{RewardID: rw.RewardID, RewardName: rw.RewardName,
			Redemptions: rw.Redemptions, Qty: rw.Qty, Points: rw.Points, SharePercent: rw.SharePercent})
	}
	return response.OK(c, out)
}
