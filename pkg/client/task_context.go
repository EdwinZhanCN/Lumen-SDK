package client

import "context"

type taskKey struct{}

// WithTask attaches an inference task name to the context. The lumenPicker
// reads it to route the RPC to a node that supports the requested task.
func WithTask(ctx context.Context, task string) context.Context {
	return context.WithValue(ctx, taskKey{}, task)
}

// TaskFromContext extracts the task name set by WithTask.
func TaskFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(taskKey{}).(string); ok {
		return v
	}
	return ""
}
