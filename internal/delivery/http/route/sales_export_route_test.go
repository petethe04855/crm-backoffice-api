package route

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestSalesDashboardExportsEveryTab(t *testing.T) {
	e := newEnv(t)
	e.seedSales()
	owner := token(t, 1, "owner")

	sheets := map[string]string{
		"summary": "Summary", "products": "Flavors", "provinces": "Provinces",
		"customers": "Customers", "timing": "Hours", "repurchase": "Repurchase",
	}
	for section, first := range sheets {
		res := e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard/"+section+dashboardQuery+"&format=xlsx", owner, nil)
		require.Equal(t, http.StatusOK, res.Status, "%s: %s", section, res.Raw)
		require.Contains(t, res.Header.Get("Content-Disposition"), "sales-"+section+"-", section)
		f, err := excelize.OpenReader(bytes.NewReader(res.Raw))
		require.NoError(t, err, section)
		require.Equal(t, first, f.GetSheetName(0), section)
		_ = f.Close()
	}
}

func TestSalesDashboardExportMasksBuyersUnlessOwner(t *testing.T) {
	e := newEnv(t)
	e.seedSales()

	buyers := func(bearer string) []string {
		res := e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard/customers"+dashboardQuery+"&format=xlsx", bearer, nil)
		require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
		f, err := excelize.OpenReader(bytes.NewReader(res.Raw))
		require.NoError(t, err)
		defer f.Close()
		rows, err := f.GetRows("Customers")
		require.NoError(t, err)
		var out []string
		for _, r := range rows[1:] {
			if len(r) > 1 && r[1] != "" {
				out = append(out, r[1])
			}
		}
		return out
	}
	owner := strings.Join(buyers(token(t, 1, "owner")), ",")
	require.Contains(t, owner, "alice1234")
	sales := strings.Join(buyers(token(t, 2, "sales")), ",")
	require.NotContains(t, sales, "alice1234")
	require.Contains(t, sales, "***")

	res := e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard/customers"+dashboardQuery+"&format=xlsx", token(t, 6, "accountant"), nil)
	require.Equal(t, http.StatusForbidden, res.Status, "accountant never gets buyer rows, not even as a file")
	res = e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard/provinces"+dashboardQuery+"&format=xlsx", token(t, 6, "accountant"), nil)
	require.Equal(t, http.StatusOK, res.Status, "aggregates stay exportable")
}
