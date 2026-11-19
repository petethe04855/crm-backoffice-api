package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"backoffice-api/internal/client/erp"

	"github.com/stretchr/testify/require"
)

type countingLookup struct {
	calls int
	err   error
}

func (l *countingLookup) Me(context.Context, string) (*erp.StaffUser, error) {
	l.calls++
	if l.err != nil {
		return nil, l.err
	}
	return &erp.StaffUser{ID: 7, Role: "sales", IsActive: true}, nil
}

func TestCheckerCachesWithinTTL(t *testing.T) {
	lookup := &countingLookup{}
	c := NewChecker(lookup, time.Minute)
	now := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		u, err := c.Check(context.Background(), "tok")
		require.NoError(t, err)
		require.EqualValues(t, 7, u.ID)
	}
	require.Equal(t, 1, lookup.calls)

	_, _ = c.Check(context.Background(), "other-token")
	require.Equal(t, 2, lookup.calls, "cache is per token")

	now = now.Add(61 * time.Second)
	_, _ = c.Check(context.Background(), "tok")
	require.Equal(t, 3, lookup.calls, "expired entries are refreshed")
}

func TestCheckerDoesNotCacheFailures(t *testing.T) {
	lookup := &countingLookup{err: errors.New("down")}
	c := NewChecker(lookup, time.Minute)
	_, err := c.Check(context.Background(), "tok")
	require.Error(t, err)
	_, err = c.Check(context.Background(), "tok")
	require.Error(t, err)
	require.Equal(t, 2, lookup.calls)
}
