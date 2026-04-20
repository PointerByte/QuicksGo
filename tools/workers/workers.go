package workers

import (
	"sync"
	"sync/atomic"
)

const defaultWorkerLimit = 1000

var (
	workerPool  chan func()
	stateMu     sync.Mutex
	stopSignal  chan struct{}
	flagRunning atomic.Bool
)

func init() {
	workerPool = make(chan func(), defaultWorkerLimit)
}

func AddTask(task func()) {
	workerPool <- task
}

// SetWorkerLimit sets the maximum number of concurrent workers for future runs.
// Non-positive limits fall back to the default worker limit.
func SetWorkerLimit(limit int) {
	if limit <= 0 {
		limit = defaultWorkerLimit
	}
	workerPool = make(chan func(), limit)
}

// StopWorkers stops the currently running worker loop, if any.
func StopWorkers() {
	stateMu.Lock()
	defer stateMu.Unlock()

	if stopSignal != nil {
		close(stopSignal)
		stopSignal = nil
	}
	flagRunning.Store(false)
}

// RunWorkers starts a managed worker loop for the given task if one is not already running.
// Each started task still respects the configured worker pool limit.
func RunWorkers() {
	if !flagRunning.CompareAndSwap(false, true) {
		return
	}

	pool := workerPool
	stop := make(chan struct{})

	stateMu.Lock()
	stopSignal = stop
	stateMu.Unlock()

	go func(stop <-chan struct{}) {
		defer func() {
			stateMu.Lock()
			if stopSignal != nil {
				stopSignal = nil
			}
			stateMu.Unlock()
			flagRunning.Store(false)
		}()

		for {
			select {
			case <-stop:
				return
			case task := <-pool:
				go task()
			}
		}
	}(stop)
}
