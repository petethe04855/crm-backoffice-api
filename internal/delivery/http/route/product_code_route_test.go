package route

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func batchBody() map[string]any {
	return map[string]any{
		"erpSkuId": 101, "skuCode": "CHK-01", "skuName": "ขนมไก่", "skuPrice": 59,
		"quantity": 25, "pointsPerCode": 10, "lotNumber": "LOT-A", "codeExpiresAt": "2099-12-31",
	}
}

func TestProductCodeBatchLifecycle(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")

	res := e.do(t, http.MethodGet, "/api/v1/crm/product-code-batches", token(t, 2, "sales"), nil)
	require.Equal(t, http.StatusForbidden, res.Status, "sales cannot manage product codes")

	bad := batchBody()
	bad["quantity"] = 0
	res = e.do(t, http.MethodPost, "/api/v1/crm/product-code-batches", owner, bad)
	require.Equal(t, http.StatusBadRequest, res.Status)

	res = e.do(t, http.MethodPost, "/api/v1/crm/product-code-batches", owner, batchBody())
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	var b struct {
		ID      uint   `json:"id"`
		BatchNo string `json:"batchNo"`
		Status  string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &b))
	require.Equal(t, "draft", b.Status)
	path := func(action string) string { return fmt.Sprintf("/api/v1/crm/product-code-batches/%d/%s", b.ID, action) }

	res = e.do(t, http.MethodPost, path("activate"), owner, nil)
	require.Equal(t, http.StatusConflict, res.Status)
	require.Equal(t, "BATCH_NOT_EXPORTED", res.Body.Error.Code)

	res = e.do(t, http.MethodPost, path("export")+"?format=csv", owner, nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	require.Contains(t, res.Header.Get("Content-Disposition"), b.BatchNo+"-codes.csv")
	lines := strings.Split(strings.TrimSpace(string(res.Raw)), "\n")
	require.Len(t, lines, 26, "header + 25 codes")
	require.Regexp(t, `^1,[0-9A-Z]{4}-[0-9A-Z]{4}-[0-9A-Z]{4},`+b.BatchNo+`,CHK-01,`, strings.TrimSpace(lines[1]))

	res = e.do(t, http.MethodPost, path("export"), owner, nil)
	require.Equal(t, http.StatusConflict, res.Status, "second export")
	require.Equal(t, "BATCH_ALREADY_EXPORTED", res.Body.Error.Code)

	res = e.do(t, http.MethodPost, path("activate"), owner, nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var active struct {
		Status string `json:"status"`
		Stats  struct {
			Total  int `json:"total"`
			Active int `json:"active"`
		} `json:"stats"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &active))
	require.Equal(t, "active", active.Status)
	require.Equal(t, 25, active.Stats.Total)

	res = e.do(t, http.MethodPost, path("void"), owner, map[string]string{"reason": "ฉลากพิมพ์ผิด"})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	res = e.do(t, http.MethodGet, "/api/v1/crm/product-code-batches?status=void", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.EqualValues(t, 1, res.Body.Meta.Total)
}

func TestProductCodeExportIsOwnerOnly(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")
	res := e.do(t, http.MethodPost, "/api/v1/crm/product-code-batches", owner, batchBody())
	require.Equal(t, http.StatusCreated, res.Status)
	var b struct {
		ID uint `json:"id"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &b))

	// A CRM admin manages batches but the export needs an owner (authz matrix).
	res = e.do(t, http.MethodPut, "/api/v1/crm/staff-roles/2", owner, map[string]string{"email": "sales@example.com", "role": "admin"})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	admin := token(t, 2, "sales")
	res = e.do(t, http.MethodGet, "/api/v1/crm/product-code-batches", admin, nil)
	require.Equal(t, http.StatusOK, res.Status, "admin can list")
	res = e.do(t, http.MethodPost, fmt.Sprintf("/api/v1/crm/product-code-batches/%d/export", b.ID), admin, nil)
	require.Equal(t, http.StatusForbidden, res.Status, "admin cannot export codes")
}
