package workers

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireWorkerBlocksUntilRelease(t *testing.T) {
	originalPool := workerPool
	workerPool = make(chan struct{}, 1)
	t.Cleanup(func() {
		workerPool = originalPool
	})

	acquireWorker()

	acquired := make(chan struct{})
	go func() {
		acquireWorker()
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("expected acquireWorker to block while the pool is full")
	case <-time.After(30 * time.Millisecond):
	}

	releaseWorker()

	select {
	case <-acquired:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected acquireWorker to resume after releaseWorker")
	}

	releaseWorker()
}

func TestSetWorkerLimitReplacesPoolCapacity(t *testing.T) {
	originalPool := workerPool
	t.Cleanup(func() {
		workerPool = originalPool
	})

	SetWorkerLimit(3)

	if cap(workerPool) != 3 {
		t.Fatalf("expected worker pool capacity 3, got %d", cap(workerPool))
	}
}

func TestSetWorkerLimitUsesDefaultForInvalidValues(t *testing.T) {
	originalPool := workerPool
	t.Cleanup(func() {
		workerPool = originalPool
	})

	SetWorkerLimit(0)

	if cap(workerPool) != defaultWorkerLimit {
		t.Fatalf("expected default worker pool capacity %d, got %d", defaultWorkerLimit, cap(workerPool))
	}
}

func TestRunWorkersStartsTasksUsingWorkerPool(t *testing.T) {
	originalPool := workerPool
	originalStop := stopSignal
	originalRunning := flagRunning.Load()
	workerPool = make(chan struct{}, 1)
	t.Cleanup(func() {
		StopWorkers()
		workerPool = originalPool
		stopSignal = originalStop
		flagRunning.Store(originalRunning)
	})

	taskStarted := make(chan struct{})
	releaseTask := make(chan struct{})
	var started atomic.Int32

	go RunWorkers(func() {
		if started.Add(1) == 1 {
			close(taskStarted)
		}
		<-releaseTask
	})

	select {
	case <-taskStarted:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected RunWorkers to start at least one task")
	}

	if len(workerPool) != 1 {
		t.Fatalf("expected one worker slot to be acquired, got %d", len(workerPool))
	}

	close(releaseTask)
	StopWorkers()
}

func TestRunWorkersIgnoresSecondStartAndStopResetsState(t *testing.T) {
	originalPool := workerPool
	originalStop := stopSignal
	originalRunning := flagRunning.Load()
	workerPool = make(chan struct{}, 1)
	t.Cleanup(func() {
		StopWorkers()
		workerPool = originalPool
		stopSignal = originalStop
		flagRunning.Store(originalRunning)
	})

	releaseTask := make(chan struct{})
	var started atomic.Int32

	RunWorkers(func() {
		started.Add(1)
		<-releaseTask
	})

	time.Sleep(40 * time.Millisecond)
	RunWorkers(func() {
		started.Add(1)
	})

	time.Sleep(40 * time.Millisecond)
	if started.Load() != 1 {
		t.Fatalf("expected only one active worker loop, got %d task starts", started.Load())
	}

	close(releaseTask)
	StopWorkers()

	if flagRunning.Load() {
		t.Fatal("expected worker loop state to be reset after StopWorkers")
	}
}

func TestRunWorkersReturnsWhenTaskIsNil(t *testing.T) {
	originalRunning := flagRunning.Load()
	t.Cleanup(func() {
		flagRunning.Store(originalRunning)
	})

	flagRunning.Store(false)
	RunWorkers(nil)

	if flagRunning.Load() {
		t.Fatal("expected nil task to leave workers stopped")
	}
}
