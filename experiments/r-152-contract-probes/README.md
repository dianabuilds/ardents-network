# R-152 contract feasibility probes

Question: can the selected closed text-Service contract compose maintained
blind RSA, finite authority and Ubuntu confinement without a format/process
contradiction? These disposable build-ignore Go programs are evidence, not
maintained product modules. Generated files and caches stay outside Git.

## Predeclared checks

Before the relevant run:
- use reviewed RFC 9474 primitives, independent RSA-PSS verification and
  negative message/signature/receiver cases;
- distinguish RFC 9578 RSA-PSS SPKI from generic RSA serialization;
- deny forbidden network, host IPC/files, namespace/privilege and child effects,
  with working unconfined positive controls and a permitted local byte stream;
- an unavailable systemd/kernel/property is an invalid environment, never a skip;
- retain every causal network exchange in the model and reject a required
  reference cell exceeding 3 s cold or 1 s warm under its declared assumptions.

## Reproduction

From the root Go module with its selected toolchain/dependencies already
provisioned, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly and CGO_ENABLED=0:

~~~sh
go mod verify
go test github.com/cloudflare/circl/blindsign/blindrsa -count=1
go run experiments/r-152-contract-probes/token_composition.go
go run experiments/r-152-contract-probes/cost_model.go
~~~

Capture stdout/stderr and exit codes in a fresh external evidence directory.
The token probe generates ephemeral synthetic keys, performs 64 round trips
and 192 negative checks. The cost model is deterministic arithmetic over its
listed assumptions, not an end-to-end performance test.

For confinement, on the declared Ubuntu host with systemd as system manager,
cgroup v2 and the required namespace/seccomp features:

~~~sh
CGO_ENABLED=0 go build -o /absolute/external/evidence/confinement-probe experiments/r-152-contract-probes/confinement_probe.go
sudo sh experiments/r-152-contract-probes/run_confinement.sh /absolute/external/evidence/confinement-probe /absolute/external/evidence
~~~

The evidence directory must exist and must not already contain baseline.json
or sandbox.json. Inspect both receipts and the complete parent/child positive
and denial sets; a process exit alone is not the verdict. The script retains
its generated root and failures for inspection. It installs no tools or units
persistently. System-manager privileges are a prerequisite of this disposable
probe, not authority to create an arbitrary privileged product launcher.

Record the full Go/module/tool/database and Ubuntu package inventory and
source/output hashes for a repeated candidate. Follow make tools-install for
tool installation and the dependency owner's current acceptance procedure.
The repository's actual candidate gates are not replaced by these probes.

## Results and disposition

The [research assessment](../../docs/research/records/r-152-closed-scheme-contract.md)
contains exact original environment, hashes, failures and limitations.

- Go 1.26.6: 64 token round trips and 192 negative checks passed; selected
  RSA-PSS SPKI 346 bytes, generic RSA SPKI 294, token 354, request 259.
  Local p95 3,118 microseconds is not network timing.
- WSL Ubuntu 24.04.4/systemd 255: fourteen parent/child denial checks and
  positive controls passed, with the permitted local byte marker retained.
  The earlier working-directory and positive-control failures are preserved.
  This is mechanism evidence, not installed-Application or bare-host qualification.
- The later portable-wrapper syntax check passed in Git Bash; WSL access for
  that syntax-only retry was denied. No new confinement pass is claimed.
- The first model omitted Terminal receipt/confirmation and is retained as
  invalid complete-schedule evidence. The corrected model includes them,
  concurrent JOIN/capsule delivery, finite actual-work Carrier reuse and the
  explicitly coalesced generation-3 Instance/Continuity authentication flight.
  It gives TCP cold/warm 2,248.30/930.49 ms and QUIC
  2,128.30/930.49 ms at 20 ms/link under the same processing/transfer assumptions. Larger RTT cells
  fail and remain in its output. This is neither a measured p95 nor an anonymity test.
- The historical scanner output is not a fresh final-candidate admission;
  its empty database receipt is not accepted evidence. The assessment states
  the exact OpenPGP non-applicability scope and invalidation conditions.

Retain all three probes for reproducing decisions. Full token wire vectors,
real admission ledgers, installed IPC/grants, network timing, hostile behavior
and migration are implementation acceptance in P1–P11.
