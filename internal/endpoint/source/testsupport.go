//go:build linux

package source

// The symbols in this file exist for the endpoint package's integration
// tests, which must force a Route's finite lifetime and verify handle
// identity guards without the production Handle exposing Close, Done, or its
// live slot. Production code retires route prefixes only through the
// Lifecycle retirement paths and must not call these functions. The deadcode
// allowlist tracks these symbols under "test fixture support" until the test
// audit slice reworks route-death and transplant simulation.

// TerminateRoute closes the Route held by handle, exactly like the retired
// endpoint-package test wrapper did. It reports the same handle-unavailable
// error as every other handle operation when the prefix is already gone.
func TerminateRoute(handle *Handle) error {
	prefix, err := handle.routePrefix()
	if err != nil {
		return err
	}
	return prefix.Close()
}

// RouteDone observes the finite lifetime of the Route held by handle. A
// handle whose prefix is already retired observes an immediately closed
// channel, matching the retired endpoint-package test wrapper.
func RouteDone(handle *Handle) <-chan struct{} {
	prefix, err := handle.routePrefix()
	if err != nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return prefix.Done()
}

// TransplantLive installs handle as the lifecycle's live slot so identity
// guards can be exercised against a trusted handle from another context. A
// nil handle clears the slot exactly like the retired whitebox field write.
func TransplantLive(lifecycle *Lifecycle, handle *Handle) {
	lifecycle.live = handle
}
