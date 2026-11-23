package route

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTestMembersAndCodePolicy(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")
	e.seedMembers(t)
	memberID := e.memberIDs(t)[0]

	res := e.do(t, http.MethodPut, fmt.Sprintf("/api/v1/crm/test-members/%d", memberID), owner, nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	res = e.do(t, http.MethodPut, "/api/v1/crm/test-members/99999", owner, nil)
	require.Equal(t, http.StatusNotFound, res.Status)

	res = e.do(t, http.MethodGet, "/api/v1/crm/test-members", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	var testers []struct {
		MemberID uint `json:"memberId"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &testers))
	require.Len(t, testers, 1)
	require.Equal(t, memberID, testers[0].MemberID)

	res = e.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/crm/test-members/%d", memberID), owner, nil)
	require.Equal(t, http.StatusOK, res.Status)

	res = e.do(t, http.MethodGet, "/api/v1/crm/product-code-policy", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.JSONEq(t, `{"dailyQuota":20,"wrongPerHour":5,"wrongPerDay":20}`, string(res.Body.Data))

	res = e.do(t, http.MethodPut, "/api/v1/crm/product-code-policy", owner, map[string]int{"dailyQuota": 30, "wrongPerHour": 5, "wrongPerDay": 20})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	res = e.do(t, http.MethodPut, "/api/v1/crm/product-code-policy", owner, map[string]int{"dailyQuota": 30, "wrongPerHour": 10, "wrongPerDay": 5})
	require.Equal(t, http.StatusBadRequest, res.Status, "a day limit below the hour limit")

	res = e.do(t, http.MethodGet, "/api/v1/crm/product-code-flags", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.JSONEq(t, `[]`, string(res.Body.Data))

	res = e.do(t, http.MethodGet, "/api/v1/crm/test-members", token(t, 2, "sales"), nil)
	require.Equal(t, http.StatusForbidden, res.Status)
}
