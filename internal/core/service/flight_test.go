package service

import (
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
