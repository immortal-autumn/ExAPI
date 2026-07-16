package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunCleanupParallelHonorsDeadlineForBlockedStep(t *testing.T) {
	blocked := make(chan struct{})
	defer close(blocked)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	started := time.Now()
	results, completed := runCleanupParallel(ctx, []cleanupStep{{name: "blocked", fn: func() error {
		<-blocked
		return nil
	}}})
	require.False(t, completed)
	require.Nil(t, results)
	require.Less(t, time.Since(started), 500*time.Millisecond)
}

func TestRunCleanupParallelCollectsResults(t *testing.T) {
	results, completed := runCleanupParallel(context.Background(), []cleanupStep{
		{name: "one", fn: func() error { return nil }},
		{name: "two", fn: func() error { return nil }},
	})
	require.True(t, completed)
	require.Len(t, results, 2)
}
