# Multi-host Status Contract

MR-02 adds one bounded workstation-side inspection command:

```text
ardentsctl topology status --manifest FILE
```

The command first applies the complete
[`ardents.topology/v1`](multi-host-topology-contract.md) admission contract.
Invalid input opens no connection. Accepted input always describes exactly
three Nodes and produces exactly three rows sorted by stable Node slot.

## Protected context binding

Each manifest `host.ssh_alias` selects a named entry in the Operator contexts
file. The entry must contain:

- an SSH target and explicit `ssh_known_hosts` file;
- an absolute `ssh_operator_socket`;
- `host_key_pin_ref` exactly equal to the manifest host pin reference;
- a local `signer_file` and `signer_alias` exactly equal to the manifest
  `operator_signer_alias`;
- `expected_node` and `expected_principal` exactly equal to the manifest Node.

Transport, signer and identity values are read from the named entry without
environment transport overrides. The pin reference is metadata that binds the
reviewed manifest to the reviewed known-hosts record; OpenSSH performs the
actual strict host-key check. MR-02 disables external OpenSSH configuration and
the global known-hosts store so the reviewed target and explicit known-hosts
file are the only SSH routing and host-key inputs.

The command creates and closes a separate SSH client and process-local Operator
Session manager for every Node. SSH uses `-N -T` stream-local forwarding to the
protected remote Unix socket. It runs no remote command or shell, installs no
helper, and never copies signer or Session material to a host.

## Observed truth

For each Node, the adapter makes at most these three protected
product-observation calls:

1. `NodeService.GetNodeRuntime`;
2. `NetworkService.GetNetworkStatus`;
3. `NodeService.GetNodeFeatures`.

After the observation, the adapter makes one bounded
`IdentityService.EndSession` session-lifecycle call. That cleanup is not a
product observation and does not extend either the per-Node or aggregate
deadline.

The aggregate checks exact Node name and Principal, ADR-0008 composite
readiness, topology-compatible joined/reachability state, declared persistent
Store availability, and the configured immutable image reference. Image output
is limited to `match`, `mismatch`, or `unverified`; image names and digests are
never returned. An absent runtime image reference is truthfully degraded as
`unverified`.

Each Node is bounded to 10 seconds and the complete operation to 30 seconds.
Existing Operator authentication may refresh once after `Unauthenticated`.
Permission denial, unavailable Node, pin/tunnel failure and timeout do not add
another retry.

## Output and exit contract

The topology outcome is:

- `ready` when all three complete observations match;
- `degraded` when all Nodes are observed but any observed truth is negative;
- `partial` when any Node cannot be observed.

Ordinary output contains only stable slot, role, observation, readiness,
joined/reachability, Store and image states, plus one stable reason code. It
never contains SSH targets, paths, socket locations, host pins, Principal or
Waku identifiers, signer/Session details, addresses, images or digests.

`ready` exits zero. `degraded` and `partial` still return the complete bounded
aggregate on stdout and exit non-zero. An invalid manifest produces no
aggregate. A missing or mismatched per-Node local context is a redacted
inaccessible observation, so the command still returns all three rows with a
`partial` outcome.

The closed inaccessible-node reasons are `host_key_mismatch`,
`tunnel_timeout`, `tunnel_failure`, `local_signer_unavailable`,
`remote_unauthenticated`, `remote_denied`, `node_unavailable`, and
`remote_invalid_response`.

## Support boundary

MR-02 is read-only local inspection. It does not install, start, repair,
roll out, fence or rejoin Nodes; probe DNS/WAN reachability; mutate Authority
state; or qualify real hosts. `deployment.multi-host` remains `Q=no` until the
separate matching-commit MR-08 qualification gate succeeds.
