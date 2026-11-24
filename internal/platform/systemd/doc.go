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
//
// Responsibility clusters and their tests:
//   - bounded typed JSON property vocabulary: properties.go
//     (properties_test.go)
//   - fixed busctl manager observations and thread-pinned subprocess
//     mechanism: bus_linux.go, control_linux.go
//     (bus_native_linux_test.go; native linux)
//   - fresh Startup observation and cancellation-cause normalization:
//     ObserveStartup, startupBus, startupDialFailure and startupCallFailure
//     in startup_observation_linux.go (startup_cancellation_linux_test.go;
//     linux, startup_observation_native_linux_test.go,
//     source_reference_native_linux_test.go; native linux); the latter
//     compares the shared close-cause normalizer against the pinned library
//     sentinel despite its Source name
//   - native typed JSON projection: startupProperties, managerJSONValue and
//     managerJSONQuotedSize in startup_observation_linux.go
//     (startup_property_encoding_linux_test.go; linux)
//   - bounded native value validation: manager_values_linux.go
//     (manager_values_linux_test.go; linux)
//   - original private bus transport physical I/O: referenceTransport
//     Read/Write/Close in reference_transport_linux.go
//     (reference_transport_native_linux_test.go; native linux)
//   - closed per-purpose unit reply vocabularies: verifyManagerFrame,
//     verifyForwardingManagerFrame, verifyClosedManagerFrame and
//     verifyNodeManagerFrame in reference_transport_linux.go,
//     managerUnitReply in manager_reference_linux.go
//     (source_object_linux_test.go, descriptor_manager_linux_test.go,
//     join_manager_linux_test.go; linux)
//   - shared Node-slot absence observation and absence-only reply grammar:
//     node_slots_linux.go, nodeSlotsReply grammar in
//     reference_transport_linux.go (forwarding_object_linux_test.go,
//     node_slots_linux_test.go; linux)
//   - shared request-observation deadline and cancellation marker:
//     request_observation_linux.go (request_observation_linux_test.go; linux)
//   - platform infrastructure observation and sibling absence:
//     platform_linux.go (platform_linux_test.go; linux)
//   - prestart loaded-policy verification: infrastructure_prestart_linux.go
//     (infrastructure_prestart_linux_test.go; linux)
//
// manager_reference_linux.go (original manager-reference connection),
// endpoint_reference_linux.go (Endpoint unit reference retention), the
// per-role read-only runtime observers (forwarding_manager_linux.go,
// descriptor_manager_linux.go, join_manager_linux.go,
// introduction_manager_linux.go, source_manager_linux.go) and the
// infrastructure unit template (infrastructure_unit.go,
// infrastructure_unit_template.go, rendered for release infrastructure launch)
// carry no dedicated package behavior test; their reply vocabularies are
// covered by the clusters above.
package systemd

import "errors"

var (
	ErrInput       = errors.New("invalid Installation manager observation")
	ErrObservation = errors.New("installation manager observation differs")
	ErrUnavailable = errors.New("installation manager observation unavailable")
)
