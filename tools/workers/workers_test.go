package workers

import (
	"sync/atomic"
	"testing"
	"time"
)

func resetWorkerState(t *testing.T, capacity int) {
	t.Helper()

	originalPool := workerPool
	originalStop := stopSignal
	originalRunning := flagRunning.Load()

	workerPool = make(chan func(), capacity)
	stopSignal = nil
	flagRunning.Store(false)

	t.Cleanup(func() {
		StopWorkers()
		workerPool = originalPool
		stopSignal = originalStop
		flagRunning.Store(originalRunning)
	})
}

func TestAddTaskBlocksWhenPoolIsFullUntilDispatcherConsumesTask(t *testing.T) {
	resetWorkerState(t, 1)

	secondTaskQueued := make(chan struct{})

	AddTask(func() {})

	go func() {
		AddTask(func() {})
		close(secondTaskQueued)
	}()

	select {
	case <-secondTaskQueued:
		t.Fatal("expected AddTask to block while the pool is full")
	case <-time.After(30 * time.Millisecond):
	}

	RunWorkers()

	select {
	case <-secondTaskQueued:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected AddTask to resume after RunWorkers consumes from the pool")
	}
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

func TestRunWorkersExecutesQueuedTasks(t *testing.T) {
	resetWorkerState(t, 2)

	taskStarted := make(chan struct{})
	AddTask(func() {
		close(taskStarted)
	})

	RunWorkers()

	select {
	case <-taskStarted:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected queued task to run")
	}
}

func TestRunWorkersIgnoresSecondStartAndStopResetsState(t *testing.T) {
	resetWorkerState(t, 1)

	RunWorkers()
	firstStop := stopSignal
	if firstStop == nil {
		t.Fatal("expected stop signal to be initialized")
	}

	RunWorkers()

	if stopSignal != firstStop {
		t.Fatal("expected second RunWorkers call to keep the existing worker loop")
	}

	StopWorkers()

	if flagRunning.Load() {
		t.Fatal("expected workers to be marked as stopped")
	}
	if stopSignal != nil {
		t.Fatal("expected stop signal to be cleared")
	}
}

func TestRunWorkersProcessesMultipleTasks(t *testing.T) {
	resetWorkerState(t, 3)

	var executed atomic.Int32
	done := make(chan struct{})

	for range 3 {
		AddTask(func() {
			if executed.Add(1) == 3 {
				close(done)
			}
		})
	}

	RunWorkers()

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("expected all tasks to execute, got %d", executed.Load())
	}
}
