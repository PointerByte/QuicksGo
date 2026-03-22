// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package jobs provides concurrent execution of periodic and daily scheduled
// jobs.
//
// The package supports two usage styles:
//   - instance-based scheduling with [NewJobs]
//   - process-wide scheduling through the package-level helpers
//
// Basic usage:
//
//	j := jobs.NewJobs()
//	j.Job(func() { fmt.Println("tick") }, time.Second, nil) // no timeout
//	j.StartJobs()
//	// ...
//	jobs.StopAllJobs(true) // stops and clears all registered instances
package jobs

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/viper"
)

var globalJobs IJobs
var checkJobsStop atomic.Bool

func init() {
	globalJobs = NewJobs()
}

// CronTrigger defines the daily execution time for a CronJob using hour, minute,
// and second fields.
//
// Example:
//
//	trg := jobs.CronTrigger{Hour: 9, Minute: 0, Second: 0} // every day at 09:00:00
type CronTrigger struct {
	Hour   uint
	Minute uint
	Second uint
}

// IJobs defines the public operations used to schedule and start jobs.
// It is implemented by *Jobs.
//
// Recommendation: create instances with NewJobs().
//
// Example:
//
//	j := jobs.NewJobs()
//	j.Job(func() { /* ... */ }, 2*time.Second, nil)
//	j.CronJob(func() { /* ... */ }, jobs.CronTrigger{Hour: 9}, 0)
//	j.StartJobs()
type IJobs interface {
	// Job schedules fn to run periodically every interval.
	// If timeout != nil and *timeout > 0, the job stops automatically when
	// the timeout expires. If timeout is nil, the job keeps running until
	// StopAllJobs, Destroy, or process shutdown.
	//
	// Example:
	//
	//	timeout := 10 * time.Second
	//	j.Job(func() { fmt.Println("every 500ms") }, 500*time.Millisecond, &timeout)
	Job(fn func(), interval time.Duration, timeout *time.Duration)

	// CronJob schedules fn to start at the time defined by trigger.
	// If interval <= 0, the job runs daily.
	// If interval > 0, the first execution is aligned to trigger and the job
	// then repeats every interval until the instance is stopped.
	//
	// Examples:
	//
	//  // 1) Daily schedule:
	//  j.CronJob(func() { fmt.Println("daily") },
	//      jobs.CronTrigger{Hour: 7, Minute: 45, Second: 0}, 0)
	//
	//  // 2) First run at 09:00, then every 5 seconds:
	//  j.CronJob(func() { fmt.Println("starts at 09:00 and then runs every 5s") },
	//      jobs.CronTrigger{Hour: 9, Minute: 0, Second: 0}, 5*time.Second)
	CronJob(fn func(), trigger CronTrigger, interval time.Duration)

	// StartJobs starts all jobs registered in the instance.
	// The operation is idempotent: multiple calls do not duplicate running jobs.
	StartJobs()
}

// Jobs manages the lifecycle of periodic and cron jobs for a single instance.
// Create it with NewJobs and stop it with Destroy or globally with StopAllJobs.
type Jobs struct {
	_       noCopy
	started atomic.Bool
	stopCh  atomic.Value

	mu           sync.Mutex
	intervalJobs []intervalJob
	cronJobs     []cronJob

	wg sync.WaitGroup
}

// NewJobs creates and registers a new Jobs instance in the global registry.
// Any instance created with NewJobs is affected by StopAllJobs.
//
// Example:
//
//	j := jobs.NewJobs()
//	j.Job(func() { fmt.Println("hello") }, time.Second, nil)
//	j.StartJobs()
func NewJobs() *Jobs {
	j := &Jobs{}
	register(j)
	return j
}

// Job registers a job that runs fn every interval.
// If timeout is not nil and greater than zero, the job stops automatically
// when the timeout expires.
//
// If the instance has already been started, the job starts immediately.
// Otherwise, it is queued until StartJobs is called.
//
// Examples:
//
//	// 1) No timeout:
//	j.Job(func() { fmt.Println("tick") }, 500*time.Millisecond, nil)
//
//	// 2) With timeout (stops after 5 seconds):
//	t := 5 * time.Second
//	j.Job(func() { fmt.Println("timed tick") }, 500*time.Millisecond, &t)
func (j *Jobs) Job(fn func(), interval time.Duration, timeout *time.Duration) {
	if fn == nil || interval <= 0 {
		return
	}
	ij := intervalJob{fn: fn, interval: interval, timeout: timeout}

	if j.started.Load() {
		if ch, _ := j.stopCh.Load().(chan struct{}); ch != nil {
			j.startIntervalJob(ij, ch)
		}
		return
	}

	j.mu.Lock()
	j.intervalJobs = append(j.intervalJobs, ij)
	j.mu.Unlock()
}

func Job(fn func(), interval time.Duration, timeout *time.Duration) {
	globalJobs.Job(fn, interval, timeout)
}

// CronJob registers a job that starts according to the provided trigger.
// If the instance is already running, scheduling begins immediately and waits
// for the next trigger. Otherwise, the job is queued until StartJobs.
//
// Example:
//
//	j.CronJob(func() { fmt.Println("daily") },
//		jobs.CronTrigger{Hour: 7, Minute: 45, Second: 0})
func (j *Jobs) CronJob(fn func(), trigger CronTrigger, interval time.Duration) {
	if fn == nil {
		return
	}
	cj := cronJob{fn: fn, trigger: trigger, interval: interval}

	if j.started.Load() {
		if ch, _ := j.stopCh.Load().(chan struct{}); ch != nil {
			j.startCronJob(cj, ch)
		}
		return
	}

	j.mu.Lock()
	j.cronJobs = append(j.cronJobs, cj)
	j.mu.Unlock()
}

func CronJob(fn func(), trigger CronTrigger, interval time.Duration) {
	globalJobs.CronJob(fn, trigger, interval)
}

var restartJobs chan struct{}

func init() {
	restartJobs = make(chan struct{})
	go RestartJobs()
}

func StartJobs() {
	go func() {
		for {
			select {
			case <-restartJobs:
				StopAllJobs(false)
				globalJobs.StartJobs()
			default:
				if !CheckStatusJobs() {
					return
				}
				time.Sleep(time.Minute)
			}
		}
	}()
}

func RestartJobs() {
	restartJobs <- struct{}{}
}

// StartJobs starts all previously registered jobs.
// If the instance is already started, the call has no additional effect.
//
// Jobs added after StartJobs begin executing immediately.
//
// Example:
//
//	j.StartJobs()
//	j.Job(func() { fmt.Println("starts now") }, 200*time.Millisecond, nil) // starts immediately
func (j *Jobs) StartJobs() {
	if viper.GetBool("server.modeTest") {
		return
	}
	if !j.started.CompareAndSwap(false, true) {
		return
	}
	ch := make(chan struct{})
	j.stopCh.Store(ch)

	j.mu.Lock()
	intervals := append([]intervalJob(nil), j.intervalJobs...)
	crons := append([]cronJob(nil), j.cronJobs...)
	j.mu.Unlock()

	for _, ij := range intervals {
		j.startIntervalJob(ij, ch)
	}
	for _, cj := range crons {
		j.startCronJob(cj, ch)
	}

	checkJobsStop.Store(true)
}

func (j *Jobs) stop() {
	if j.started.CompareAndSwap(true, false) {
		if ch, _ := j.stopCh.Load().(chan struct{}); ch != nil {
			close(ch)
		}
		j.wg.Wait()
	}
}

// StopAndClear stops and clears only this instance.
// It is used internally by Destroy and global resets.
func (j *Jobs) stopAndClear() {
	j.stop()

	j.mu.Lock()
	j.intervalJobs = nil
	j.cronJobs = nil
	j.mu.Unlock()
}

// Destroy stops the instance jobs and removes the instance from the global registry.
// After Destroy, StopAllJobs no longer affects the instance.
//
// Example:
//
//	j.Destroy() // stops and unregisters this instance
func (j *Jobs) Destroy() {
	j.stopAndClear()
	unregister(j)
}

// StopAllJobs stops and clears all globally registered instances created with NewJobs.
// It is useful for coordinated shutdowns, tests, or global resets.
//
// Example:
//
//	jobs.StopAllJobs(true) // stops and clears all registered jobs in the process
func StopAllJobs(clearJobs bool) {
	regMu.Lock()
	list := make([]*Jobs, 0, len(registry))
	for j := range registry {
		list = append(list, j)
	}
	regMu.Unlock()

	for _, j := range list {
		if clearJobs {
			j.stopAndClear()
		} else {
			j.stop()
		}
	}

	checkJobsStop.Store(false)
}

func CheckStatusJobs() bool {
	return checkJobsStop.Load()
}

// -------------------- Unexported internals --------------------

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

type intervalJob struct {
	fn       func()
	interval time.Duration
	timeout  *time.Duration
}

type cronJob struct {
	fn       func()
	trigger  CronTrigger
	interval time.Duration
}

var (
	regMu    sync.Mutex
	registry = make(map[*Jobs]struct{})
)

func register(j *Jobs) {
	regMu.Lock()
	registry[j] = struct{}{}
	regMu.Unlock()
}

func unregister(j *Jobs) {
	regMu.Lock()
	delete(registry, j)
	regMu.Unlock()
}

func (j *Jobs) startIntervalJob(ij intervalJob, stopCh chan struct{}) {
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()

		ticker := time.NewTicker(ij.interval)
		defer ticker.Stop()

		var timeoutCh <-chan time.Time
		if ij.timeout != nil && *ij.timeout > 0 {
			timer := time.NewTimer(*ij.timeout)
			timeoutCh = timer.C
			defer timer.Stop()
		}

		// Run once immediately.
		ij.fn()

		for {
			select {
			case <-ticker.C:
				ij.fn()
			case <-timeoutCh:
				// Timeout expired: stop the job automatically.
				return
			case <-stopCh:
				return
			}
		}
	}()
}

func (j *Jobs) startCronJob(cj cronJob, stopCh chan struct{}) {
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()

		// Case 1: no interval provided, so the job runs daily.
		if cj.interval <= 0 {
			for {
				delay := nextDelay(cj.trigger, time.Now())
				timer := time.NewTimer(delay)

				select {
				case <-timer.C:
					cj.fn()
				case <-stopCh:
					if !timer.Stop() {
						// No channel drain is needed here.
					}
					return
				}
			}
		}

		// Case 2: align the first execution with the trigger,
		// then repeat every cj.interval.
		delay := nextDelay(cj.trigger, time.Now())
		timer := time.NewTimer(delay)

		select {
		case <-timer.C:
			cj.fn()
		case <-stopCh:
			if !timer.Stop() {
				// No channel drain is needed here.
			}
			return
		}

		ticker := time.NewTicker(cj.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cj.fn()
			case <-stopCh:
				if !timer.Stop() {
					// No channel drain is needed here.
				}
				return
			}
		}
	}()
}

func nextDelay(trg CronTrigger, now time.Time) time.Duration {
	loc := now.Location()
	next := time.Date(now.Year(), now.Month(), now.Day(),
		int(trg.Hour), int(trg.Minute), int(trg.Second), 0, loc)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return time.Until(next)
}
