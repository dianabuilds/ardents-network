// Package receiving owns receiver-side Admission decisions against current
// authenticated facts supplied by Network State. It selects the token's exact
// issuer key, receiver, class and hour; it grants no State authority and owns
// no transport, listener or provider resource budget. Redemption owns reserve-
// before-spend ordering, final expiry, and rollback without token refunds.
// OpenRetained selects the strict existing spend owner, without creating or
// repairing missing/damaged history or issuing a fresh-root fact.
// Original operations serialize observations and spends outside the local
// state mutex. Close denies admission before joining those users and their
// refusal cleanup; only the work owner releases a transferred reservation.
//
// Responsibility clusters and their tests:
//   - allowance arithmetic and verification gating: allowance.go,
//     verification_gate.go (allowance_test.go)
//   - token verification and current authority binding: verification.go,
//     current_authority.go (verification_test.go, with the independent test
//     key-encoding fixture verification_fixture_test.go)
//   - redemption reserve-before-spend ordering and no-refund expiry:
//     redemption.go (redemption_test.go)
//   - owner lifetime, synchronous denial and local command allowance:
//     owner.go, local_allowance.go, local_command.go (owner_lifetime_test.go)
//   - fresh-root fact non-survival across refusal and retained reopen:
//     owner.go (fresh_root_test.go)
//   - genuine signed Network State and Linux Hosting integration:
//     network_joined_work_linux_test.go, network_lifetime_linux_test.go,
//     network_reservation_linux_test.go, registration_allowance_linux_test.go;
//     linux
//   - registration initialization fact through the genuine Network receiving
//     owner: registration_initialization_integration_test.go; unix
package receiving
