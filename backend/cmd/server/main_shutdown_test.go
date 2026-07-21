package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestActiveHandlerTrackerWaitsBeforeCleanupBoundary(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	tracker := newActiveHandlerTracker(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))

	done := make(chan struct{})
	go func() {
		tracker.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		close(done)
	}()
	<-started
	tracker.StopAccepting()

	waited := make(chan struct{})
	go func() {
		tracker.Wait()
		close(waited)
	}()
	select {
	case <-waited:
		t.Fatal("Wait returned while a handler was still active")
	case <-time.After(20 * time.Millisecond):
	}

	blocked := httptest.NewRecorder()
	tracker.ServeHTTP(blocked, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusServiceUnavailable, blocked.Code)

	close(release)
	select {
	case <-waited:
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after active handler completed")
	}
	<-done
}
