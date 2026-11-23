// Package line is a small LINE Messaging API client: push messages, per-user
// rich menus and the rich menu setup used by cmd/richmenu. It implements
// crm-core/usecase/linemsg.Sender.
package line

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	domainlinemsg "crm-core/domain/linemsg"
)

const (
	defaultAPI  = "https://api.line.me"
	defaultData = "https://api-data.line.me"
)

// Client calls the Messaging API with a channel access token.
type Client struct {
	token        string
	memberMenuID string
	api          string
	data         string
	http         *http.Client
}

// New builds the client. memberMenuID is the rich menu linked to members
// (LINE_MEMBER_RICH_MENU_ID); guests see the default rich menu.
func New(token, memberMenuID string) *Client {
	return &Client{token: token, memberMenuID: memberMenuID, api: defaultAPI, data: defaultData, http: &http.Client{Timeout: 15 * time.Second}}
}

// WithBaseURLs points the client elsewhere (tests).
func (c *Client) WithBaseURLs(api, data string) *Client { c.api, c.data = api, data; return c }

// APIError is a non-2xx answer from LINE.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string { return fmt.Sprintf("LINE API HTTP %d: %s", e.Status, e.Body) }

// Unwrap marks 4xx answers (except 429 rate limit) as permanent: the request
// itself is wrong (user blocked the OA, bad message) and retrying cannot help.
func (e *APIError) Unwrap() error {
	if e.Status >= 400 && e.Status < 500 && e.Status != http.StatusTooManyRequests {
		return domainlinemsg.ErrPermanent
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, url string, body any, contentType string, out any) error {
	var reader io.Reader
	if raw, ok := body.([]byte); ok {
		reader = bytes.NewReader(raw)
	} else if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
		contentType = "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("LINE API request: %w", err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 64*1024))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return &APIError{Status: res.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// Push sends messages to one user (POST /v2/bot/message/push).
func (c *Client) Push(ctx context.Context, to string, messages json.RawMessage) error {
	return c.do(ctx, http.MethodPost, c.api+"/v2/bot/message/push", map[string]any{"to": to, "messages": messages}, "", nil)
}

// LinkMemberMenu links the member rich menu to the user.
func (c *Client) LinkMemberMenu(ctx context.Context, userID string) error {
	if c.memberMenuID == "" {
		return fmt.Errorf("%w: LINE_MEMBER_RICH_MENU_ID is not set", domainlinemsg.ErrPermanent)
	}
	return c.do(ctx, http.MethodPost, c.api+"/v2/bot/user/"+userID+"/richmenu/"+c.memberMenuID, nil, "", nil)
}

// UnlinkMenu removes the user's rich menu, so the default (guest) one shows.
func (c *Client) UnlinkMenu(ctx context.Context, userID string) error {
	err := c.do(ctx, http.MethodDelete, c.api+"/v2/bot/user/"+userID+"/richmenu", nil, "", nil)
	var apiErr *APIError
	if ok := asAPIError(err, &apiErr); ok && apiErr.Status == http.StatusNotFound {
		return nil // no per-user menu: already on the default
	}
	return err
}

func asAPIError(err error, target **APIError) bool {
	e, ok := err.(*APIError)
	if ok {
		*target = e
	}
	return ok
}

// CreateRichMenu creates a rich menu and returns its id.
func (c *Client) CreateRichMenu(ctx context.Context, menu any) (string, error) {
	var out struct {
		RichMenuID string `json:"richMenuId"`
	}
	if err := c.do(ctx, http.MethodPost, c.api+"/v2/bot/richmenu", menu, "", &out); err != nil {
		return "", err
	}
	return out.RichMenuID, nil
}

// UploadRichMenuImage uploads the menu image (PNG or JPEG).
func (c *Client) UploadRichMenuImage(ctx context.Context, menuID string, image []byte, contentType string) error {
	return c.do(ctx, http.MethodPost, c.data+"/v2/bot/richmenu/"+menuID+"/content", image, contentType, nil)
}

// SetDefaultRichMenu makes the menu the default for every user.
func (c *Client) SetDefaultRichMenu(ctx context.Context, menuID string) error {
	return c.do(ctx, http.MethodPost, c.api+"/v2/bot/user/all/richmenu/"+menuID, nil, "", nil)
}
