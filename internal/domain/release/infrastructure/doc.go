// Package infrastructure encodes the signed artifact descriptors for fixed
// infrastructure invocations. Its Linux admission consumes the original
// Enrollment snapshot and fresh coherent Release proofs for a program and
// descriptor. The deployment caller owns process and service configuration.
//
// Responsibility clusters and their tests:
//   - public descriptor grammar and role-separated bytes: artifact.go,
//     artifact_identity.go binds the exact fixed-role descriptor, unit and target names
//     (artifact_test.go, descriptor_artifact_test.go,
//     forwarding_artifact_test.go, join_artifact_test.go)
//   - candidate authorization, outer guards and retained release history:
//     admission.go, descriptor_admission.go, forwarding_admission.go,
//     join_admission.go (admission_linux_test.go, source_release_linux_test.go,
//     forwarding_artifact_linux_test.go; linux)
//   - authorization custody to handoff: admission.go
//     (authorization_lifetime_linux_test.go; linux)
//   - runtime release admission with retained floors and original proof:
//     runtime_artifact_linux.go (platform_release_linux_test.go; linux)
//   - runtime artifact preparation and inventory custody: preparation_linux.go,
//     preparation_command_linux.go (infrastructure_inventory_refusal_linux_test.go;
//     linux, preparation_native_linux_test.go; linux gated by the
//     network_signing_native profile)
//   - source release inspection history independence:
//     source_release_inspection_linux.go (TestSourceReleaseHistoryIsIndependent
//     in admission_linux_test.go; the remaining inspection entry points are
//     command-adapter consumers without a package-local unit test)
package infrastructure
