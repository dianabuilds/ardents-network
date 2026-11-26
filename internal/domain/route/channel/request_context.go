package channel

import "context"

// requestContext retains both the original request and the owner's derived
// interruption. A delayed cancellation callback cannot resurrect the request.
type requestContext struct {
	context.Context
	caller context.Context
}

// WithRequest retains synchronous cancellation of the exact original caller
// alongside the separately owned physical interruption context.
func WithRequest(caller, interruption context.Context) context.Context {
	return requestContext{Context: interruption, caller: caller}
}

func (c requestContext) Err() error {
	if err := c.caller.Err(); err != nil {
		return err
	}
	return c.Context.Err()
}
