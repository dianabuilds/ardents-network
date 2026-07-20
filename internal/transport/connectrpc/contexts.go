package connectrpc

import "context"

func mutationContext(parent context.Context) (context.Context, context.CancelFunc) {
	base := context.WithoutCancel(parent)
	if deadline, ok := parent.Deadline(); ok {
		return context.WithDeadline(base, deadline)
	}
	return base, func() {}
}
