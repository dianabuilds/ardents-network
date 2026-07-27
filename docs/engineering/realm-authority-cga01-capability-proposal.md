# Capability proposal: Realm Authority CGA-01

This is a governance proposal only. It does not modify
`docs/engineering/capabilities.json` or its generated evidence register.

After the CGA-01 implementation commit receives independent
architecture/security review, governance may consider this exact change for
`realm.channel-grant-authority`:

- I: remain `partial` — genesis/inspect exist, but grant issuance, delivery,
  rotation, fencing, renewal, and recovery are absent.
- R: `no` -> `partial` — an authenticated Operator can execute the protected,
  exact-grant create/reopen/inspect journey.
- O: `no` -> `partial` — explicit provisioning, readiness, stable errors,
  diagnostics, bounded metrics, CLI output, and restart behavior exist for
  genesis only.
- Q: remain `no` — CGA-07 and matching-commit release evidence do not exist.

The proposed supported interface would be protected Operator
`AuthorityService` plus `ardentsctl authority create` and
`ardentsctl authority inspect`; implementation
owners would become `internal/authority`, `internal/localapi/authority`,
`internal/cli/authority`, and the daemon composition adapter. Operability must
remain explicitly limited to CGA-01 genesis/inspection until downstream slices
and qualification are complete.
