package service

import "sync"

// flight collapses concurrent calls that share a key into one execution.
// It exists so a burst of container events, a poll tick, and a handler can
// share a single daemon list instead of each opening their own.
type flight struct {
	mu    sync.Mutex
	calls map[string]*flightCall
}

type flightCall struct {
	done chan struct{}
	val  any
	err  error
}

func (f *flight) Do(key string, fn func() (any, error)) (any, error) {
	f.mu.Lock()
	if f.calls == nil {
		f.calls = make(map[string]*flightCall)
	}
	if c, ok := f.calls[key]; ok {
		f.mu.Unlock()
		<-c.done
		return c.val, c.err
	}
	c := &flightCall{done: make(chan struct{})}
	f.calls[key] = c
	f.mu.Unlock()

	c.val, c.err = fn()
	close(c.done)

	f.mu.Lock()
	delete(f.calls, key)
	f.mu.Unlock()
	return c.val, c.err
}
