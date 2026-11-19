package route

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemberReportAPI(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t) // 3 members registered 10–14 Nov 2026, one suspended

	res := e.do(t, http.MethodGet, "/api/v1/crm/reports/members?dateFrom=2026-11-01&dateTo=2026-11-30", token(t, 6, "accountant"), nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var r struct {
		DateFrom     string `json:"dateFrom"`
		DateTo       string `json:"dateTo"`
		TotalMembers int64  `json:"totalMembers"`
		NewMembers   int64  `json:"newMembers"`
		Months       []struct {
			Month      string `json:"month"`
			NewMembers int64  `json:"newMembers"`
		} `json:"months"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &r))
	require.Equal(t, "2026-11-01", r.DateFrom)
	require.Equal(t, "2026-11-30", r.DateTo)
	require.EqualValues(t, 3, r.NewMembers)
	require.Len(t, r.Months, 1)
	require.Equal(t, "2026-11", r.Months[0].Month)

	res = e.do(t, http.MethodGet, "/api/v1/crm/reports/members", token(t, 3, "warehouse"), nil)
	require.Equal(t, http.StatusForbidden, res.Status)
	res = e.do(t, http.MethodGet, "/api/v1/crm/reports/members?dateFrom=2026-12-01&dateTo=2026-11-01", token(t, 1, "owner"), nil)
	require.Equal(t, http.StatusBadRequest, res.Status)
	res = e.do(t, http.MethodGet, "/api/v1/crm/reports/members?dateFrom=2024-01-01&dateTo=2026-11-01", token(t, 1, "owner"), nil)
	require.Equal(t, http.StatusBadRequest, res.Status, "over 366 days")
	res = e.do(t, http.MethodGet, "/api/v1/crm/reports/members", token(t, 2, "sales"), nil)
	require.Equal(t, http.StatusOK, res.Status, "defaults to the last 90 days")
}
