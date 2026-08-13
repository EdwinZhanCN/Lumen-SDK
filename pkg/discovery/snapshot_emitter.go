package discovery

import (
	"context"
	"reflect"
	"sort"
	"sync"
)

// snapshotEmitter decouples source reconciliation from event delivery. A slow
// consumer can delay individual events, but it cannot block query completion
// or prevent the desired snapshot from advancing. When delivery resumes, the
// emitter diffs against the newest complete state and converges to it.
type snapshotEmitter struct {
	mu      sync.Mutex
	desired map[string]ResolvedNode
	wake    chan struct{}
}

func newSnapshotEmitter() *snapshotEmitter {
	return &snapshotEmitter{
		desired: make(map[string]ResolvedNode),
		wake:    make(chan struct{}, 1),
	}
}

func (e *snapshotEmitter) replace(snapshot map[string]ResolvedNode) {
	copySnapshot := cloneResolvedMap(snapshot)
	e.mu.Lock()
	e.desired = copySnapshot
	e.mu.Unlock()
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

func (e *snapshotEmitter) run(ctx context.Context, out chan<- NodeEvent) {
	published := make(map[string]ResolvedNode)
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.wake:
		}

		for {
			desired := e.snapshot()
			event, ok := nextSnapshotEvent(published, desired)
			if !ok {
				break
			}
			select {
			case <-ctx.Done():
				return
			case out <- event:
			}
			switch event.Type {
			case NodeExpired:
				delete(published, event.Resolved.Key())
			case NodeDiscovered:
				published[event.Resolved.Key()] = event.Resolved.Normalized()
			}
		}
	}
}

func (e *snapshotEmitter) snapshot() map[string]ResolvedNode {
	e.mu.Lock()
	defer e.mu.Unlock()
	return cloneResolvedMap(e.desired)
}

func nextSnapshotEvent(published, desired map[string]ResolvedNode) (NodeEvent, bool) {
	keys := make([]string, 0, len(published)+len(desired))
	seen := make(map[string]struct{}, len(published)+len(desired))
	for key := range published {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range desired {
		if _, ok := seen[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	// Revoke absent identities before publishing additions or replacements so
	// endpoint ownership cannot transiently grow without bound.
	for _, key := range keys {
		previous, existed := published[key]
		if _, retained := desired[key]; existed && !retained {
			return eventFromResolved(NodeExpired, previous), true
		}
	}
	for _, key := range keys {
		current, exists := desired[key]
		if !exists {
			continue
		}
		previous, publishedAlready := published[key]
		if !publishedAlready || !reflect.DeepEqual(previous.Normalized(), current.Normalized()) {
			return eventFromResolved(NodeDiscovered, current), true
		}
	}
	return NodeEvent{}, false
}

func cloneResolvedMap(in map[string]ResolvedNode) map[string]ResolvedNode {
	out := make(map[string]ResolvedNode, len(in))
	for key, node := range in {
		copyNode := node.Normalized()
		copyNode.Addresses = append([]string(nil), copyNode.Addresses...)
		copyNode.Sources = append([]string(nil), copyNode.Sources...)
		copyNode.Txt = copyStringMap(copyNode.Txt)
		out[key] = copyNode
	}
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
