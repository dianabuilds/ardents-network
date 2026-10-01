//go:build linux

package service

// Binding exposes test-only Endpoint worker ownership until root fixtures
// verify that relation through a production operation.

// Binding exposes the retained authority seam of an opened stream so root
// fixtures can assert exact job ownership.
func (connection *Stream) Binding() Binding {
	if connection == nil {
		return nil
	}
	return connection.binding
}
