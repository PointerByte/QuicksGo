// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package jobs provides in-process scheduling for periodic and trigger-aligned
// background jobs.
//
// The package supports two usage styles:
//   - instance-based scheduling with [newJobs]
//   - process-wide scheduling through the package-level helpers
//
// The package-level helpers are:
//   - [Job] to register fixed-interval work
//   - [CronJob] to register jobs aligned to a daily trigger
//   - [StartJobs] to start the global scheduler
//   - [RestartJobs] to restart global jobs without clearing their definitions
//   - [StopAllJobs] to stop global jobs, optionally clearing them
//   - [CheckStatusJobs] to inspect whether the global scheduler is active
//
// Important behavior:
//   - registering a job does not start it immediately
//   - jobs begin running when [StartJobs] is called
//   - jobs added after startup begin executing immediately
//   - if `server.modeTest=true`, [StartJobs] does not run jobs
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

var globalJobs ijobs
var checkJobsStop atomic.Bool

func init() {
	globalJobs = newJobs()
}

// cronTrigger defines the daily execution time for a CronJob using hour, minute,
// and second fields.
//
// Example:
//
//	trg := jobs.cronTrigger{Hour: 9, Minute: 0, Second: 0} // every day at 09:00:00
type cronTrigger struct {
	Hour   uint
	Minute uint
	Second uint
}

// ijobs defines the public operations used to schedule and start jobs.
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
type ijobs interface {
	// job schedules fn to run periodically every interval.
	// If timeout != nil and *timeout > 0, the job stops automatically when
	// the timeout expires. If timeout is nil, the job keeps running until
	// StopAllJobs, Destroy, or process shutdown.
	//
	// Example:
	//
	//	timeout := 10 * time.Second
	//	j.job(func() { fmt.Println("every 500ms") }, 500*time.Millisecond, &timeout)
	job(fn func(), interval time.Duration, timeout *time.Duration)

	// cronJob schedules fn to start at the time defined by trigger.
	// If interval <= 0, the job runs daily.
	// If interval > 0, the first execution is aligned to trigger and the job
	// then repeats every interval until the instance is stopped.
	//
	// Examples:
	//
	//  // 1) Daily schedule:
	//  j.cronJob(func() { fmt.Println("daily") },
	//      jobs.CronTrigger{Hour: 7, Minute: 45, Second: 0}, 0)
	//
	//  // 2) First run at 09:00, then every 5 seconds:
	//  j.cronJob(func() { fmt.Println("starts at 09:00 and then runs every 5s") },
	//      jobs.CronTrigger{Hour: 9, Minute: 0, Second: 0}, 5*time.Second)
	cronJob(fn func(), trigger cronTrigger, interval time.Duration)

	// startJobs starts all jobs registered in the instance.
	// The operation is idempotent: multiple calls do not duplicate running jobs.
	startJobs()
}

// jobs manages the lifecycle of periodic and cron jobs for a single instance.
// Create it with NewJobs and stop it with Destroy or globally with StopAllJobs.
type jobs struct {
	started atomic.Bool
	stopCh  atomic.Value

	mu           sync.Mutex
	intervalJobs []intervalJob
	cronJobs     []cronJob

	wg sync.WaitGroup
}

// newJobs creates and registers a new Jobs instance in the global registry.
// Any instance created with newJobs is affected by StopAllJobs.
//
// Example:
//
//	j := jobs.newJobs()
//	j.Job(func() { fmt.Println("hello") }, time.Second, nil)
//	j.StartJobs()
func newJobs() *jobs {
	j := &jobs{}
	register(j)
	return j
}

// job registers a job that runs fn every interval.
// If timeout is not nil and greater than zero, the job stops automatically
// when the timeout expires.
//
// If the instance has already been started, the job starts immediately.
// Otherwise, it is queued until StartJobs is called.
//
// Examples:
//
//	// 1) No timeout:
//	j.job(func() { fmt.Println("tick") }, 500*time.Millisecond, nil)
//
//	// 2) With timeout (stops after 5 seconds):
//	t := 5 * time.Second
//	j.job(func() { fmt.Println("timed tick") }, 500*time.Millisecond, &t)
func (j *jobs) job(fn func(), interval time.Duration, timeout *time.Duration) {
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

// Job registers a fixed-interval job in the package-level scheduler.
//
// The job runs once immediately when the scheduler starts, and then repeats
// every interval. If timeout is nil, the job keeps running until it is stopped
// by [StopAllJobs] or process shutdown. If timeout is non-nil and greater than
// zero, the job stops automatically when that duration expires.
//
// The job is only registered here; execution begins when [StartJobs] runs.
func Job(fn func(), interval time.Duration, timeout *time.Duration) {
	globalJobs.job(fn, interval, timeout)
}

// cronJob registers a job that starts according to the provided trigger.
// If the instance is already running, scheduling begins immediately and waits
// for the next trigger. Otherwise, the job is queued until StartJobs.
//
// Example:
//
//	j.cronJob(func() { fmt.Println("daily") },
//		jobs.CronTrigger{Hour: 7, Minute: 45, Second: 0})
func (j *jobs) cronJob(fn func(), trigger cronTrigger, interval time.Duration) {
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

// CronJob registers a trigger-aligned job in the package-level scheduler.
//
// If interval <= 0, the job runs once per day at the time defined by trigger.
// If interval > 0, the first execution waits until the next matching trigger,
// and the job then repeats every interval.
//
// The job is only registered here; execution begins when [StartJobs] runs.
func CronJob(fn func(), trigger cronTrigger, interval time.Duration) {
	globalJobs.cronJob(fn, trigger, interval)
}

var restartJobs chan struct{}

func init() {
	restartJobs = make(chan struct{})
	go RestartJobs()
}

// StartJobs starts the package-level jobs scheduler.
//
// Internally it waits for restart signals and starts the global job registry
// when requested. This is the entry point used by higher-level server packages
// such as `server_Gin.Start(...)`.
//
// When `server.modeTest=true`, registered jobs are not started.
func StartJobs() {
	go func() {
		for {
			select {
			case <-restartJobs:
				StopAllJobs(false)
				globalJobs.startJobs()
			default:
				if !CheckStatusJobs() {
					return
				}
				time.Sleep(time.Minute)
			}
		}
	}()
}

// RestartJobs requests a restart of the package-level scheduler.
//
// The restart flow stops currently running jobs without clearing their
// registered definitions and starts them again from the current process state.
func RestartJobs() {
	restartJobs <- struct{}{}
}

// startJobs starts all previously registered jobs.
// If the instance is already started, the call has no additional effect.
//
// Jobs added after startJobs begin executing immediately.
//
// Example:
//
//	j.startJobs()
//	j.Job(func() { fmt.Println("starts now") }, 200*time.Millisecond, nil) // starts immediately
func (j *jobs) startJobs() {
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

func (j *jobs) stop() {
	if j.started.CompareAndSwap(true, false) {
		if ch, _ := j.stopCh.Load().(chan struct{}); ch != nil {
			close(ch)
		}
		j.wg.Wait()
	}
}

// StopAndClear stops and clears only this instance.
// It is used internally by Destroy and global resets.
func (j *jobs) stopAndClear() {
	j.stop()

	j.mu.Lock()
	j.intervalJobs = nil
	j.cronJobs = nil
	j.mu.Unlock()
}

// destroy stops the instance jobs and removes the instance from the global registry.
// After destroy, StopAllJobs no longer affects the instance.
//
// Example:
//
//	j.destroy() // stops and unregisters this instance
func (j *jobs) destroy() {
	j.stopAndClear()
	unregister(j)
}

// StopAllJobs stops and clears all globally registered instances created with NewJobs.
// It is useful for coordinated shutdowns, tests, or global resets.
//
// Example:
//
//	jobs.StopAllJobs(true) // stops and clears all registered jobs in the process
// StopAllJobs stops every globally registered jobs instance.
//
// If clearJobs is false, the jobs stop but remain registered and can be
// started again later. If clearJobs is true, the jobs stop and their stored
// definitions are removed from each registered instance.
func StopAllJobs(clearJobs bool) {
	regMu.Lock()
	list := make([]*jobs, 0, len(registry))
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

// CheckStatusJobs reports whether the package-level scheduler is currently
// marked as active.
//
// It returns true after jobs have started and false after they have been fully
// stopped.
func CheckStatusJobs() bool {
	return checkJobsStop.Load()
}

// -------------------- Unexported internals --------------------

type intervalJob struct {
	fn       func()
	interval time.Duration
	timeout  *time.Duration
}

type cronJob struct {
	fn       func()
	trigger  cronTrigger
	interval time.Duration
}

var (
	regMu    sync.Mutex
	registry = make(map[*jobs]struct{})
)

func register(j *jobs) {
	regMu.Lock()
	registry[j] = struct{}{}
	regMu.Unlock()
}

func unregister(j *jobs) {
	regMu.Lock()
	delete(registry, j)
	regMu.Unlock()
}

func (j *jobs) startIntervalJob(ij intervalJob, stopCh chan struct{}) {
	j.wg.Go(func() {
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
	})
}

func (j *jobs) startCronJob(cj cronJob, stopCh chan struct{}) {
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

func nextDelay(trg cronTrigger, now time.Time) time.Duration {
	loc := now.Location()
	next := time.Date(now.Year(), now.Month(), now.Day(),
		int(trg.Hour), int(trg.Minute), int(trg.Second), 0, loc)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return time.Until(next)
}
