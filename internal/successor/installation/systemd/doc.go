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
// Fresh Startup observation uses a separate read-only reply vocabulary on one
// bounded original connection. Nested lengths and duplicate dictionary keys
// are checked before decoding; reference-only clients still admit no arrays.
// Version and both complete fixed-object rounds are read anew on every call,
// with unique manager comparison before/after and joined physical cancellation.
// An original interrupted call retains its context cause instead of adding the
// closed-socket error induced by this observation's own transport interruption.
// EOF, protocol/identity errors and actual Close failures remain retained; a
// canceled read supplies no manager fact or successful partial observation.
// Callers decide the expected generation, protection and invocation. These
// observations and command results grant no Release, process, join or startup
// authority. Installation admits each effect and retains original pins/lease
// until physical join; this Module never chooses or authorizes a generation.
// The transitional Forwarding orchestration mechanisms (ForwardingStopped,
// ForwardingControl, ForwardingReplacement and the manager-declaration identity
// observer) are retired (ADR-0125, #604 FJD stage D); the platform-led Forwarding
// lifecycle owns installation, start, stop and replacement through the canonical
// platform unit. The transitional root NodeSlots custody wrapper retired with
// the last installed Descriptor input preflight (#604 FJD stage D, ADR-0125);
// the shared fixed Node service/socket slot absence observation remains for the
// platform sibling-absence check and the retained read-only role managers. Its
// absence-only reply grammar accepts no unit or job object. Native namespace,
// account, Release and transition decisions
// stay with each Installation owner. Each fixed reply vocabulary is bound before
// decoder startup. A Source connection refuses Forwarding objects; a Forwarding
// connection refuses Source, other units and job replies. This is native frame
// validation, not role dispatch or shared policy/authority.
// ForwardingRuntime independently observes only the fixed Forwarding unit and
// all five Node slots, without loading, retaining or mutating a unit. The
// platform-led retained-producer repair consumes it; it grants no Start/Stop,
// installed admission or original domain/process completion.
// SourcePreflight is a separate fixed read-only Source namespace consumer. Its
// original connection observes Version/UnitPath and exact missing-unit responses,
// without LoadUnit/RefUnit, mutation or reconnect. Copies share original caller,
// denial and Close; actual manager users and cancellation callbacks join before
// retained physical release. Source expected policy stays in infrastructure.
// SourceRuntime opens its own non-root read-only GetUnit/property connection;
// SourceControl opens a separate root connection, refs only the fixed Source
// unit to prevent inactive-unit collection, and retains that original reference
// until physical connection Close. Reference acquisition grants no permission
// or loaded-policy fact and cannot be used by preflight or non-root runtime.
// It permits one fixed Source Start
// and one cleanup Stop, both mode fail. It admits canonical native job replies
// only on that control connection. A job is asynchronous request evidence,
// never process identity or join. Copies share original attempt/Close state;
// cleanup cannot restore denied admission or reconnect. Native control and
// observations grant no Release, Startup or Source serving authority.
// The transitional JOIN orchestration mechanisms (JoinStopped, JoinControl,
// JoinReplacement and the manager-declaration identity observer) are retired
// (ADR-0125, #604 FJD stage D); the platform-led JOIN lifecycle owns
// installation, start, stop and replacement through the canonical platform
// unit. JoinRuntime independently observes only the fixed JOIN unit and the
// other fixed Node slots, without loading, retaining or mutating a unit. The
// platform-led retained-producer repair consumes it; it grants no Start/Stop,
// installed admission or original domain/process completion.
// Native typed JSON projection preserves complete property bytes, original
// depth/size limits and numeric byte arrays. Primitive arrays and safe-ASCII
// quoted lengths avoid redundant reflection/allocation without omitting facts
// or caching any observation.
// The transitional Descriptor orchestration mechanisms (DescriptorStopped,
// DescriptorControl, DescriptorReplacement and the manager-declaration identity
// observer) are retired (ADR-0125, #604 FJD stage D); the platform-led Descriptor
// lifecycle owns installation, start, stop and replacement through the canonical
// platform unit. DescriptorRuntime independently observes only the fixed
// Descriptor unit and the other fixed Node slots, without loading, retaining or
// mutating a unit. The platform-led retained-producer repair consumes it; it
// grants no Start/Stop, installed admission or original domain/process completion.
// Its literal unit and reply vocabulary grant no sibling role capability.
package systemd

import "errors"

var (
	ErrInput       = errors.New("invalid Installation manager observation")
	ErrObservation = errors.New("installation manager observation differs")
	ErrUnavailable = errors.New("installation manager observation unavailable")
)
