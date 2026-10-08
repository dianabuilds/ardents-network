// Package systemd owns bounded typed observations of the fixed Installation
// Endpoint and activation socket objects on the real system bus and the fixed
// daemon-reload/start/stop subprocess mechanisms.
// Each subprocess retains its creating OS thread through physical Run/join
// and receives kernel SIGKILL on that thread's death. This prevents a surviving
// helper from submitting a later effect after fatal caller death; it does not
// cancel or establish completion of an already accepted manager request.
// Its original private bus connection retains the fixed Endpoint unit through
// replacement and reload;
// Close interrupts and joins original transport I/O before releasing custody.
// The connection admits only bounded scalar replies and broker notifications,
// pins the original manager identity, and never reconnects. It preserves
// variant signatures, exact numbers, absence and duplicate-field refusal.
// Callers decide the expected generation, protection and invocation. These
// observations and command results grant no Release, process, join or startup
// authority. Installation admits each effect and retains original pins/lease
// until physical join; this Module never chooses or authorizes a generation.
package systemd

import "errors"

var (
	ErrInput       = errors.New("invalid Installation manager observation")
	ErrObservation = errors.New("installation manager observation differs")
	ErrUnavailable = errors.New("installation manager observation unavailable")
)
