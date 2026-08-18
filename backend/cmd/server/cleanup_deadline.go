package main

import (
	"context"
	"sync"
)

type cleanupStep struct {
	name string
	fn   func() error
}

type cleanupResult struct {
	name string
	err  error
}

// runCleanupParallel starts all application cleanup steps and waits only until
// they finish or the caller-owned deadline expires. Step goroutines may outlive
// the wait when a legacy Stop method is uncooperative; callers must therefore
// make the infrastructure-close policy explicit.
func runCleanupParallel(ctx context.Context, steps []cleanupStep) ([]cleanupResult, bool) {
	results := make(chan cleanupResult, len(steps))
	var wg sync.WaitGroup
	for i := range steps {
		step := steps[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- cleanupResult{name: step.name, err: step.fn()}
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		close(results)
		out := make([]cleanupResult, 0, len(steps))
		for result := range results {
			out = append(out, result)
		}
		return out, true
	case <-ctx.Done():
		return nil, false
	}
}
