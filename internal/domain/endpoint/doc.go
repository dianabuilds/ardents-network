// Package endpoint owns one original installed participant: Startup, local
// Execution, State, permission exchange, shared funding parents and IPC children.
// It joins children before returning their original roots. Domain authority and
// policy remain with Installation, Execution, Network, Admission and Service owners.
// Explicit offline first history preparation authenticates signed State/profile
// and initializes participation, Route selection and Stock; ordinary runtime
// opens retained only. The authenticated plan constructor renders preparation
// plans from an already accepted retained State's pins plus bounded operator
// declarations; it creates no State, permission or runtime authority.
// The only command consumer is ardents-next; schemas are bound Installation input,
// while arguments, diagnostics and process exits remain with that command.
//
// Start at command for CLI preparation, installed start and bounded reports.
// history_preparation_linux.go owns the first signed State and local history
// effects. launch authenticates and retains the original installed startup.
// participant_linux.go owns the shared funding and Execution lifetime and
// joins Reader, Publisher and local IPC before returning those owners.
// internal/permission owns original file exchange, while Stock authenticates
// its response under the participant's live Startup check.
//
// Responsibility clusters and their tests:
//   - declaration binding: declarations.go (declarations_test.go; portable);
//     declarations_linux.go projects installed identity and configuration
//   - participant execution and stock: participant_linux.go
//     (participant_linux_test.go, participant_stock_linux_test.go; linux)
//   - explicit first history preparation: history_preparation_linux.go
//     (history_preparation_linux_test.go; linux)
//   - authenticated plan construction and file entries:
//     plan_construction_linux.go, plan_delivery_linux.go
//     (plan_construction_linux_test.go; linux)
//   - source trust preparation: source_trust_preparation_linux.go
//     (source_trust_preparation_linux_test.go; linux,
//     source_trust_native_linux_test.go; native)
//   - installed connection over the joined network, served through the
//     session and publisher children by participant_linux.go
//     (connection_accepted_request_installed_linux_test.go,
//     connection_authority_linux_test.go, connection_installed_linux_test.go,
//     connection_response_loss_linux_test.go; linux)
//   - installed document, execution and publication flows:
//     document_command_installed_linux_test.go,
//     execution_installed_linux_test.go,
//     publication_ack_installed_linux_test.go,
//     publication_receiving_network_linux_test.go; linux
//   - native journey, latency and worker replacement:
//     endpoint_journey_native_linux_test.go,
//     endpoint_latency_native_linux_test.go, latency_native_linux_test.go,
//     endpoint_replacement_worker_native_linux_test.go; native
//   - Linux installed-suite fixtures carry harnesses and TestMain, not
//     behavior of their own: network_admission_fixture_linux_test.go (the
//     admission-network base), route_fixture_linux_test.go,
//     route_issuer_fixture_linux_test.go, installed_route_fixture_linux_test.go
//     (budget and permission-stock base), installed_service_linux_test.go,
//     publication_installed_linux_test.go (the publisher harness) and
//     endpoint_launch_fixture_linux_test.go
package endpoint
