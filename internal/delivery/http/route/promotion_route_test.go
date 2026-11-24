package route

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// promotionPayload is the subset of PromotionResponse the route tests assert.
type promotionPayload struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Placement      string   `json:"placement"`
	ImageURL       string   `json:"imageUrl"`
	DisplayStartAt *string  `json:"displayStartAt"`
	DisplayEndAt   *string  `json:"displayEndAt"`
	Status         string   `json:"status"`
	Priority       int      `json:"priority"`
	DisplayStatus  string   `json:"displayStatus"`
	Warnings       []string `json:"warnings"`
	Audience       struct {
		Tiers         []string `json:"tiers"`
		NewMemberDays *int     `json:"newMemberDays"`
	} `json:"audience"`
}

func decodePromotion(t *testing.T, res apiResult) promotionPayload {
	t.Helper()
	var p promotionPayload
	require.NoError(t, json.Unmarshal(res.Body.Data, &p), string(res.Raw))
	return p
}

// windowAhead returns an RFC3339 window starting two hours from now, so the
// published promotion is "scheduled" rather than live or expired.
func windowAhead() (string, string) {
	start := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	return start.Format(time.RFC3339), start.Add(24 * time.Hour).Format(time.RFC3339)
}

func TestPromotionRBAC(t *testing.T) {
	e := newEnv(t)
	sales := token(t, 2, "sales")
	warehouse := token(t, 3, "warehouse")
	accountant := token(t, 6, "accountant")

	res := e.do(t, http.MethodGet, "/api/v1/crm/promotions", sales, nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	res = e.do(t, http.MethodGet, "/api/v1/crm/promotions?status=draft&placement=popup&search=x", sales, nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	res = e.do(t, http.MethodGet, "/api/v1/crm/promotions", warehouse, nil)
	require.Equal(t, http.StatusForbidden, res.Status, string(res.Raw))

	// The accountant is numbers-only and does not see marketing screens.
	res = e.do(t, http.MethodGet, "/api/v1/crm/promotions", accountant, nil)
	require.Equal(t, http.StatusForbidden, res.Status, string(res.Raw))

	// Sales may look but not change anything.
	body := map[string]any{"title": "โปรใหม่", "placement": "popup"}
	res = e.do(t, http.MethodPost, "/api/v1/crm/promotions", sales, body)
	require.Equal(t, http.StatusForbidden, res.Status, string(res.Raw))

	res = e.do(t, http.MethodPost, "/api/v1/crm/promotions", warehouse, body)
	require.Equal(t, http.StatusForbidden, res.Status, string(res.Raw))

	res = e.do(t, http.MethodPost, "/api/v1/crm/promotions", sales, body)
	require.Equal(t, http.StatusForbidden, res.Status)
}

func TestPromotionListRejectsBadMonth(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")

	for _, month := range []string{"2026-13", "11-2026", "2026-1"} {
		res := e.do(t, http.MethodGet, "/api/v1/crm/promotions?month="+month, owner, nil)
		require.Equal(t, http.StatusBadRequest, res.Status, month)
		require.Equal(t, "VALIDATION_ERROR", res.Body.Error.Code, month)
	}
}

func TestPromotionLifecycle(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")

	// A draft may be saved with only a title; the response always carries a
	// displayStatus, a warnings array and empty (not null) audience tiers.
	res := e.do(t, http.MethodPost, "/api/v1/crm/promotions", owner, map[string]any{
		"title":     "โปรตุลาคม",
		"placement": "home_banner",
	})
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	draft := decodePromotion(t, res)
	require.Equal(t, "draft", draft.Status)
	require.Equal(t, "draft", draft.DisplayStatus)
	require.NotNil(t, draft.Warnings)
	require.Empty(t, draft.Warnings)
	require.NotNil(t, draft.Audience.Tiers)
	require.Empty(t, draft.Audience.Tiers)
	require.Nil(t, draft.DisplayStartAt, "a draft without a window has null dates")

	var list struct {
		Data []promotionPayload `json:"data"`
	}
	// The search filter is URL-escaped like the browser sends it.
	res = e.do(t, http.MethodGet, "/api/v1/crm/promotions?search="+url.QueryEscape("ตุลาคม"), owner, nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	require.NoError(t, json.Unmarshal(res.Raw, &list))
	require.Len(t, list.Data, 1)
	require.Equal(t, draft.ID, list.Data[0].ID)

	// Publishing without an image or window is rejected by the usecase.
	res = e.do(t, http.MethodPost, "/api/v1/crm/promotions/"+draft.ID+"/publish", owner, nil)
	require.Equal(t, http.StatusBadRequest, res.Status, string(res.Raw))

	start, end := windowAhead()
	res = e.do(t, http.MethodPatch, "/api/v1/crm/promotions/"+draft.ID, owner, map[string]any{
		"title":          "โปรตุลาคม",
		"imageUrl":       "/api/v1/files/banner",
		"placement":      "home_banner",
		"displayStartAt": start,
		"displayEndAt":   end,
		"priority":       3,
		"audience":       map[string]any{"tiers": []string{"gold"}},
	})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	edited := decodePromotion(t, res)
	require.Equal(t, []string{"gold"}, edited.Audience.Tiers)
	require.NotNil(t, edited.DisplayStartAt)

	res = e.do(t, http.MethodPost, "/api/v1/crm/promotions/"+draft.ID+"/publish", owner, nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	published := decodePromotion(t, res)
	require.Equal(t, "published", published.Status)
	require.Equal(t, "scheduled", published.DisplayStatus, "the window starts in two hours")
	require.NotNil(t, published.Warnings)
	require.Empty(t, published.Warnings)

	// The list screen reorders a live promotion without resending the form.
	res = e.do(t, http.MethodPatch, "/api/v1/crm/promotions/"+draft.ID+"/priority", owner, map[string]any{"priority": 7})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	require.Equal(t, 7, decodePromotion(t, res).Priority)
	require.Equal(t, "published", decodePromotion(t, res).Status)
	res = e.do(t, http.MethodPatch, "/api/v1/crm/promotions/"+draft.ID+"/priority", owner, map[string]any{"priority": 10000})
	require.Equal(t, http.StatusBadRequest, res.Status, string(res.Raw))
	res = e.do(t, http.MethodPatch, "/api/v1/crm/promotions/"+draft.ID+"/priority", owner, map[string]any{})
	require.Equal(t, http.StatusBadRequest, res.Status, string(res.Raw))

	// Publishing twice and deleting a published promotion are conflicts.
	res = e.do(t, http.MethodPost, "/api/v1/crm/promotions/"+draft.ID+"/publish", owner, nil)
	require.Equal(t, http.StatusConflict, res.Status, string(res.Raw))

	res = e.do(t, http.MethodDelete, "/api/v1/crm/promotions/"+draft.ID, owner, nil)
	require.Equal(t, http.StatusConflict, res.Status, string(res.Raw))

	// Duplicate produces a deletable draft; the original stays published.
	res = e.do(t, http.MethodPost, "/api/v1/crm/promotions/"+draft.ID+"/duplicate", owner, nil)
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	copied := decodePromotion(t, res)
	require.NotEqual(t, draft.ID, copied.ID)
	require.Equal(t, "draft", copied.Status)
	require.Equal(t, "draft", copied.DisplayStatus)

	res = e.do(t, http.MethodDelete, "/api/v1/crm/promotions/"+copied.ID, owner, nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	res = e.do(t, http.MethodPost, "/api/v1/crm/promotions/"+draft.ID+"/archive", owner, nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	archived := decodePromotion(t, res)
	require.Equal(t, "archived", archived.Status)
	require.Equal(t, "archived", archived.DisplayStatus)

	// Archived promotions can no longer be edited or published.
	res = e.do(t, http.MethodPatch, "/api/v1/crm/promotions/"+draft.ID, owner, map[string]any{"title": "แก้ไม่ได้"})
	require.Equal(t, http.StatusConflict, res.Status, string(res.Raw))
	res = e.do(t, http.MethodPatch, "/api/v1/crm/promotions/"+draft.ID+"/priority", owner, map[string]any{"priority": 1})
	require.Equal(t, http.StatusConflict, res.Status, string(res.Raw))
	res = e.do(t, http.MethodPost, "/api/v1/crm/promotions/"+draft.ID+"/publish", owner, nil)
	require.Equal(t, http.StatusConflict, res.Status, string(res.Raw))
}

func TestPromotionGetAndNotFound(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")

	res := e.do(t, http.MethodGet, "/api/v1/crm/promotions/does-not-exist", owner, nil)
	require.Equal(t, http.StatusNotFound, res.Status, string(res.Raw))
	require.Equal(t, "NOT_FOUND", res.Body.Error.Code)

	res = e.do(t, http.MethodPatch, "/api/v1/crm/promotions/does-not-exist", owner, map[string]any{"title": "x"})
	require.Equal(t, http.StatusNotFound, res.Status, string(res.Raw))
}

func TestPromotionValidationAndWarningsShape(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")

	// An http:// CTA target is rejected.
	res := e.do(t, http.MethodPost, "/api/v1/crm/promotions", owner, map[string]any{
		"title": "CTA", "placement": "popup", "ctaType": "url", "ctaTarget": "http://example.com",
	})
	require.Equal(t, http.StatusBadRequest, res.Status, string(res.Raw))
	require.Contains(t, string(res.Raw), "https://")

	// An empty body is a malformed request (400), not a 500.
	req := e.do(t, http.MethodPost, "/api/v1/crm/promotions", owner, nil)
	require.Equal(t, http.StatusBadRequest, req.Status, string(req.Raw))

	// warnings is always serialized as an array.
	res = e.do(t, http.MethodPost, "/api/v1/crm/promotions", owner, map[string]any{
		"title": "ปกติ", "placement": "promotion_list",
	})
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	require.Contains(t, string(res.Raw), `"warnings":[]`)
	require.Contains(t, string(res.Raw), `"displayStatus":"draft"`)
}
