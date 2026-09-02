package docker

import (
	"context"
	"time"

	dockertypes "github.com/docker/docker/api/types"

	"dockvista/internal/core/domain"
)

// Events subscribes to the daemon's event feed and maps each message into
// domain.Event. Unlike the other methods here, this isn't bounded by
// defaultCallTimeout — the caller's context is the only thing that ends it,
// since it's meant to live for the process's lifetime.
func (c *Client) Events(ctx context.Context) (<-chan domain.Event, <-chan error) {
	msgs, sdkErrs := c.sdk.Events(ctx, dockertypes.EventsOptions{})

	out := make(chan domain.Event)
	errs := make(chan error, 1)

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				select {
				case out <- domain.Event{
					Type:       string(msg.Type),
					Action:     string(msg.Action),
					ActorID:    msg.Actor.ID,
					Attributes: msg.Actor.Attributes,
					Time:       time.Unix(0, msg.TimeNano).UTC(),
				}:
				case <-ctx.Done():
					return
				}
			case err, ok := <-sdkErrs:
				if ok && err != nil {
					errs <- wrapErr("stream daemon events", err)
				}
				return
			}
		}
	}()

	return out, errs
}
