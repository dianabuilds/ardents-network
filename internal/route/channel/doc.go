// Package channel owns portable Route framing, bounded principal memory,
// scheduling, credit and byte accounting, parent exchanges and joined physical
// retirement. It consumes an already selected ordered transport and observes
// caller-supplied currentness; it grants no Network or Admission authority.
//
// New starts the sole reader after the caller's accepted handshake. Prepare
// and PrepareJoined defer that transfer until Start, so refusal can close an
// unused preparation. Callers supply a nonnil context, connection and Budget,
// their original finite deadline and already accepted byte allowance. Handlers
// are captured before reading starts; they own domain decisions and return
// genuine owner-backed results to the physical channel.
// Both OPEN directions share 256 ordinary child slots and two additional slots
// reserved for Issuer, Reachability, Introduction and Submission. Controls use
// ordinary slots first. Each original claim lasts through joined Lane Finish;
// the extra reserve never bypasses the shared principal's 1024-child bound.
// PrepareOpen binds child capacity synchronously outside the framing lock,
// before pipelined handshake input. ConstrainQueues applies a separate shared
// child-group queue without replacing principal accounting; it includes queued
// output headers/control and keeps finite CLOSE space until physical finish.
// ConstrainTraffic counts complete child input/output, the original OPEN and
// prepaid CLOSE in both directions. It binds the actual operation's fast output
// admission before physical writes. A private witness distinguishes refusal
// before output from an attempted physical failure; only the former can retire
// the limited child while preserving the ordinary Carrier and its siblings.
//
// Retire interrupts physical work. Done signals reader completion; callers
// still call Close to join writers and children and retain terminal failures
// before returning reservations. Lane Finish returns its bounded memory only
// after its operation joins. Shared Budget is one physical principal's memory
// owner; it is independent of Admission allowances and Hosting capacity.
// Close also joins physical-return callbacks already started by Lane Finish.
// FinishRole completes an outgoing nested forwarding role after its borrowers
// join: inner half-close, lower EOF, reverse reader and exact lower CLOSE(0).
// It shares the original one-second cleanup horizon and retains cancellation
// and all started physical failures. A reverse EOF without that peer terminal
// cannot discharge retirement. Physical Close still owns join and returns.
//
// Navigation: one Session owns the clusters below. Test naming follows
// docs/development/repository-layout.md; all tests are package channel
// behavior tests, and a test listed under several clusters verifies each of
// them.
//
//   - core: session.go, joined_session.go, role_frames.go, request_context.go.
//     Tests: session_lifecycle_test.go, join_input_test.go, role_frames_test.go,
//     open_lane_test.go, deadline_effect_test.go.
//   - lane: lane.go, record_capacity.go. Tests: lane_stop_test.go,
//     credit_batch_test.go, credit_close_test.go, credit_retirement_test.go,
//     nested_control_close_test.go, open_lane_test.go.
//   - output: frame_schedule.go, physical_output.go, physical_close.go,
//     framing_retirement.go, terminal_outcome.go. Tests:
//     frame_schedule_test.go, framing_retirement_test.go,
//     native_refusal_test.go, frame_admission_test.go,
//     credit_retirement_test.go (TerminalFailureStage classification).
//   - capacity: queue_budget.go, child_capacity.go, child_limits.go. Tests:
//     budget_test.go, child_capacity_test.go, child_limits_test.go,
//     child_queue_test.go.
//   - parent: parent_exchange.go. Tests: parent_exchange_test.go,
//     parent_allowance_test.go.
//   - retirement: role_retirement.go, peer_close.go. Tests:
//     role_retirement_test.go, peer_close_test.go, peer_close_writer_test.go,
//     close_collision_test.go, dedicated_close_test.go, eof_retirement_test.go,
//     refused_close_test.go.
package channel
