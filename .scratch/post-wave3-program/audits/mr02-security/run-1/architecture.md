# MR-02 security-audit architecture

## Scope and baseline

This run audits only MR-02 changes after fixed point `f1402d1`, not the whole
Ardents repository. Ardents is a Go 1.26 private distributed Node network.
Operators use `ardentsctl` and a Principal capability model over protected
Operator Unix sockets; Nodes use ConnectRPC, protobuf and Waku/libp2p.

MR-02 is a read-only workstation coordinator. Its closest baseline is Ansible
inventory plus strict OpenSSH host verification, but MR-02 is narrower: no
remote command/module is allowed. Its status UX resembles `kubectl get nodes`
or Nomad node status, but it deliberately opens one host-local path per Node
instead of trusting a central deployment controller.

A repository-wide audit from 2026-07-23 reported unrelated identity, content,
network, lifecycle, release and CI findings. The prior skill-format run has an
empty `findings.json`. Neither run covered this new MR-02 path; known unrelated
findings are excluded from this hunt.

## Execution and trust boundaries

1. `cmd/ardentsctl/main.go` enters `internal/cli/run.go`.
2. `internal/cli/topology/command.go` accepts only
   `topology status --manifest FILE`, requires a regular file and caps it at
   256 KiB.
3. `internal/deployment/status.go` applies the complete strict MR-01 manifest
   decoder/validator before any connection. It creates exactly three sorted
   protected targets and keeps protected identifiers out of the ordinary
   status model.
4. `internal/cli/configuration/config.go` resolves each manifest `ssh_alias`
   from a strict, regular, 1 MiB-bounded contexts file. It deliberately skips
   transport/signer environment overrides and requires SSH target, explicit
   known-hosts file, absolute remote socket, signer alias/file, pin reference,
   expected Node slot and Principal.
5. `internal/cli/topology/probe.go` compares context name, pin metadata, signer
   alias, Node and Principal before opening a client. It creates a separate
   signer/client/SSH tunnel/Session manager per Node.
6. `internal/cli/client/transport.go` invokes OpenSSH with argv, not a shell:
   `-F none -N -T`, batch mode, exit-on-forward-failure, no global known-hosts,
   strict explicit known-hosts, and stream-local `-L` to a protected Unix
   socket. It creates the local socket in a private temporary directory and
   runs no remote command.
7. `internal/cli/client/session.go` authenticates a typed Principal challenge
   bound to expected Node Principal, Operator interface, protocol major,
   Unix-local transport and peer binding. A protected call refreshes once only
   after `Unauthenticated`; other failures do not refresh.
8. Server authorization remains in
   `internal/localapi/identity/operator_interceptor.go` and the closed access
   catalog. The three calls require `node.runtime`,
   `transport.network_status`, and `node.features`.
9. `internal/cli/topology/probe.go` maps three protected responses to selected
   runtime, reachability, Store and image truth. Client reads are capped at
   4 MiB; essential nil submessages fail closed.
10. `internal/deployment/status.go` compares exact identity/image, ADR-0008
    composite readiness, mode-compatible reachability and separate Store
    pressure truth. It converts raw errors to closed codes.
11. `internal/cli/topology/command.go` emits a closed human/JSON model containing
    stable slot/role/state/reason values only.

## Actors and controls

- The Operator controls CLI arguments and reviewed manifest/context locations.
- The contexts registry, explicit known-hosts file and local signer bundle are
  trusted workstation state. Signer bundles already require private bounded
  regular files.
- SSH server identity is enforced by OpenSSH; the manifest pin reference is
  registry metadata bound by equality, not a cryptographic hash of
  `known_hosts`.
- A remote Node is authenticated by host key and exact Node Principal, then
  authorized by the existing capability model. It is still untrusted to return
  well-formed/bounded truth, so responses are capped and projected.
- A compromised local user able to rewrite the Operator's trusted context,
  known-hosts and signer files is outside the ordinary MR-02 attacker model;
  such filesystem integrity assumptions must remain explicit.

## Inputs and sinks

Inputs are CLI args, manifest path/JSON, context-file selection/JSON, trusted
SSH/signer file paths, OpenSSH environment/executable, SSH stderr, protected
ConnectRPC responses and caller cancellation/time. Dangerous sinks are
`os.Open`, context JSON allocation, signer reads, `exec.Command("ssh", argv)`,
Unix socket creation/dial, protobuf decoding, terminal/JSON output, process
kill/wait and temporary-directory removal.

Primary controls are strict schema/duplicate/secret-field rejection, exact
three-Node bounds, canonical Principal and immutable image validation, regular
and bounded files, argument validation, disabled external SSH configuration,
explicit strict known-hosts, private temporary state, per-Node 10-second and
overall 30-second contexts, 4 MiB response bounds, nil/length/NUL checks,
closed failure vocabularies and redacted output.

## Hunt priorities

- File/command injection, OpenSSH option/config escape, local temp/socket races,
  unbounded input/response/resource use and cleanup hangs.
- Context/pin/signer/Node binding bypass, cross-Node Session reuse, refresh
  amplification, weaker parallel API path, authorization mismatch and
  self-reported image/Store truth driving a false ready result.
- Identifier/secret/path leakage through errors, stderr, protobuf fields,
  JSON/human rendering and failure classification; timeout/cancellation and
  partial-result ordering; chains between trusted context mutation and remote
  impersonation.
