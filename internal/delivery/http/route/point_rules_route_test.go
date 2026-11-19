package route

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPointRulesRBAC(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")
	sales := token(t, 2, "sales")
	warehouse := token(t, 3, "warehouse")
	accountant := token(t, 6, "accountant")

	rule := map[string]any{"type": "campaign_multiplier", "multiplier": 2}
	cases := []struct {
		name   string
		method string
		path   string
		bearer string
		body   any
		status int
	}{
		{"accountant lists rules", http.MethodGet, "/api/v1/crm/earn-rules", accountant, nil, http.StatusOK},
		{"accountant reads redeem policy", http.MethodGet, "/api/v1/crm/settings/redeem-policy", accountant, nil, http.StatusOK},
		{"accountant reads expiry", http.MethodGet, "/api/v1/crm/settings/point-expiry", accountant, nil, http.StatusOK},
		{"accountant simulates", http.MethodPost, "/api/v1/crm/point-rules/simulate", accountant, map[string]any{"tierId": e.tierID, "basePoints": 10}, http.StatusOK},
		{"accountant cannot create rules", http.MethodPost, "/api/v1/crm/earn-rules", accountant, rule, http.StatusForbidden},
		{"accountant cannot change policy", http.MethodPut, "/api/v1/crm/settings/redeem-policy", accountant, map[string]any{"allowedStatuses": []string{"active"}}, http.StatusForbidden},
		{"sales has no settings", http.MethodGet, "/api/v1/crm/earn-rules", sales, nil, http.StatusForbidden},
		{"warehouse has no settings", http.MethodGet, "/api/v1/crm/settings/point-adjust-policy", warehouse, nil, http.StatusForbidden},
		{"owner creates a rule", http.MethodPost, "/api/v1/crm/earn-rules", owner, rule, http.StatusCreated},
	}
	for _, c := range cases {
		res := e.do(t, c.method, c.path, c.bearer, c.body)
		require.Equal(t, c.status, res.Status, "%s: %s", c.name, res.Raw)
	}
}

func TestEarnRulesAndSimulate(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")

	// A disabled rule must stay disabled (regression: a DB default turned it on).
	res := e.do(t, http.MethodPost, "/api/v1/crm/earn-rules", owner,
		map[string]any{"type": "campaign_multiplier", "multiplier": 5, "isActive": false})
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	var disabled struct {
		ID       uint `json:"id"`
		IsActive bool `json:"isActive"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &disabled))
	require.False(t, disabled.IsActive)

	res = e.do(t, http.MethodPost, "/api/v1/crm/earn-rules", owner,
		map[string]any{"type": "campaign_multiplier", "multiplier": 2})
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))

	res = e.do(t, http.MethodPost, "/api/v1/crm/point-rules/simulate", owner,
		map[string]any{"tierId": e.tierID, "basePoints": 15})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var sim struct {
		Points int64             `json:"points"`
		Rules  []json.RawMessage `json:"rules"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &sim))
	require.EqualValues(t, 30, sim.Points, "only the active ×2 campaign applies")
	require.Len(t, sim.Rules, 1)

	// Re-enabling the ×5 rule through PATCH changes the result.
	res = e.do(t, http.MethodPatch, fmt.Sprintf("/api/v1/crm/earn-rules/%d", disabled.ID), owner,
		map[string]any{"type": "campaign_multiplier", "multiplier": 5, "priority": 9, "isActive": true})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	res = e.do(t, http.MethodPost, "/api/v1/crm/point-rules/simulate", owner,
		map[string]any{"tierId": e.tierID, "basePoints": 15})
	require.NoError(t, json.Unmarshal(res.Body.Data, &sim))
	require.EqualValues(t, 75, sim.Points, "the higher-priority non-stackable campaign wins")

	res = e.do(t, http.MethodPost, "/api/v1/crm/earn-rules", owner, map[string]any{"type": "nope"})
	require.Equal(t, http.StatusBadRequest, res.Status)
	res = e.do(t, http.MethodPatch, "/api/v1/crm/earn-rules/9999", owner, map[string]any{"type": "campaign_multiplier", "multiplier": 2})
	require.Equal(t, http.StatusNotFound, res.Status)
}

func TestPointPoliciesRoundTrip(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")

	res := e.do(t, http.MethodPut, "/api/v1/crm/settings/redeem-policy", owner, map[string]any{
		"minBalanceToRedeem": 100, "minMemberAgeDays": 7, "maxRedemptionsPerDay": 3,
		"requireEmailVerified": true, "allowedStatuses": []string{"Active"},
	})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	res = e.do(t, http.MethodGet, "/api/v1/crm/settings/redeem-policy", owner, nil)
	require.JSONEq(t, `{"minBalanceToRedeem":100,"minMemberAgeDays":7,"maxRedemptionsPerDay":3,"requireEmailVerified":true,"allowedStatuses":["active"]}`, string(res.Body.Data))

	res = e.do(t, http.MethodPut, "/api/v1/crm/settings/redeem-policy", owner, map[string]any{"allowedStatuses": []string{"banned"}})
	require.Equal(t, http.StatusBadRequest, res.Status)

	res = e.do(t, http.MethodGet, "/api/v1/crm/settings/point-expiry", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.JSONEq(t, `{"mode":"rolling_months","months":12}`, string(res.Body.Data), "default from the work order")

	res = e.do(t, http.MethodPut, "/api/v1/crm/settings/point-expiry", owner, map[string]any{"mode": "rolling_months", "months": 6})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	res = e.do(t, http.MethodPut, "/api/v1/crm/settings/point-adjust-policy", owner, map[string]any{"maxPerAdjust": 500, "maxPerDay": 2000})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	res = e.do(t, http.MethodGet, "/api/v1/crm/settings/point-adjust-policy", owner, nil)
	require.JSONEq(t, `{"maxPerAdjust":500,"maxPerDay":2000}`, string(res.Body.Data))
}
