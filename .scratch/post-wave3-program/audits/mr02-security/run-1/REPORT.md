# MR-02 Security Audit

## Executive summary

No exploitable vulnerability was confirmed in the MR-02 delta from
`f1402d1`. The command preserves the intended workstation/host-local trust
boundary: topology input is strictly admitted before connection, every Node
gets a separately bound SSH and Operator Session path, protected server
authorization remains in force, and ordinary output is closed and redacted.
The audit did identify resource and trust-closure hardening opportunities;
these were fixed during the task and re-tested.

## Baseline and scope

The closest operational baseline is strict OpenSSH host inventory tooling such
as Ansible, combined with bounded multi-node status UX such as Kubernetes or
Nomad. MR-02 is narrower than those remote-execution/control-plane designs:
it permits no remote command or helper and opens only stream-local forwarding
to each Node's protected Operator Unix socket.

This run covers the MR-02 implementation only. The repository-wide 2026-07-23
audit predates this delta and contains unrelated findings; the previous
skill-format run has an empty `findings.json`.

## Findings

No confirmed findings.

## Hardening completed during the audit

- Manifest and contexts inputs now require regular files and enforce 256 KiB
  and 1 MiB bounds before JSON decoding.
- OpenSSH now ignores external user/system configuration and the global
  known-hosts store; only the explicit reviewed known-hosts file participates.
- Operator client responses are capped at 4 MiB before protobuf projection.
- Nil Connect responses fail closed as `remote_invalid_response`.
- Waku Store pressure has a separate state from general abuse/rate-limit
  health, preventing unrelated defense activity from producing false Store
  degradation.

The remaining metadata fact is intentional: `host_key_pin_ref` names the
reviewed trust record, while OpenSSH verifies the actual key material.
Contexts, known-hosts and signer files are trusted Operator workstation state.

## Positive patterns

- No shell interpolation, remote command, remote helper or remote signer copy.
- Exact context, pin metadata, signer alias, Node slot and Principal checks
  occur before client creation.
- Per-Node client and process-local Session managers prevent cross-Node reuse.
- Authentication refresh is limited to one retry after `Unauthenticated`.
- Server-side capability admission remains mandatory for all three RPCs.
- Host-key stderr is bounded and converted to a stable redacted sentinel.
- Protected response identifiers and raw failures do not reach human or JSON
  output.
- Per-Node and whole-operation time bounds remain explicit.

## Coverage limits

The audit used source tracing, unit tests, race tests and local OpenSSH
configuration expansion. It did not run a live hostile SSH server or a real
three-host environment; those belong to later integration/security and MR-08
qualification gates. No real host or production state was touched.
