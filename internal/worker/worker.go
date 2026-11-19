// Package worker runs the CRM's recurring background jobs (coupon expiry,
// point expiry, promotion state changes). A job is either an interval job or a
// daily job at a Thai wall-clock time.
package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"crm-core/timeutil"
)

// DailyAt is a time of day in Asia/Bangkok.
type DailyAt struct {
	Hour   int
	Minute int
}

// Valid reports whether the time is a real clock time.
func (d DailyAt) Valid() bool {
	return d.Hour >= 0 && d.Hour < 24 && d.Minute >= 0 && d.Minute < 60
}

// Job is one recurring task. Set exactly one of Every or At.
type Job struct {
	Name  string
	Every time.Duration
	At    *DailyAt
	Run   func(ctx context.Context) error
}

// Runner runs jobs until Stop.
//
// Each job owns one goroutine, so two runs of the same job can never overlap: a
// slow run simply skips the ticks that pass while it works. That is deliberate -
// a queued backlog is worse than a skipped tick for the jobs planned here.
type Runner struct {
	jobs   []Job
	logger *log.Logger

	wg      sync.WaitGroup
	cancel  context.CancelFunc
	mu      sync.Mutex
	started bool
}

// New builds a runner. A nil logger falls back to the standard logger.
func New(jobs []Job, logger *log.Logger) *Runner {
	if logger == nil {
		logger = log.Default()
	}
	return &Runner{jobs: jobs, logger: logger}
}

// Start launches one goroutine per job. Calling it twice is a no-op.
func (r *Runner) Start(ctx context.Context) {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return
	}
	r.started = true
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.mu.Unlock()

	for _, job := range r.jobs {
		switch {
		case job.Name == "" || job.Run == nil:
			r.logger.Printf("[WARN] worker: skipping a job without a name or Run func")
		case job.At != nil:
			if !job.At.Valid() {
				r.logger.Printf("[WARN] worker %s: invalid daily time %02d:%02d, skipped", job.Name, job.At.Hour, job.At.Minute)
				continue
			}
			r.wg.Add(1)
			go r.loopDaily(runCtx, job)
		case job.Every > 0:
			r.wg.Add(1)
			go r.loopEvery(runCtx, job)
		default:
			r.logger.Printf("[WARN] worker %s: needs either Every or At, skipped", job.Name)
		}
	}
}

// Stop cancels every job and waits for the running runs to finish. It is safe
// to call before Start or more than once.
func (r *Runner) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	r.cancel = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	r.wg.Wait()
}

func (r *Runner) loopEvery(ctx context.Context, job Job) {
	defer r.wg.Done()
	ticker := time.NewTicker(job.Every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runOnce(ctx, job)
		}
	}
}

func (r *Runner) loopDaily(ctx context.Context, job Job) {
	defer r.wg.Done()
	for {
		timer := time.NewTimer(time.Until(NextDaily(time.Now(), *job.At)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			r.runOnce(ctx, job)
		}
	}
}

func (r *Runner) runOnce(ctx context.Context, job Job) {
	start := time.Now()
	r.logger.Printf("[INFO] worker %s: start", job.Name)
	if err := job.Run(ctx); err != nil {
		r.logger.Printf("[ERROR] worker %s: %v (after %s)", job.Name, err, time.Since(start).Round(time.Millisecond))
		return
	}
	r.logger.Printf("[INFO] worker %s: done in %s", job.Name, time.Since(start).Round(time.Millisecond))
}

// NextDaily returns the next occurrence of at strictly after from, cut in
// Asia/Bangkok. A time later today returns today; otherwise tomorrow.
func NextDaily(from time.Time, at DailyAt) time.Time {
	local := from.In(timeutil.Bangkok())
	next := time.Date(local.Year(), local.Month(), local.Day(), at.Hour, at.Minute, 0, 0, timeutil.Bangkok())
	if !next.After(local) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
