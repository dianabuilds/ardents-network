//go:build linux

package endpoint

import "context"

// textOperationFlight records cancellation and completion of one Context
// operation. Publisher Introduction/Responder openings and withdrawal use the
// same lifetime shape; their separate lifecycle owners retain the pointers.
type textOperationFlight struct {
	context context.Context
	cancel  context.CancelFunc
	done    chan struct{}
}
