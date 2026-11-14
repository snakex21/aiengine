// tasks.go - a queue of functions posted from other goroutines (the agent
// bridge's HTTP server) and run on the main thread once per frame, so game
// and console state are only ever touched by the main loop.

package main

import "sync"

type taskQueue struct {
	mu      sync.Mutex
	pending []func()
	running []func()
}

// post queues fn for the next run; safe from any goroutine.
func (q *taskQueue) post(fn func()) {
	q.mu.Lock()
	q.pending = append(q.pending, fn)
	q.mu.Unlock()
}

// run executes the queued functions on the calling (main) goroutine.
func (q *taskQueue) run() {
	q.mu.Lock()
	q.running, q.pending = q.pending, q.running[:0]
	q.mu.Unlock()
	for i, fn := range q.running {
		fn()
		q.running[i] = nil
	}
}
