//go:build linux

package service

// The remaining helpers expose test-only Endpoint worker ownership and
// terminal diagnostics until their root fixtures use observable outcomes.

// Binding exposes the retained authority seam of an opened stream so root
// fixtures can assert exact job ownership.
func (connection *Stream) Binding() Binding {
	if connection == nil {
		return nil
	}
	return connection.binding
}

// RunErr returns the internal terminal cause. Read it only after Finished
// has closed.
func (connection *Stream) RunErr() error { return connection.runErr }

// FinishErr returns the joined cleanup and native retirement result. Read it
// only after Finished has closed.
func (connection *Stream) FinishErr() error { return connection.finishErr }
