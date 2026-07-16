package service

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPaymentOrderExpiryLifecycleIsSingleStartAndTerminalStop(t *testing.T) {
	svc := NewPaymentOrderExpiryService(&PaymentService{}, time.Hour)
	svc.lifecycleMu.Lock()
	svc.started = true
	svc.running.Store(true)
	svc.lifecycleMu.Unlock()

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.Start()
		}()
	}
	wg.Wait()
	require.True(t, svc.Running())

	svc.Stop()
	svc.running.Store(false)
	require.False(t, svc.Running())
	svc.Start()
	require.False(t, svc.Running())
	svc.Stop()
}

func TestPaymentOrderExpiryStopBeforeStartIsTerminal(t *testing.T) {
	svc := NewPaymentOrderExpiryService(&PaymentService{}, time.Hour)
	svc.Stop()
	svc.Start()
	require.False(t, svc.Running())
}
