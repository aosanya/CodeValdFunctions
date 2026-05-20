package codevaldelfunctions

import (
	"context"

	"github.com/aosanya/CodeValdSharedLib/eventbus"
)

// CrossPublisher is implemented by the registrar; it lets domain logic publish
// events to CodeValdCross without importing the registrar package directly.
type CrossPublisher interface {
	Publish(ctx context.Context, e eventbus.Event) error
}
