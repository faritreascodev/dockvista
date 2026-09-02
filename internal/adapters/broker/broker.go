// Package broker is a tiny in-process pub/sub used to fan a single upstream
// stream (the Docker daemon's event feed) out to many SSE clients without
// opening a daemon connection per browser tab.
package broker

import "sync"

// Broker fans values published on one side out to every current
// subscriber. Zero value is not usable; construct with New.
type Broker[T any] struct {
	mu   sync.Mutex
	subs map[chan T]struct{}
}

func New[T any]() *Broker[T] {
	return &Broker[T]{subs: make(map[chan T]struct{})}
}

// Subscribe returns a channel that receives every value published from this
// point on, and an unsubscribe func the caller must call exactly once
// (typically deferred) to stop receiving and let the channel be garbage
// collected.
func (b *Broker[T]) Subscribe() (<-chan T, func()) {
	ch := make(chan T, 16)

	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
	}
	return ch, unsubscribe
}

// Publish fans v out to every current subscriber. A subscriber that isn't
// keeping up gets this value dropped rather than blocking every other
// subscriber (and the publisher) behind it.
func (b *Broker[T]) Publish(v T) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.subs {
		select {
		case ch <- v:
		default:
		}
	}
}
