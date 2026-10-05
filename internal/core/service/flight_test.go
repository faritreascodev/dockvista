package service

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFlight_CollapsesConcurrentCalls(t *testing.T) {
	var f flight
	var calls atomic.Int32
	var once sync.Once
	entered := make(chan struct{})
	release := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(8)
	for i := 0; i < 8; i++ {
		go func() {
			defer wg.Done()
			_, err := f.Do("containers", func() (any, error) {
				calls.Add(1)
				once.Do(func() { close(entered) })
				<-release
				return nil, nil
			})
			if err != nil {
				t.Errorf("Do: %v", err)
			}
		}()
	}

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("call never started")
	}
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 execution, got %d", got)
	}
}

func TestFlight_PanicReleasesWaitersWithError(t *testing.T) {
	var f flight
	entered := make(chan struct{})
	release := make(chan struct{})

	leaderDone := make(chan any, 1)
	go func() {
		defer func() { leaderDone <- recover() }()
		_, _ = f.Do("k", func() (any, error) {
			close(entered)
			<-release
			panic("boom")
		})
	}()
	<-entered

	waiterErr := make(chan error, 1)
	go func() {
		_, err := f.Do("k", func() (any, error) { return "should not run", nil })
		waiterErr <- err
	}()
	time.Sleep(20 * time.Millisecond)
	close(release)

	if r := <-leaderDone; r == nil {
		t.Fatal("leader did not panic")
	}
	select {
	case err := <-waiterErr:
		if !errors.Is(err, errFlightAborted) {
			t.Fatalf("waiter error = %v, want errFlightAborted", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter was never released")
	}

	// The key must be free again after a panic.
	if v, err := f.Do("k", func() (any, error) { return 7, nil }); err != nil || v != 7 {
		t.Fatalf("after panic Do = (%v, %v), want (7, nil)", v, err)
	}
}
