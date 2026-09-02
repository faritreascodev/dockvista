package broker_test

import (
	"testing"
	"time"

	"dockvista/internal/adapters/broker"
)

func TestBroker_FansOutToAllSubscribers(t *testing.T) {
	b := broker.New[int]()

	ch1, unsub1 := b.Subscribe()
	defer unsub1()
	ch2, unsub2 := b.Subscribe()
	defer unsub2()

	b.Publish(42)

	for _, ch := range []<-chan int{ch1, ch2} {
		select {
		case v := <-ch:
			if v != 42 {
				t.Fatalf("expected 42, got %d", v)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for published value")
		}
	}
}

func TestBroker_UnsubscribeStopsDelivery(t *testing.T) {
	b := broker.New[int]()

	ch, unsub := b.Subscribe()
	unsub()

	b.Publish(1)

	if _, open := <-ch; open {
		t.Fatal("expected channel to be closed after unsubscribe")
	}
}

func TestBroker_SlowSubscriberDoesNotBlockPublish(t *testing.T) {
	b := broker.New[int]()

	// Never drained — Publish must still return promptly for every value,
	// once the small buffer fills, instead of blocking on this subscriber.
	_, unsub := b.Subscribe()
	defer unsub()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			b.Publish(i)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a slow subscriber")
	}
}
