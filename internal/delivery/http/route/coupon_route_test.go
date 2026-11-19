package route

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	domainmember "crm-core/domain/member"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// freeCoupon is a valid template that costs no points. Paid claims happen on
// the LINE side and are covered by the crm-core usecase tests; here the
// backoffice grant path exercises the same Claim.
func freeCoupon(quota int) map[string]any {
	return map[string]any{
		"name": "ของขวัญสมาชิกใหม่", "benefitType": "amount_off", "benefitValue": 50,
		"pointsCost": 0, "expiryMode": "days_after_claim", "validDays": 30,
		"totalQuota": quota, "limitPerMember": 1, "channel": "store", "codeType": "unique",
	}
}

func (e *testEnv) createCoupon(t *testing.T, body map[string]any, activate bool) uint {
	t.Helper()
	owner := token(t, 1, "owner")
	res := e.do(t, http.MethodPost, "/api/v1/crm/coupons", owner, body)
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	var c struct {
		ID     uint   `json:"id"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &c))
	require.Equal(t, "draft", c.Status)
	if activate {
		res = e.do(t, http.MethodPost, fmt.Sprintf("/api/v1/crm/coupons/%d/activate", c.ID), owner, nil)
		require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	}
	return c.ID
}

func (e *testEnv) memberIDs(t *testing.T) []uint {
	t.Helper()
	var ms []domainmember.Member
	require.NoError(t, e.db.Order("id").Find(&ms).Error)
	ids := make([]uint, 0, len(ms))
	for _, m := range ms {
		ids = append(ids, m.ID)
	}
	return ids
}

type grantedCoupon struct {
	ID     uint   `json:"id"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

func (e *testEnv) grant(t *testing.T, memberID, couponID uint) (int, grantedCoupon, string) {
	t.Helper()
	res := e.do(t, http.MethodPost, fmt.Sprintf("/api/v1/crm/members/%d/coupons", memberID), token(t, 1, "owner"),
		map[string]any{"couponId": couponID})
	var mc grantedCoupon
	if res.Status == http.StatusCreated {
		require.NoError(t, json.Unmarshal(res.Body.Data, &mc))
	}
	return res.Status, mc, res.Body.Error.Code
}

func TestCouponRBAC(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	id := e.createCoupon(t, freeCoupon(10), true)
	member := e.memberIDs(t)[0]
	_, mc, _ := e.grant(t, member, id)

	sales := token(t, 2, "sales")
	warehouse := token(t, 3, "warehouse")
	accountant := token(t, 6, "accountant")
	base := fmt.Sprintf("/api/v1/crm/coupons/%d", id)

	cases := []struct {
		name   string
		method string
		path   string
		bearer string
		body   any
		status int
	}{
		{"sales lists", http.MethodGet, "/api/v1/crm/coupons", sales, nil, http.StatusOK},
		{"sales views", http.MethodGet, base, sales, nil, http.StatusOK},
		{"sales sees holders", http.MethodGet, base + "/holders", sales, nil, http.StatusOK},
		{"sales looks a code up", http.MethodGet, "/api/v1/crm/coupons/codes/" + mc.Code, sales, nil, http.StatusOK},
		{"sales cannot create", http.MethodPost, "/api/v1/crm/coupons", sales, freeCoupon(1), http.StatusForbidden},
		{"sales cannot pause", http.MethodPost, base + "/pause", sales, nil, http.StatusForbidden},
		{"sales cannot cancel", http.MethodPost, fmt.Sprintf("/api/v1/crm/member-coupons/%d/cancel", mc.ID), sales, map[string]string{"reason": "ทดสอบ"}, http.StatusForbidden},
		{"sales cannot grant", http.MethodPost, fmt.Sprintf("/api/v1/crm/members/%d/coupons", member), sales, map[string]any{"couponId": id}, http.StatusForbidden},
		{"sales sees member coupons", http.MethodGet, fmt.Sprintf("/api/v1/crm/members/%d/coupons", member), sales, nil, http.StatusOK},
		{"accountant views", http.MethodGet, base, accountant, nil, http.StatusOK},
		{"accountant cannot use", http.MethodPost, "/api/v1/crm/coupons/codes/" + mc.Code + "/use", accountant, map[string]string{"usedRef": "INV-1"}, http.StatusForbidden},
		{"warehouse sees nothing", http.MethodGet, "/api/v1/crm/coupons", warehouse, nil, http.StatusForbidden},
		{"sales uses at the store", http.MethodPost, "/api/v1/crm/coupons/codes/" + mc.Code + "/use", sales, map[string]string{"usedRef": "INV-1"}, http.StatusOK},
	}
	for _, c := range cases {
		res := e.do(t, c.method, c.path, c.bearer, c.body)
		require.Equal(t, c.status, res.Status, "%s: %s", c.name, res.Raw)
	}
}

func TestCouponGrantUseAndCancelFlow(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	owner := token(t, 1, "owner")
	sales := token(t, 2, "sales")
	members := e.memberIDs(t)

	draft := e.createCoupon(t, freeCoupon(2), false)
	status, _, code := e.grant(t, members[0], draft)
	require.Equal(t, http.StatusConflict, status)
	require.Equal(t, "COUPON_NOT_ACTIVE", code)

	id := e.createCoupon(t, freeCoupon(2), true)
	status, first, _ := e.grant(t, members[0], id)
	require.Equal(t, http.StatusCreated, status)
	require.Equal(t, "available", first.Status)

	status, _, code = e.grant(t, members[0], id)
	require.Equal(t, http.StatusConflict, status)
	require.Equal(t, "COUPON_LIMIT_REACHED", code)

	// members[1] is suspended in the seed.
	status, _, code = e.grant(t, members[1], id)
	require.Equal(t, http.StatusConflict, status)
	require.Equal(t, "MEMBER_NOT_ALLOWED", code)

	status, second, _ := e.grant(t, members[2], id)
	require.Equal(t, http.StatusCreated, status)
	status, _, code = e.grant(t, members[1], id)
	require.Equal(t, http.StatusConflict, status)
	require.Equal(t, "COUPON_QUOTA_REACHED", code, "quota is checked before the member")

	res := e.do(t, http.MethodGet, fmt.Sprintf("/api/v1/crm/coupons/%d", id), owner, nil)
	var detail struct {
		ClaimedCount   int  `json:"claimedCount"`
		RemainingQuota *int `json:"remainingQuota"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &detail))
	require.Equal(t, 2, detail.ClaimedCount)
	require.NotNil(t, detail.RemainingQuota)
	require.Equal(t, 0, *detail.RemainingQuota)

	// Store lookup and use (lower-case input is accepted).
	res = e.do(t, http.MethodGet, "/api/v1/crm/coupons/codes/"+strings.ToLower(first.Code), sales, nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var view struct {
		Usable bool `json:"usable"`
		Member struct {
			MemberCode string `json:"memberCode"`
		} `json:"member"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &view))
	require.True(t, view.Usable)
	require.Equal(t, "M2611-000001", view.Member.MemberCode)

	usePath := "/api/v1/crm/coupons/codes/" + first.Code + "/use"
	res = e.do(t, http.MethodPost, usePath, sales, map[string]string{"usedRef": ""})
	require.Equal(t, http.StatusBadRequest, res.Status)
	res = e.do(t, http.MethodPost, usePath, sales, map[string]string{"usedRef": "INV-001"})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	res = e.do(t, http.MethodPost, usePath, sales, map[string]string{"usedRef": "INV-001"})
	require.Equal(t, http.StatusOK, res.Status, "same bill again is idempotent")
	res = e.do(t, http.MethodPost, usePath, sales, map[string]string{"usedRef": "INV-002"})
	require.Equal(t, http.StatusConflict, res.Status)
	require.Equal(t, "COUPON_ALREADY_USED", res.Body.Error.Code)

	// Cancel: a reason is required, used coupons cannot be cancelled.
	cancel := func(mcID uint, reason string) apiResult {
		return e.do(t, http.MethodPost, fmt.Sprintf("/api/v1/crm/member-coupons/%d/cancel", mcID), owner, map[string]string{"reason": reason})
	}
	require.Equal(t, http.StatusBadRequest, cancel(second.ID, "").Status)
	res = cancel(first.ID, "ลูกค้าขอยกเลิก")
	require.Equal(t, http.StatusConflict, res.Status)
	require.Equal(t, "COUPON_ALREADY_USED", res.Body.Error.Code)
	res = cancel(second.ID, "ออกผิดคน")
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	// The freed quota slot can be claimed again; the cancelled coupon no longer
	// counts against the member's limit.
	status, _, _ = e.grant(t, members[2], id)
	require.Equal(t, http.StatusCreated, status)

	res = e.do(t, http.MethodGet, fmt.Sprintf("/api/v1/crm/coupons/%d/holders?status=cancelled", id), owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.EqualValues(t, 1, res.Body.Meta.Total)

	res = e.do(t, http.MethodGet, fmt.Sprintf("/api/v1/crm/members/%d/coupons", members[0]), owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.EqualValues(t, 1, res.Body.Meta.Total)

	res = e.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/crm/coupons/%d", id), owner, nil)
	require.Equal(t, http.StatusConflict, res.Status, "claimed coupons cannot be deleted")
	res = e.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/crm/coupons/%d", draft), owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
}

func TestCouponValidationAndLookupErrors(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	owner := token(t, 1, "owner")

	bad := freeCoupon(1)
	bad["benefitType"], bad["benefitValue"] = "percent_off", 10 // no maxDiscount
	res := e.do(t, http.MethodPost, "/api/v1/crm/coupons", owner, bad)
	require.Equal(t, http.StatusBadRequest, res.Status)

	bad = freeCoupon(1)
	bad["channel"] = "shopee" // marketplaces need a shared code
	res = e.do(t, http.MethodPost, "/api/v1/crm/coupons", owner, bad)
	require.Equal(t, http.StatusBadRequest, res.Status)

	paid := freeCoupon(5)
	paid["pointsCost"] = 100
	paidID := e.createCoupon(t, paid, true)
	status, _, code := e.grant(t, e.memberIDs(t)[0], paidID)
	require.Equal(t, http.StatusBadRequest, status, "only free coupons can be granted")
	require.Equal(t, "VALIDATION_ERROR", code)

	res = e.do(t, http.MethodGet, "/api/v1/crm/coupons/codes/NOPE123456", owner, nil)
	require.Equal(t, http.StatusNotFound, res.Status)
	res = e.do(t, http.MethodGet, "/api/v1/crm/coupons?month=2026-13", owner, nil)
	require.Equal(t, http.StatusBadRequest, res.Status)
	res = e.do(t, http.MethodPost, fmt.Sprintf("/api/v1/crm/coupons/%d/activate", paidID), owner, nil)
	require.Equal(t, http.StatusConflict, res.Status, "already active")
}

func TestCouponHoldersExportMasksUnlessOwner(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	id := e.createCoupon(t, freeCoupon(0), true)
	status, _, _ := e.grant(t, e.memberIDs(t)[0], id)
	require.Equal(t, http.StatusCreated, status)

	read := func(bearer string) [][]string {
		res := e.do(t, http.MethodGet, fmt.Sprintf("/api/v1/crm/coupons/%d/holders?format=xlsx", id), bearer, nil)
		require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
		require.Contains(t, res.Header.Get("Content-Disposition"), fmt.Sprintf("coupon-%d-holders-", id))
		f, err := excelize.OpenReader(bytes.NewReader(res.Raw))
		require.NoError(t, err)
		defer f.Close()
		rows, err := f.GetRows("Holders")
		require.NoError(t, err)
		require.Len(t, rows, 2)
		return rows
	}
	ownerRows := read(token(t, 1, "owner"))
	require.Equal(t, "nopharat@gmail.com", ownerRows[1][3])
	salesRows := read(token(t, 2, "sales"))
	require.NotEqual(t, "nopharat@gmail.com", salesRows[1][3])
	require.Contains(t, salesRows[1][3], "@gmail.com")
	require.NotContains(t, salesRows[1][4], "71166", "phone is masked")

}
