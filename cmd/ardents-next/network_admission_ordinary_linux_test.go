//go:build linux && !text_worker_installed

package main

// Ordinary signed-Network tests have no installed receiving-root controls.
type networkAdmissionFixture = networkAdmissionFixtureState
