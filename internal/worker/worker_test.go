package worker

import (
	"bytes"
	"context"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"crm-core/timeutil"

	"github.com/stretchr/testify/require"
)

func quietLogger() *log.Logger { return log.New(&bytes.Buffer{}, "", 0) }

func TestIntervalJobRunsRepeatedly(t *testing.T) {
	var runs int64
	runner := New([]Job{{
		Name:  "tick",
		Every: 10 * time.Millisecond,
		Run: func(context.Context) error {
			atomic.AddInt64(&runs, 1)
			return nil
		},
	}}, quietLogger())

	runner.Start(context.Background())
	time.Sleep(90 * time.Millisecond)
	runner.Stop()

	require.GreaterOrEqual(t, atomic.LoadInt64(&runs), int64(2), "an interval job should tick more than once")
}

func TestSlowJobNeverOverlapsItself(t *testing.T) {
	var (
		mu       sync.Mutex
		running  int
		maxSeen  int
		finished int64
	)
	runner := New([]Job{{
		Name:  "slow",
		Every: 5 * time.Millisecond,
		Run: func(context.Context) error {
			mu.Lock()
			running++
			if running > maxSeen {
				maxSeen = running
			}
			mu.Unlock()

			time.Sleep(40 * time.Millisecond)

			mu.Lock()
			running--
			mu.Unlock()
			atomic.AddInt64(&finished, 1)
			return nil
		},
	}}, quietLogger())

	runner.Start(context.Background())
	// 160ms with a 40ms job and 5ms ticks: ticks are dropped, never queued.
	time.Sleep(160 * time.Millisecond)
	runner.Stop()

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, maxSeen, "the same job must never run twice at once")
	require.GreaterOrEqual(t, atomic.LoadInt64(&finished), int64(2))
}

func TestStopWaitsForTheRunningJob(t *testing.T) {
	var (
		started  = make(chan struct{})
		released = make(chan struct{})
		finished int64
	)
	runner := New([]Job{{
		Name:  "blocker",
		Every: 5 * time.Millisecond,
		Run: func(context.Context) error {
			select {
			case <-started:
			default:
				close(started)
			}
			<-released
			atomic.AddInt64(&finished, 1)
			return nil
		},
	}}, quietLogger())

	runner.Start(context.Background())
	<-started

	stopped := make(chan struct{})
	go func() {
		runner.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("Stop returned while a job was still running")
	case <-time.After(30 * time.Millisecond):
	}

	close(released)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after the job finished")
	}
	require.GreaterOrEqual(t, atomic.LoadInt64(&finished), int64(1), "the in-flight run must finish before Stop returns")
}

// TestNoRunsAfterStop pins the other half: once Stop returns, nothing runs.
func TestNoRunsAfterStop(t *testing.T) {
	var runs int64
	runner := New([]Job{{
		Name:  "counter",
		Every: 5 * time.Millisecond,
		Run: func(context.Context) error {
			atomic.AddInt64(&runs, 1)
			return nil
		},
	}}, quietLogger())

	runner.Start(context.Background())
	time.Sleep(40 * time.Millisecond)
	runner.Stop()

	after := atomic.LoadInt64(&runs)
	require.GreaterOrEqual(t, after, int64(1))
	time.Sleep(40 * time.Millisecond)
	require.Equal(t, after, atomic.LoadInt64(&runs), "no job may run after Stop")
}

func TestStopBeforeStartIsSafe(t *testing.T) {
	runner := New(nil, quietLogger())
	require.NotPanics(t, runner.Stop)
}

func TestJobsWithBadScheduleAreSkipped(t *testing.T) {
	var runs int64
	runner := New([]Job{
		{Name: "no schedule", Run: func(context.Context) error { atomic.AddInt64(&runs, 1); return nil }},
		{Name: "bad time", At: &DailyAt{Hour: 25}, Run: func(context.Context) error { atomic.AddInt64(&runs, 1); return nil }},
		{Every: time.Millisecond, Run: func(context.Context) error { atomic.AddInt64(&runs, 1); return nil }},
	}, quietLogger())

	runner.Start(context.Background())
	time.Sleep(30 * time.Millisecond)
	runner.Stop()
	require.Zero(t, atomic.LoadInt64(&runs))
}

func TestNextDaily(t *testing.T) {
	at := DailyAt{Hour: 0, Minute: 5}
	now := time.Date(2026, 11, 17, 10, 0, 0, 0, timeutil.Bangkok())

	require.Equal(t, time.Date(2026, 11, 18, 0, 5, 0, 0, timeutil.Bangkok()), NextDaily(now, at),
		"a time already passed today rolls to tomorrow")

	soon := time.Date(2026, 11, 17, 0, 1, 0, 0, timeutil.Bangkok())
	require.Equal(t, time.Date(2026, 11, 17, 0, 5, 0, 0, timeutil.Bangkok()), NextDaily(soon, at))

	exact := time.Date(2026, 11, 17, 0, 5, 0, 0, timeutil.Bangkok())
	require.Equal(t, time.Date(2026, 11, 18, 0, 5, 0, 0, timeutil.Bangkok()), NextDaily(exact, at),
		"an exact hit waits for the next day")

	// The same instant expressed in UTC keeps the Bangkok schedule.
	utc := time.Date(2026, 11, 17, 20, 0, 0, 0, time.UTC) // 18 Nov 03:00 Bangkok
	require.Equal(t, time.Date(2026, 11, 19, 0, 5, 0, 0, timeutil.Bangkok()), NextDaily(utc, at))
}
