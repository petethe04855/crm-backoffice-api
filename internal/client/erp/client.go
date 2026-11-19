// Package erp is the HTTP client backoffice-api uses to talk to erp-api-v2.
// It is the only place CRM code reaches the ERP (plan: CRM_LINE_OA_MEMBERSHIP.md §6).
// It covers staff session checks (Me), the sales dashboard facts and the
// service-token calls to /integrations/crm (crm_integration.go).
package erp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	// ErrUnauthorized means the ERP rejected the credentials (401/403).
	ErrUnauthorized = errors.New("erp: unauthorized")
	// ErrUnavailable means the ERP could not be reached or failed (network, 5xx).
	ErrUnavailable = errors.New("erp: unavailable")
)

// Client calls erp-api-v2.
type Client struct {
	baseURL      string
	serviceToken string
	http         *http.Client
}

// New builds a client. baseURL is the ERP API root ending in /api/v1.
func New(baseURL, serviceToken string, timeout time.Duration) *Client {
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		serviceToken: serviceToken,
		http:         &http.Client{Timeout: timeout},
	}
}

// StaffUser is the subset of the ERP /auth/me payload the CRM needs.
type StaffUser struct {
	ID                uint   `json:"id"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	Role              string `json:"role"`
	IsActive          bool   `json:"isActive"`
	MustResetPassword bool   `json:"mustResetPassword"`
}

type envelope[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
}

// Me returns the current state of the staff user owning bearerToken. The ERP// re-checks the account in its database, so disabled or demoted users are
// caught here even while their JWT is still unexpired.
func (c *Client) Me(ctx context.Context, bearerToken string) (*StaffUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/auth/me", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrUnauthorized
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}

	var body envelope[StaffUser]
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrUnavailable, err)
	}
	if !body.Success || body.Data.ID == 0 {
		return nil, fmt.Errorf("%w: unexpected payload", ErrUnavailable)
	}
	return &body.Data, nil
}
