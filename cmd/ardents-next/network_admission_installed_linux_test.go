//go:build linux && text_worker_installed

package main

// Installed receiving-root reopen checks retain their controls with that
// profile. The common signed-Network fixture supplies the same genuine owners.
type networkAdmissionFixture struct {
	networkAdmissionFixtureState
	receiverReopen bool
	afterRouteJoin []func() int
}
