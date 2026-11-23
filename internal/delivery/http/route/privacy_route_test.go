package route

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	domainmember "crm-core/domain/member"
	domainprivacy "crm-core/domain/privacy"

	"github.com/stretchr/testify/require"
)

func TestDeletionQueue(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")
	e.seedMembers(t)
	ids := e.memberIDs(t)
	for i, id := range ids[:2] {
		require.NoError(t, e.db.Create(&domainprivacy.DataRequest{MemberID: id, Type: domainprivacy.RequestDelete,
			Status: domainprivacy.RequestPending, MemberCode: fmt.Sprintf("M%d", i), CreatedAt: time.Now()}).Error)
	}

	res := e.do(t, http.MethodGet, "/api/v1/crm/data-requests?status=pending", token(t, 2, "sales"), nil)
	require.Equal(t, http.StatusForbidden, res.Status)

	res = e.do(t, http.MethodGet, "/api/v1/crm/data-requests?status=pending", owner, nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	require.EqualValues(t, 2, res.Body.Meta.Total)
	var rows []struct {
		ID       uint `json:"id"`
		MemberID uint `json:"memberId"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &rows))

	res = e.do(t, http.MethodPost, fmt.Sprintf("/api/v1/crm/data-requests/%d/complete", rows[0].ID), owner, map[string]string{"note": "ok"})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var m domainmember.Member
	require.NoError(t, e.db.First(&m, rows[0].MemberID).Error)
	require.Equal(t, domainmember.StatusDeleted, m.Status)

	res = e.do(t, http.MethodPost, fmt.Sprintf("/api/v1/crm/data-requests/%d/complete", rows[0].ID), owner, nil)
	require.Equal(t, http.StatusConflict, res.Status)
	require.Equal(t, "REQUEST_CLOSED", res.Body.Error.Code)

	res = e.do(t, http.MethodPost, fmt.Sprintf("/api/v1/crm/data-requests/%d/reject", rows[1].ID), owner, map[string]string{"note": ""})
	require.Equal(t, http.StatusBadRequest, res.Status, "a reason is required")
	res = e.do(t, http.MethodPost, fmt.Sprintf("/api/v1/crm/data-requests/%d/reject", rows[1].ID), owner, map[string]string{"note": "มียอดค้าง"})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	res = e.do(t, http.MethodGet, "/api/v1/crm/data-requests?status=pending", owner, nil)
	require.EqualValues(t, 0, res.Body.Meta.Total)
}
