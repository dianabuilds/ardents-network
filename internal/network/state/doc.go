// Package state owns authenticated Network State admission, transition, and
// publication. Open verifies retained current, pending, and distribution state
// before starting any Source server or background work. Accept admits a
// current offline decision; Refresh resolves finite Source waves and may
// stage a pending one. Both obey the current/pending/conflict invariant and
// publish immutable decisions through the physical durable root.
//
// Epoch authenticates candidate bytes; Source owns finite TLS acquisition and
// distribution framing. State alone selects and commits the accepted decision.
// Closedprofile verifies the separate signed profile grammar, while State
// joins it to the current Epoch before exposing closed Route facts. Duty holds
// local Source exposure and serving-role conflicts. Process constraints are
// borrowed through PermitWork; Network owns no process-resource monitor.
// Readers receive copied views,
// and Wait and Close supervise and join the work owned by this State root.
//
// CurrentRuntime observes trusted time and joins the accepted profile
// with exact Node Records under one read lock. Its opaque view owns copied
// facts, rejects its zero value, and binds retained local duties to the signed
// identity, generation, family, assignment and Carrier. Member validity is
// separate from profile validity. Consumers retain Purpose, path selection and
// transport ownership, and must query the live State again at admission
// rechecks; retaining a view does not acquire a lease on future acceptance.
// CurrentRuntimeWitness additionally retains an opaque, locally checked
// original-owner refusal projection. It cannot replace the full observation,
// and synchronous retirement never changes or revives the immutable view.
// Linux NodeInspection separately authenticates retained signed/time/security
// history and stopped participation without opening runtime work. Its copied
// public binding facts grant no RuntimeView, producer or Installation right;
// original reader and descriptor joins precede returning its existing leases.
//
// Navigation: one aggregate (State) owns every cluster below; each
// cluster is a method or adapter family sharing its lock, commit boundary and
// joined retirement. The durable subpackage owns the physical State root.
// Test naming follows docs/development/repository-layout.md; a test listed
// under several clusters verifies each of them.
//
//   - C1 contract/config: contract.go, config_validation.go, runtime_state.go.
//     Tests: validation_test.go, selected_profile_test.go, state_test.go.
//   - C2 open/lifecycle: open.go, retained_open.go, lifecycle.go, scheduler.go.
//     Tests: retained_open_test.go, lifecycle_close_test.go, scheduler_test.go,
//     automatic_refresh_test.go, work_admission_test.go, state_test.go.
//   - C3 epoch authentication adapter: epoch.go, candidate.go, epoch_intake.go
//     (the closed AREP-v3 intake schema gate). Tests: epoch_intake_test.go.
//   - C4 intake/acceptance: offline_accept.go, closed_profile_accept.go,
//     epoch_history.go, pending.go, active_publication.go. Tests:
//     offline_accept_clock_test.go, offline_pending_consistency_test.go,
//     closed_profile_store_test.go, closed_profile_liveness_test.go,
//     closed_profile_role_join_test.go, golden_test.go, control_commit_test.go.
//   - C5 acquisition/wave: refresh.go, source_wave.go, source_wave_state.go,
//     attempts.go, outcomes.go, acquisition_history.go. Tests: refresh_test.go,
//     refresh_cycle_test.go, refresh_resume_test.go, fallback_test.go,
//     materialization_index_test.go, source_bootstrap_test.go,
//     acquisition_closure_test.go, outcomes_test.go, source_wave_guard_test.go,
//     source_wave_guard_failure_test.go, pending_completion_time_test.go.
//   - C6 distribution journal: control_state.go, control_decoding.go,
//     control_encoding.go (private ARDS1D4 grammar). Tests:
//     control_commit_test.go, control_cycle_test.go, state_test.go.
//   - C7 history/recovery: storage.go, recovery.go, retained_history.go;
//     readRetainedHistory is the shared read seam of repair and inspection.
//     Tests: retained_open_test.go, validation_test.go, state_test.go.
//   - C8 serving/participation: server.go, local_roles.go,
//     source_completion.go. Tests: source_participation_test.go,
//     source_predecessor_retirement_test.go, source_server_duty_test.go,
//     source_server_liveness_test.go, source_server_predecessor_test.go,
//     source_close_custody_test.go, source_completion_test.go,
//     source_exposure_capacity_test.go, local_roles_concurrency_test.go.
//   - C9 runtime observation: runtime_observation.go, runtime_witness.go,
//     accepted_observation.go, membership.go, snapshot_access.go. Tests:
//     runtime_witness_test.go, closed_runtime_view_test.go,
//     membership_transition_test.go, selected_profile_test.go.
//   - C10 clock/time: clock_observation.go. Tests: offline_accept_clock_test.go.
//   - C11 retained inspection (linux): node_inspection_linux.go (retained
//     test-only), source_inspection_linux.go, inspection_state_linux.go.
//     Tests: node_inspection_native_linux_test.go,
//     source_inspection_native_linux_test.go.
//   - C12 repair policy: source_repair.go, introduction_repair.go,
//     forwarding_repair.go, join_repair.go, descriptor_repair.go; the shared
//     nodeDutyMember binding is defined in forwarding_repair.go. Tests:
//     source_repair_test.go, introduction_repair_test.go,
//     forwarding_repair_test.go, join_repair_test.go, descriptor_repair_test.go.
//   - C13 repair adapters (linux): source_repair_linux.go,
//     introduction_repair_linux.go, forwarding_repair_linux.go,
//     join_repair_linux.go, descriptor_repair_linux.go. Tests:
//     source_repair_linux_test.go, introduction_repair_native_linux_test.go.
//
// Fixture and export helpers carry no behavior of their own: fixture_test.go,
// fixture_encoding_test.go, fixture_commitment_test.go, durable_test.go,
// closed_profile_test.go, epoch_intake_export_test.go,
// automatic_refresh_export_test.go. Package state tests exercise internal
// mechanisms; package state_test tests exercise the public behavior surface.
package state
