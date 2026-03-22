// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func setupJobsTest(t *testing.T) {
	t.Helper()

	StopAllJobs(true)
	viper.Set("server.modeTest", false)

	t.Cleanup(func() {
		StopAllJobs(true)
		viper.Set("server.modeTest", false)
	})
}

func waitFor(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !check() {
		t.Fatalf("condition was not met within %s", timeout)
	}
}

func TestJobRunsWithIntervalAndTimeout(t *testing.T) {
	setupJobsTest(t)

	j := NewJobs()
	var hits int32

	timeout := 80 * time.Millisecond
	j.Job(func() {
		atomic.AddInt32(&hits, 1)
	}, 15*time.Millisecond, &timeout)

	j.StartJobs()

	waitFor(t, 120*time.Millisecond, func() bool {
		return atomic.LoadInt32(&hits) >= 2
	})

	waitFor(t, 250*time.Millisecond, func() bool {
		snapshot := atomic.LoadInt32(&hits)
		time.Sleep(50 * time.Millisecond)
		return atomic.LoadInt32(&hits) == snapshot
	})
}

func TestStartJobsIsIdempotentAndJobAddedAfterStartRunsImmediately(t *testing.T) {
	setupJobsTest(t)

	j := NewJobs()
	var beforeCount int32

	j.Job(func() {
		atomic.AddInt32(&beforeCount, 1)
	}, 200*time.Millisecond, nil)

	j.StartJobs()
	j.StartJobs()

	waitFor(t, 50*time.Millisecond, func() bool {
		return atomic.LoadInt32(&beforeCount) == 1
	})

	time.Sleep(40 * time.Millisecond)
	if got := atomic.LoadInt32(&beforeCount); got != 1 {
		t.Fatalf("expected StartJobs to be idempotent, got %d immediate executions", got)
	}

	var afterCount int32
	j.Job(func() {
		atomic.AddInt32(&afterCount, 1)
	}, 100*time.Millisecond, nil)

	waitFor(t, 50*time.Millisecond, func() bool {
		return atomic.LoadInt32(&afterCount) >= 1
	})
}

func TestCronJobRunsAtNextTrigger(t *testing.T) {
	setupJobsTest(t)

	j := NewJobs()
	j.StartJobs()

	var fired int32
	trigTime := time.Now().Add(2 * time.Second)
	trigger := CronTrigger{
		Hour:   uint(trigTime.Hour()),
		Minute: uint(trigTime.Minute()),
		Second: uint(trigTime.Second()),
	}

	j.CronJob(func() {
		atomic.AddInt32(&fired, 1)
	}, trigger, 0)

	waitFor(t, 4*time.Second, func() bool {
		return atomic.LoadInt32(&fired) >= 1
	})
}

func TestCronJobWithIntervalRunsAgainAfterFirstTrigger(t *testing.T) {
	setupJobsTest(t)

	j := NewJobs()
	j.StartJobs()

	var hits int32
	trigTime := time.Now().Add(2 * time.Second)
	trigger := CronTrigger{
		Hour:   uint(trigTime.Hour()),
		Minute: uint(trigTime.Minute()),
		Second: uint(trigTime.Second()),
	}

	j.CronJob(func() {
		atomic.AddInt32(&hits, 1)
	}, trigger, time.Second)

	waitFor(t, 5*time.Second, func() bool {
		return atomic.LoadInt32(&hits) >= 3
	})
}

func TestStopAllJobsStopsAllRegisteredInstances(t *testing.T) {
	setupJobsTest(t)

	a := NewJobs()
	b := NewJobs()

	var countA int32
	var countB int32

	a.Job(func() { atomic.AddInt32(&countA, 1) }, 20*time.Millisecond, nil)
	b.Job(func() { atomic.AddInt32(&countB, 1) }, 20*time.Millisecond, nil)

	a.StartJobs()
	b.StartJobs()

	waitFor(t, 120*time.Millisecond, func() bool {
		return atomic.LoadInt32(&countA) > 1 && atomic.LoadInt32(&countB) > 1
	})

	beforeA := atomic.LoadInt32(&countA)
	beforeB := atomic.LoadInt32(&countB)

	StopAllJobs(false)

	time.Sleep(80 * time.Millisecond)

	afterA := atomic.LoadInt32(&countA)
	afterB := atomic.LoadInt32(&countB)

	if afterA != beforeA {
		t.Fatalf("expected instance A to stop; before=%d after=%d", beforeA, afterA)
	}
	if afterB != beforeB {
		t.Fatalf("expected instance B to stop; before=%d after=%d", beforeB, afterB)
	}
}

func TestDestroyUnregistersInstanceFromGlobalStop(t *testing.T) {
	setupJobsTest(t)

	j := NewJobs()
	var hits int32

	j.Job(func() { atomic.AddInt32(&hits, 1) }, 15*time.Millisecond, nil)
	j.StartJobs()

	waitFor(t, 80*time.Millisecond, func() bool {
		return atomic.LoadInt32(&hits) >= 2
	})

	j.Destroy()

	var afterDestroyHits int32
	j.Job(func() { atomic.AddInt32(&afterDestroyHits, 1) }, 15*time.Millisecond, nil)
	j.StartJobs()
	defer j.Destroy()

	waitFor(t, 80*time.Millisecond, func() bool {
		return atomic.LoadInt32(&afterDestroyHits) >= 2
	})

	beforeGlobalStop := atomic.LoadInt32(&afterDestroyHits)
	StopAllJobs(true)
	time.Sleep(60 * time.Millisecond)
	afterGlobalStop := atomic.LoadInt32(&afterDestroyHits)

	if afterGlobalStop == beforeGlobalStop {
		t.Fatalf("expected unregistered instance to keep running after global stop; before=%d after=%d", beforeGlobalStop, afterGlobalStop)
	}
}

func TestStartJobsDoesNothingInModeTest(t *testing.T) {
	setupJobsTest(t)

	viper.Set("server.modeTest", true)

	j := NewJobs()
	var hits int32

	j.Job(func() { atomic.AddInt32(&hits, 1) }, 15*time.Millisecond, nil)
	j.StartJobs()

	time.Sleep(60 * time.Millisecond)

	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Fatalf("expected no executions in mode test, got %d", got)
	}
}
