package discovery

import (
	"context"
	"sync"
)

// CompositeResolver fans in events from multiple discovery backends so that
// mDNS, Broker push, and static nodes can run side by side. Backends emit
// nodes under distinct identities, so the downstream resolver/pool layers
// handle merging naturally.
type CompositeResolver struct {
	resolvers []NodeResolver
}

// NewCompositeResolver combines the given backends. Nil entries are dropped;
// a single backend is returned as-is.
func NewCompositeResolver(resolvers ...NodeResolver) NodeResolver {
	out := make([]NodeResolver, 0, len(resolvers))
	for _, r := range resolvers {
		if r != nil {
			out = append(out, r)
		}
	}
	if len(out) == 1 {
		return out[0]
	}
	return &CompositeResolver{resolvers: out}
}

// Watch starts every backend and merges their event channels. It fails if any
// backend fails to start; the merged channel closes when all backends close
// (all backends stop on ctx cancellation).
func (c *CompositeResolver) Watch(ctx context.Context) (<-chan NodeEvent, error) {
	channels := make([]<-chan NodeEvent, 0, len(c.resolvers))
	for _, r := range c.resolvers {
		ch, err := r.Watch(ctx)
		if err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}

	out := make(chan NodeEvent, 32)
	var wg sync.WaitGroup
	wg.Add(len(channels))
	for _, ch := range channels {
		go func(ch <-chan NodeEvent) {
			defer wg.Done()
			for ev := range ch {
				select {
				case out <- ev:
				case <-ctx.Done():
					return
				}
			}
		}(ch)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out, nil
}
