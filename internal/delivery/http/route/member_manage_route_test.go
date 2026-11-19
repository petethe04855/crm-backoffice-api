package route

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	domainmember "crm-core/domain/member"

	"github.com/stretchr/testify/require"
)

// firstMemberID returns the id of the first seeded member.
func (e *testEnv) firstMemberID(t *testing.T) uint {
	t.Helper()
	var m domainmember.Member
	require.NoError(t, e.db.Order("id").First(&m).Error)
	return m.ID
}

func TestMemberManageRBAC(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	base := fmt.Sprintf("/api/v1/crm/members/%d", e.firstMemberID(t))

	owner := token(t, 1, "owner")
	sales := token(t, 2, "sales")
	warehouse := token(t, 3, "warehouse")
	accountant := token(t, 6, "accountant")

	cases := []struct {
		name   string
		method string
		path   string
		bearer string
		body   any
		status int
	}{
		{"sales reads tags", http.MethodGet, base + "/tags", sales, nil, http.StatusOK},
		{"sales reads notes", http.MethodGet, base + "/notes", sales, nil, http.StatusOK},
		{"sales writes notes", http.MethodPost, base + "/notes", sales, map[string]string{"body": "ลูกค้าโทรมาสอบถามแต้ม"}, http.StatusCreated},
		{"sales cannot suspend", http.MethodPatch, base, sales, map[string]string{"status": "suspended"}, http.StatusForbidden},
		{"sales cannot unlink LINE", http.MethodDelete, base + "/line", sales, nil, http.StatusForbidden},
		{"sales cannot tag", http.MethodPost, base + "/tags", sales, map[string]string{"tag": "vip"}, http.StatusForbidden},
		{"sales cannot untag", http.MethodDelete, base + "/tags/vip", sales, nil, http.StatusForbidden},
		{"sales cannot see ledger", http.MethodGet, base + "/points", sales, nil, http.StatusForbidden},
		{"sales cannot adjust", http.MethodPost, base + "/points/adjust", sales, map[string]any{"points": 10, "reason": "ทดสอบ"}, http.StatusForbidden},
		{"accountant sees ledger", http.MethodGet, base + "/points", accountant, nil, http.StatusOK},
		{"accountant cannot adjust", http.MethodPost, base + "/points/adjust", accountant, map[string]any{"points": 10, "reason": "ทดสอบ"}, http.StatusForbidden},
		{"accountant cannot read notes", http.MethodGet, base + "/notes", accountant, nil, http.StatusForbidden},
		{"warehouse cannot read member", http.MethodGet, base + "/tags", warehouse, nil, http.StatusForbidden},
		{"owner reads ledger", http.MethodGet, base + "/points", owner, nil, http.StatusOK},
	}
	for _, c := range cases {
		res := e.do(t, c.method, c.path, c.bearer, c.body)
		require.Equal(t, c.status, res.Status, "%s: %s", c.name, res.Raw)
	}
}

func TestPointsAdjustAndLedger(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	id := e.firstMemberID(t)
	base := fmt.Sprintf("/api/v1/crm/members/%d", id)
	owner := token(t, 1, "owner")

	res := e.do(t, http.MethodPost, base+"/points/adjust", owner, map[string]any{"points": 150, "reason": "ชดเชยลูกค้า"})
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	var entry struct {
		Type         string `json:"type"`
		Points       int64  `json:"points"`
		BalanceAfter int64  `json:"balanceAfter"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &entry))
	require.Equal(t, "adjust", entry.Type)
	require.EqualValues(t, 150, entry.BalanceAfter)

	res = e.do(t, http.MethodPost, base+"/points/adjust", owner, map[string]any{"points": -500, "reason": "หักเกินยอด"})
	require.Equal(t, http.StatusConflict, res.Status)
	require.Equal(t, "INSUFFICIENT_POINTS", res.Body.Error.Code)

	res = e.do(t, http.MethodPost, base+"/points/adjust", owner, map[string]any{"points": 10, "reason": "ok"})
	require.Equal(t, http.StatusBadRequest, res.Status)
	require.Equal(t, "VALIDATION_ERROR", res.Body.Error.Code)

	res = e.do(t, http.MethodGet, base+"/points?type=adjust", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.EqualValues(t, 1, res.Body.Meta.Total)

	res = e.do(t, http.MethodGet, base, owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	var detail struct {
		PointBalance int64 `json:"pointBalance"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &detail))
	require.EqualValues(t, 150, detail.PointBalance)

	// The cached balance feeds the list filters.
	res = e.do(t, http.MethodGet, "/api/v1/crm/members?minPoints=100", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.EqualValues(t, 1, res.Body.Meta.Total)
	res = e.do(t, http.MethodGet, "/api/v1/crm/members?maxPoints=0", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.EqualValues(t, 2, res.Body.Meta.Total)
}

func TestMemberStatusTagsAndNotes(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	base := fmt.Sprintf("/api/v1/crm/members/%d", e.firstMemberID(t))
	owner := token(t, 1, "owner")

	res := e.do(t, http.MethodPatch, base, owner, map[string]string{"status": "suspended"})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	res = e.do(t, http.MethodPatch, base, owner, map[string]string{"status": "deleted"})
	require.Equal(t, http.StatusBadRequest, res.Status, "members are never deleted through the API")

	res = e.do(t, http.MethodPost, base+"/tags", owner, map[string]string{"tag": "  VIP "})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	res = e.do(t, http.MethodGet, base, owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	var detail struct {
		Status string `json:"status"`
		Tags   []struct {
			Tag string `json:"tag"`
		} `json:"tags"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &detail))
	require.Equal(t, "suspended", detail.Status)
	require.Len(t, detail.Tags, 1)
	require.Equal(t, "vip", detail.Tags[0].Tag)

	res = e.do(t, http.MethodDelete, base+"/tags/vip", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)

	res = e.do(t, http.MethodPost, base+"/notes", owner, map[string]string{"body": "ยืนยันตัวตนทางโทรศัพท์แล้ว"})
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	res = e.do(t, http.MethodGet, base+"/notes", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.EqualValues(t, 1, res.Body.Meta.Total)
}

func TestMembersListRejectsBadManageFilters(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")
	for _, q := range []string{"minPoints=abc", "minPoints=-1", "minPoints=10&maxPoints=5", "inactiveDays=0", "inactiveDays=x"} {
		res := e.do(t, http.MethodGet, "/api/v1/crm/members?"+q, owner, nil)
		require.Equal(t, http.StatusBadRequest, res.Status, q)
		require.Equal(t, "VALIDATION_ERROR", res.Body.Error.Code, q)
	}
	res := e.do(t, http.MethodGet, "/api/v1/crm/members?inactiveDays=30", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
}
