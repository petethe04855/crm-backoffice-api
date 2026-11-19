// Package session checks staff sessions against the ERP with a short cache,
// so each request does not hit erp-api-v2 /auth/me.
package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"backoffice-api/internal/client/erp"
)

// StaffLookup fetches the live staff record for a token.
type StaffLookup interface {
	Me(ctx context.Context, bearerToken string) (*erp.StaffUser, error)
}

// Checker caches ERP session lookups per token for ttl. Only successful
// lookups are cached; failures are retried on the next request.
type Checker struct {
	lookup StaffLookup
	ttl    time.Duration
	now    func() time.Time

	mu      sync.Mutex
	entries map[string]entry
}

type entry struct {
	user    erp.StaffUser
	expires time.Time
}

// NewChecker builds a cached checker.
func NewChecker(lookup StaffLookup, ttl time.Duration) *Checker {
	return &Checker{lookup: lookup, ttl: ttl, now: time.Now, entries: map[string]entry{}}
}

func tokenKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Check returns the live staff record for token.
func (c *Checker) Check(ctx context.Context, token string) (*erp.StaffUser, error) {
	key := tokenKey(token)
	now := c.now()

	c.mu.Lock()
	if e, ok := c.entries[key]; ok && now.Before(e.expires) {
		c.mu.Unlock()
		u := e.user
		return &u, nil
	}
	c.mu.Unlock()

	user, err := c.lookup.Me(ctx, token)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry{user: *user, expires: now.Add(c.ttl)}
	if len(c.entries) > 1000 { // drop expired entries occasionally
		for k, e := range c.entries {
			if !now.Before(e.expires) {
				delete(c.entries, k)
			}
		}
	}
	return user, nil
}
