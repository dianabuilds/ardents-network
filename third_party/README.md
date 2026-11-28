# Dependency source

`quic-go` is not project code. It is the published module
`github.com/quic-go/quic-go v0.66.0` with one retained patch, kept inside the
repository by [ADR-0136](../docs/adr/0136-return-quic-from-patched-official-source.md)
until an official release carries the correction. The root `go.mod` replaces the
module with this directory, so the module path and version stay official.

- The directory holds the module's root Go files, `go.mod`, `go.sum`, `LICENSE`
  and the packages `internal`, `qlog`, `qlogwriter`, `quicvarint` and
  `testutils`, byte for byte, with
  `experiments/r-172-quic-return-path/quic-go-v0.66.0-probe-budget.patch`
  applied. Nothing else of the module is kept.
- `internal/testdata` holds the module's public localhost test certificate and
  key, which its own tests load. They are published in the module archive.
- Do not edit anything in it. `TestQuicGoSourceIsPublishedModuleWithOnePatch`
  in `internal/architecture` rebuilds the directory from the published archive
  and fails on any other byte. `make quic-go-source-test` runs the module's own
  tests inside it.
- A new upstream release replaces the directory as a whole; the patch is then
  applied again with its exact context. The directory leaves with the first
  official release that carries the correction.

The [dependency register](../docs/development/dependencies.md) owns the
identity, exposure, update and removal record.
