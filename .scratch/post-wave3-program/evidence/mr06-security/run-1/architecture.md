# MR-06 security architecture — run 1

Target commit: `d05fa68`

Scope: MR-06 logical range `f167e8e..d05fa68`, concentrated on public-direct
manifest reconciliation, protected route/firewall/certificate preflight,
controlled restart, runtime configuration-generation binding, Waku
AutoNAT-gated publication, WSS validation and ordinary-output redaction.

## Entry points and flow

1. `deployment.PublicDirectCoordinator.Reconcile` strictly decodes and
   validates one exact `public_direct` manifest.
2. Deterministic protected targets bind the exact manifest, host slot/access
   references, one public address where applicable and WSS references.
3. Every public target completes fresh route/firewall/certificate preflight
   before the first host mutation. All observations are rechecked against one
   instant immediately before apply.
4. The WSS preflight supplies a canonical non-secret SHA-256 digest of the
   validated leaf/chain, public-key binding and CA-bundle generation.
5. Host apply is serial and returns one exact `installed`, `unchanged`, or
   `restarted` observation bound to current and prior configuration digests.
6. Protected final status binds the exact manifest and all three running
   per-slot configuration generations to MR-02 readiness, Store, immutable
   image and reachability truth.
7. Waku alone owns `Public`, `Private`, and `Unknown`. Only fresh libp2p
   `Public` makes the admitted endpoint publishable; stop, restart, `Private`,
   `Unknown`, or event-stream loss withdraws it.

## Trust boundaries

- Protected local Operator input -> strict topology decoder/validator.
- Coordinator -> consumer-owned preflight, host and status adapters.
- Host filesystem/OS trust roots -> WSS certificate/key/CA validation.
- Untrusted network peers -> go-libp2p AutoNAT -> local reachability event
  stream.
- Runtime endpoint gate -> signed local discovery publication.

The R0 slice contains no production SSH, firewall/router, PKI, DNS or host
adapter. Those adapters must authenticate their observations and preserve the
exact protected bindings when later composed.

## Protected and ordinary data

Protected data includes the raw manifest, addresses, DNS identities,
certificate references and material-generation digests, SSH aliases and pin
references, host plans, configuration digests and adapter observations.
Ordinary `PublicDirectResult` contains only bounded counts, a closed outcome
and a stable reason code.

## Dangerous sinks

- `Service.Endpoints` feeds daemon reachability publication.
- `publication.RefreshNetworkPublicationLocked` signs the local discovery
  record.
- `WithSecureWebsockets` loads the configured WSS keypair into the new Waku
  process.

## Prior audit coverage

The MR-05 audit covered private-LAN proof admission and withdrawal. This run
targeted the distinct public AutoNAT, WSS material lifecycle, configuration
generation, restart and final-status paths.
