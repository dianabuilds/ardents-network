//go:build linux

// Package publication owns the Endpoint duty context's publication refresh
// scheduler mechanism: the single in-flight refresh identity, its wake-up,
// cancellation, joined terminal result, and the fixed-stage failure wrapper.
//
// The scheduler is pure mechanism. It retains no registration or pair state
// and calls back into no duty context. The endpoint root supplies the
// rotation callback to Start and observes the flight through the exported
// Refresh fields. The registration pair lifecycle and its Registration
// entity live in the introduction package; the root publication owner
// composes that pair with this scheduler.
package publication
