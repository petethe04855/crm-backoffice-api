package route

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	domainconsent "crm-core/domain/consent"

	"github.com/stretchr/testify/require"
)

func TestConsentDocumentsLifecycle(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")

	res := e.do(t, http.MethodGet, "/api/v1/crm/consent-documents", token(t, 2, "sales"), nil)
	require.Equal(t, http.StatusForbidden, res.Status, "sales cannot manage consent documents")

	body := map[string]any{"type": "terms", "title": "ข้อกำหนดการใช้งาน", "content": "ข้อ 1", "isRequired": true, "isActive": true}
	res = e.do(t, http.MethodPost, "/api/v1/crm/consent-documents", owner, body)
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	var doc struct {
		ID      uint `json:"id"`
		Version int  `json:"version"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &doc))
	require.Equal(t, 1, doc.Version)

	res = e.do(t, http.MethodPost, "/api/v1/crm/consent-documents", owner, map[string]any{"type": "marketing_line", "title": "x", "content": "y", "isRequired": true})
	require.Equal(t, http.StatusBadRequest, res.Status, "marketing consent cannot be required")

	res = e.do(t, http.MethodPut, fmt.Sprintf("/api/v1/crm/consent-documents/%d", doc.ID), owner, map[string]any{"title": "ข้อกำหนด (แก้)", "content": "ข้อ 1-2", "isRequired": true, "isActive": true})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	// Once someone accepted, the version is locked.
	require.NoError(t, e.db.Create(&domainconsent.Record{LineUserID: "U1", DocumentID: doc.ID, DocumentType: "terms", DocumentVersion: 1, Action: "accept"}).Error)
	res = e.do(t, http.MethodPut, fmt.Sprintf("/api/v1/crm/consent-documents/%d", doc.ID), owner, map[string]any{"title": "again", "content": "z", "isRequired": true})
	require.Equal(t, http.StatusConflict, res.Status)
	require.Equal(t, "CONSENT_DOCUMENT_IN_USE", res.Body.Error.Code)

	res = e.do(t, http.MethodPatch, fmt.Sprintf("/api/v1/crm/consent-documents/%d/active", doc.ID), owner, map[string]any{"isActive": false})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	res = e.do(t, http.MethodPost, "/api/v1/crm/consent-documents", owner, body)
	require.Equal(t, http.StatusCreated, res.Status)
	require.NoError(t, json.Unmarshal(res.Body.Data, &doc))
	require.Equal(t, 2, doc.Version, "a new version of the same type")

	res = e.do(t, http.MethodGet, "/api/v1/crm/consent-documents", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(res.Body.Data, &list))
	require.Len(t, list, 2)
}
