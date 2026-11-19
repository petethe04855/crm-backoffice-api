package route

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"backoffice-api/internal/client/erp"

	domainreward "crm-core/domain/reward"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// fakeRewardERP is both the reward ERP port and the SKU search of the form.
type fakeRewardERP struct {
	stock    map[uint]int
	issued   map[string]string
	shortNow bool
}

func newFakeRewardERP() *fakeRewardERP {
	return &fakeRewardERP{stock: map[uint]int{101: 5}, issued: map[string]string{}}
}

func (f *fakeRewardERP) Stock(_ context.Context, ids []uint, wh uint) ([]domainreward.StockLevel, error) {
	out := make([]domainreward.StockLevel, 0, len(ids))
	for _, id := range ids {
		out = append(out, domainreward.StockLevel{SKUID: id, WarehouseID: wh, AvailableQty: f.stock[id]})
	}
	return out, nil
}

func (f *fakeRewardERP) IssueGoods(_ context.Context, gi domainreward.GoodsIssue) (string, error) {
	if ref, ok := f.issued[gi.IdempotencyKey]; ok {
		return ref, nil
	}
	if f.shortNow {
		return "", domainreward.ErrInsufficientERP
	}
	ref := fmt.Sprintf("CRMGI-%06d", len(f.issued)+1)
	f.issued[gi.IdempotencyKey] = ref
	return ref, nil
}

func (f *fakeRewardERP) SearchSKUs(_ context.Context, search string, _ uint, _, _ int) ([]erp.SKU, int64, error) {
	return []erp.SKU{{ID: 101, Code: "CHK-01", Name: "ขนมไก่ " + search, AvailableQty: 5}}, 1, nil
}

func rewardBody() map[string]any {
	return map[string]any{"name": "ขนมไก่ 1 ถุง", "erpSkuId": 101, "skuCode": "CHK-01", "skuName": "ขนมไก่", "pointsCost": 40}
}

// seedRedemption inserts a redemption directly (members redeem from the LINE
// side). Points are 0 so cancelling needs no refund in this single-connection
// test DB; refunds are covered by the crm-core usecase tests.
func (e *testEnv) seedRedemption(t *testing.T, rewardID, memberID uint, status string) domainreward.Redemption {
	t.Helper()
	sku := uint(101)
	rd := domainreward.Redemption{
		RedemptionNo: fmt.Sprintf("RD2611-%06d", time.Now().UnixNano()%1_000_000), MemberID: memberID, RewardID: rewardID,
		Qty: 1, Status: status, RewardName: "ขนมไก่ 1 ถุง", SKUCode: "CHK-01", ERPSKUID: &sku, WarehouseID: 1,
		ShipName: "สมชาย", ShipPhone: "0812345678", ShipAddress: "1 ถนนสุขุมวิท", ShipProvince: "กรุงเทพมหานคร",
	}
	require.NoError(t, e.db.Create(&rd).Error)
	return rd
}

func (e *testEnv) createReward(t *testing.T, activate bool) uint {
	t.Helper()
	owner := token(t, 1, "owner")
	res := e.do(t, http.MethodPost, "/api/v1/crm/rewards", owner, rewardBody())
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	var r struct {
		ID uint `json:"id"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &r))
	if activate {
		res = e.do(t, http.MethodPost, fmt.Sprintf("/api/v1/crm/rewards/%d/status", r.ID), owner, map[string]string{"status": "active"})
		require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	}
	return r.ID
}

func TestRewardRBAC(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	id := e.createReward(t, true)
	rd := e.seedRedemption(t, id, e.memberIDs(t)[0], domainreward.RedemptionPending)

	sales := token(t, 2, "sales")
	wh := token(t, 3, "warehouse")
	acc := token(t, 6, "accountant")
	cases := []struct {
		name   string
		method string
		path   string
		bearer string
		body   any
		status int
	}{
		{"sales sees rewards", http.MethodGet, "/api/v1/crm/rewards", sales, nil, http.StatusOK},
		{"accountant sees a reward", http.MethodGet, fmt.Sprintf("/api/v1/crm/rewards/%d", id), acc, nil, http.StatusOK},
		{"warehouse sees categories", http.MethodGet, "/api/v1/crm/reward-categories", wh, nil, http.StatusOK},
		{"sales cannot create", http.MethodPost, "/api/v1/crm/rewards", sales, rewardBody(), http.StatusForbidden},
		{"warehouse cannot search ERP", http.MethodGet, "/api/v1/crm/erp/skus?search=x", wh, nil, http.StatusForbidden},
		{"sales cannot see redemptions", http.MethodGet, "/api/v1/crm/redemptions", sales, nil, http.StatusForbidden},
		{"warehouse sees redemptions", http.MethodGet, "/api/v1/crm/redemptions", wh, nil, http.StatusOK},
		{"warehouse approves", http.MethodPost, fmt.Sprintf("/api/v1/crm/redemptions/%d/approve", rd.ID), wh, nil, http.StatusOK},
		{"accountant cannot pack", http.MethodPost, fmt.Sprintf("/api/v1/crm/redemptions/%d/pack", rd.ID), acc, nil, http.StatusForbidden},
		{"sales sees member redemptions", http.MethodGet, fmt.Sprintf("/api/v1/crm/members/%d/redemptions", rd.MemberID), sales, nil, http.StatusOK},
		{"owner searches ERP", http.MethodGet, "/api/v1/crm/erp/skus?search=x", token(t, 1, "owner"), nil, http.StatusOK},
	}
	for _, c := range cases {
		res := e.do(t, c.method, c.path, c.bearer, c.body)
		require.Equal(t, c.status, res.Status, "%s: %s", c.name, res.Raw)
	}
}

func TestRewardCatalogueAPI(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	owner := token(t, 1, "owner")

	res := e.do(t, http.MethodPost, "/api/v1/crm/reward-categories", owner, map[string]any{"name": "ขนม"})
	require.Equal(t, http.StatusCreated, res.Status)
	res = e.do(t, http.MethodPost, "/api/v1/crm/reward-categories", owner, map[string]any{"name": "ขนม"})
	require.Equal(t, http.StatusConflict, res.Status, "names are unique")

	bad := rewardBody()
	bad["pointsCost"] = 0
	res = e.do(t, http.MethodPost, "/api/v1/crm/rewards", owner, bad)
	require.Equal(t, http.StatusBadRequest, res.Status)
	bad = rewardBody()
	bad["type"] = "digital"
	res = e.do(t, http.MethodPost, "/api/v1/crm/rewards", owner, bad)
	require.Equal(t, http.StatusBadRequest, res.Status)
	require.Equal(t, "UNSUPPORTED_REWARD_TYPE", res.Body.Error.Code)

	id := e.createReward(t, true)
	res = e.do(t, http.MethodGet, fmt.Sprintf("/api/v1/crm/rewards/%d", id), owner, nil)
	var v struct {
		ERPAvailableQty *int `json:"erpAvailableQty"`
		RemainingQuota  int  `json:"remainingQuota"`
		StockStale      bool `json:"stockStale"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &v))
	require.Nil(t, v.ERPAvailableQty, "not synced yet")
	require.Zero(t, v.RemainingQuota)
	require.True(t, v.StockStale)

	// The SKU locks after a redemption.
	e.seedRedemption(t, id, e.memberIDs(t)[0], domainreward.RedemptionPending)
	require.NoError(t, e.db.Model(&domainreward.Reward{}).Where("id = ?", id).Update("redeemed_count", 1).Error)
	changed := rewardBody()
	changed["erpSkuId"] = 202
	res = e.do(t, http.MethodPatch, fmt.Sprintf("/api/v1/crm/rewards/%d", id), owner, changed)
	require.Equal(t, http.StatusConflict, res.Status)
	require.Equal(t, "REWARD_SKU_LOCKED", res.Body.Error.Code)

	res = e.do(t, http.MethodGet, "/api/v1/crm/rewards?lowStock=true", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.EqualValues(t, 1, res.Body.Meta.Total, "never-synced rewards count as low stock")
}

func TestRedemptionFlowAPI(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	wh := token(t, 3, "warehouse")
	id := e.createReward(t, true)
	rd := e.seedRedemption(t, id, e.memberIDs(t)[0], domainreward.RedemptionPending)
	path := func(action string) string { return fmt.Sprintf("/api/v1/crm/redemptions/%d/%s", rd.ID, action) }

	res := e.do(t, http.MethodPost, path("pack"), wh, nil)
	require.Equal(t, http.StatusConflict, res.Status)
	require.Equal(t, "INVALID_REDEMPTION_STATUS", res.Body.Error.Code)

	require.Equal(t, http.StatusOK, e.do(t, http.MethodPost, path("approve"), wh, nil).Status)
	require.Equal(t, http.StatusOK, e.do(t, http.MethodPost, path("pack"), wh, nil).Status)
	require.Equal(t, http.StatusBadRequest, e.do(t, http.MethodPost, path("ship"), wh, map[string]string{"trackingNo": ""}).Status)

	e.erp.shortNow = true
	res = e.do(t, http.MethodPost, path("ship"), wh, map[string]string{"trackingNo": "TH123"})
	require.Equal(t, http.StatusConflict, res.Status)
	require.Equal(t, "INSUFFICIENT_ERP_STOCK", res.Body.Error.Code)
	e.erp.shortNow = false

	res = e.do(t, http.MethodPost, path("ship"), wh, map[string]string{"trackingNo": "TH123"})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var shipped struct {
		Status        string `json:"status"`
		StockIssueRef string `json:"stockIssueRef"`
		TrackingNo    string `json:"trackingNo"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &shipped))
	require.Equal(t, "shipped", shipped.Status)
	require.Equal(t, "CRMGI-000001", shipped.StockIssueRef)

	res = e.do(t, http.MethodPost, path("cancel"), wh, map[string]string{"reason": "เปลี่ยนใจ"})
	require.Equal(t, http.StatusConflict, res.Status, "shipped cannot be cancelled")
	require.Equal(t, http.StatusOK, e.do(t, http.MethodPost, path("deliver"), wh, nil).Status)

	other := e.seedRedemption(t, id, e.memberIDs(t)[2], domainreward.RedemptionApproved)
	res = e.do(t, http.MethodPost, fmt.Sprintf("/api/v1/crm/redemptions/%d/cancel", other.ID), wh, map[string]string{"reason": ""})
	require.Equal(t, http.StatusBadRequest, res.Status)
	res = e.do(t, http.MethodPost, fmt.Sprintf("/api/v1/crm/redemptions/%d/cancel", other.ID), wh, map[string]string{"reason": "สินค้าหมด"})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	res = e.do(t, http.MethodGet, "/api/v1/crm/redemptions?status=cancelled", wh, nil)
	require.EqualValues(t, 1, res.Body.Meta.Total)
	res = e.do(t, http.MethodGet, "/api/v1/crm/redemptions?status=nope", wh, nil)
	require.Equal(t, http.StatusBadRequest, res.Status)
}

func TestRedemptionsExportMasksUnlessOwner(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	id := e.createReward(t, true)
	e.seedRedemption(t, id, e.memberIDs(t)[0], domainreward.RedemptionPending)

	read := func(bearer string) []string {
		res := e.do(t, http.MethodGet, "/api/v1/crm/redemptions?format=xlsx", bearer, nil)
		require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
		f, err := excelize.OpenReader(bytes.NewReader(res.Raw))
		require.NoError(t, err)
		defer f.Close()
		rows, err := f.GetRows("Redemptions")
		require.NoError(t, err)
		require.Len(t, rows, 2)
		return rows[1]
	}
	owner := read(token(t, 1, "owner"))
	require.Equal(t, "0812345678", owner[10])
	require.Equal(t, "1 ถนนสุขุมวิท", owner[11])
	wh := read(token(t, 3, "warehouse"))
	require.NotEqual(t, "0812345678", wh[10])
	require.Equal(t, "***", wh[11])
	require.Equal(t, "กรุงเทพมหานคร", wh[12], "province stays for reporting")
}
