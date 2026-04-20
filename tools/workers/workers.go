package workers

import (
	"sync"
	"sync/atomic"
)

const defaultWorkerLimit = 1000

var (
	workerPool  chan struct{}
	stateMu     sync.Mutex
	stopSignal  chan struct{}
	flagRunning atomic.Bool
)

func init() {
	workerPool = make(chan struct{}, defaultWorkerLimit)
}

// acquireWorker blocks until a worker slot is available in the current pool.
func acquireWorker() {
	acquireWorkerFromPool(workerPool)
}

// releaseWorker releases a worker slot from the current pool.
func releaseWorker() {
	releaseWorkerFromPool(workerPool)
}

func acquireWorkerFromPool(pool chan struct{}) {
	pool <- struct{}{}
}

func releaseWorkerFromPool(pool chan struct{}) {
	<-pool
}

// SetWorkerLimit sets the maximum number of concurrent workers for future runs.
// Non-positive limits fall back to the default worker limit.
func SetWorkerLimit(limit int) {
	if limit <= 0 {
		limit = defaultWorkerLimit
	}
	workerPool = make(chan struct{}, limit)
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
func RunWorkers(task func()) {
	if task == nil || !flagRunning.CompareAndSwap(false, true) {
		return
	}

	pool := workerPool
	stop := make(chan struct{})

	stateMu.Lock()
	stopSignal = stop
	stateMu.Unlock()

	go func(pool chan struct{}, stop <-chan struct{}) {
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
			case pool <- struct{}{}:
				go func() {
					defer releaseWorkerFromPool(pool)
					task()
				}()
			}
		}
	}(pool, stop)
}
