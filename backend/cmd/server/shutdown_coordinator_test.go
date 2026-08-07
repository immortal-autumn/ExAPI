package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestShutdownParentGraceCoversEverySequentialMaximum(t *testing.T) {
	require.Equal(t, 36*time.Second, shutdownSequentialMax)
	require.Greater(t, shutdownTotalGrace, shutdownSequentialMax)
	require.Equal(t, shutdownSequentialMax+shutdownParentSlack, shutdownTotalGrace)
}

func TestShutdownCoordinatorRunsOrderedPhases(t *testing.T) {
	var mu sync.Mutex
	var got []string
	step := func(name string) func(context.Context) []cleanupStep {
		return func(context.Context) []cleanupStep {
			return []cleanupStep{{name: name, fn: func() error {
				mu.Lock()
				defer mu.Unlock()
				got = append(got, name)
				return nil
			}}}
		}
	}
	coordinator := &shutdownCoordinator{
		producers:      shutdownPhase{steps: step("producers")},
		connections:    shutdownPhase{steps: step("connections")},
		workers:        shutdownPhase{steps: step("workers")},
		flushers:       shutdownPhase{steps: step("flushers")},
		services:       shutdownPhase{steps: step("services")},
		infrastructure: shutdownPhase{steps: step("infrastructure")},
	}

	coordinator.Run(context.Background())
	require.Equal(t, []string{"producers", "connections", "workers", "flushers", "services", "infrastructure"}, got)
}

func TestShutdownCoordinatorSkipsSharedCloseAfterIncompleteDrain(t *testing.T) {
	called := false
	coordinator := &shutdownCoordinator{
		services: shutdownPhase{steps: func(context.Context) []cleanupStep {
			called = true
			return nil
		}},
		infrastructure: shutdownPhase{steps: func(context.Context) []cleanupStep {
			called = true
			return nil
		}},
	}

	require.False(t, coordinator.Close(context.Background(), false))
	require.False(t, called)
}
