# CLI Principal Sessions

Status: operational contract for PIA-010B and PIA-010C.

## Enrollment

Create the offline root and routine device bundles first. For the one allowed
bootstrap enrollment, obtain the short-lived one-use Bootstrap Ticket through
the Node's protected provisioning procedure and store its canonical unpadded
base64url value in a private file. Never place it on argv:

```text
ardentsctl --addr unix:///var/lib/ardents/secrets/control.sock \
  --principal p1_<node> --signer-file DEVICE \
  identity enroll --root-signer-file ROOT --device-signer-file DEVICE \
  --bootstrap-ticket-file TICKET
```

The CLI obtains a typed enrollment challenge, validates its Node, Operator
interface, protocol, transport-peer binding, purpose, lifetime, and Principal,
then asks only the root signer to sign that typed structure. The Ticket and
enrollment proof are never printed and remain distinct from sessions. The first
enrollment consumes the Ticket atomically. Omit `--bootstrap-ticket-file` for a
later enrollment; that path requires the current Operator administrator session
and accepts an optional visible-ASCII `--request-id` idempotency key. After a
validated successful bootstrap response, the CLI deletes the consumed private
Ticket file. If deletion fails, enrollment remains committed and the CLI returns
an explicit cleanup failure; securely remove that same file before continuing.

For an Application installation, the Operator authorizes the prospective
Application Principal and exact initial actions through the protected Operator
session. The ticket is written only to a new protected file; it is never an argv
value or stdout/JSON field. The file contains canonical unpadded base64url with
no trailing whitespace and an existing output path is rejected before ticket
issuance:

```text
ardentsctl ... identity application-ticket issue \
  --principal p1_<application> \
  --action application.content.get \
  --action application.content.put \
  --out-file /protected/path/application-enrollment-ticket
```

This command is intentionally rejected by the daemon until PIA-014 supplies
owner-aware Principal Blob/content access. PIA-012 admission alone is not
sufficient because knowledge of a CID is not authorization to read. Do not
bypass the gate: successful
enrollment atomically disables the exact configured legacy
Application credential. The ticket expires after ten minutes, is one-use, and
is consumed only by `EnrollApplication` on the protected Application Unix
listener. It is not the existing `application-token`, and the SDK requires a
typed Application `EnrollmentSigner` while leaving root/device custody with the
embedding Application.

## Grants And Device Revocation

List grants for one exact enrolled Principal:

```text
ardentsctl ... identity grant list --subject p1_<alice>
```

Issue a Node-wide grant, or an exact resource grant, with repeatable registered
Operator actions:

```text
ardentsctl ... identity grant issue --subject p1_<alice> \
  --action node.status --action diagnostics.snapshot --valid-for 720h

ardentsctl ... identity grant issue --subject p1_<alice> \
  --action workload.status --scope exact \
  --resource-kind workload --resource-id workload_123
```

Revoke only after loading and displaying the exact active grant:

```text
ardentsctl ... identity grant revoke --subject p1_<alice> --grant-id ag1_<id>
ardentsctl ... identity device revoke --principal p1_<alice> --device-id d1_<id>
```

Human mutations display Principal, target Node, exact actions, scope and expiry,
then require typing exactly `yes`; `--yes` confirms an already reviewed human
command. JSON mode is deterministic and never prompts. Unknown actions, scope
kinds, identifiers, versions/fields, cross-Node exact resources, duplicate list
records and noncanonical time intervals fail closed. Administrative request IDs
are generated from cryptographic entropy unless `--request-id` is supplied.
Successful mutation output includes the request ID. An ambiguous
Unavailable/Internal/Unknown result is retried exactly once with the same
request and ID; denial, invalid input, conflict, capacity, cancellation and
deadline results are not retried. Reuse the displayed `--request-id` to
reconcile a mutation after an interrupted invocation.
Grant validity defaults to 30 days and cannot exceed 365 days. Operator grants
accept only `node` or `exact` scope; Principal-owned scope remains an Application
surface concept.

## Local Operator Login

Create and enroll the device Credential before using this flow. Then pin the
target Node Principal and address the permission-protected Operator socket:

```text
ardentsctl --addr unix:///var/lib/ardents/secrets/control.sock \
  --principal p1_<node> identity login

ardentsctl --addr unix:///var/lib/ardents/secrets/control.sock \
  --principal p1_<node> node status
```

The default signer is
`<os.UserConfigDir>/ardents/identity/device-v1.json`; place
`--signer-file PATH` before the command to override it. The CLI verifies and
signs the structured Operator challenge with the device key. It never loads the
root signer for routine authentication.

`identity login` verifies one invocation. Session reuse occurs only inside the
same live client, such as the interactive shell/TUI. `identity status` reports
only public cache-key facts, and `identity logout` clears and best-effort zeros
that process's entries. Session secrets, SessionIDs, signatures, Credentials,
and signer paths are not printed or persisted.

## Remote Operator Login

Principal sessions must terminate at the protected remote Unix socket:

```text
ardentsctl --ssh ops@alpha.example \
  --ssh-operator-socket /var/lib/ardents/secrets/control.sock \
  --principal p1_<alpha> identity login
```

`--ssh-port`, `--ssh-identity`, and `--ssh-known-hosts` retain their existing
OpenSSH meanings. The CLI starts one managed stream-local forward to a private
temporary local socket and waits for readiness. It does not use loopback TCP,
`ssh -W`, a remote shell, or a helper such as socat/netcat. Early exit,
cancellation, or an invalid socket path fails closed and cleans up local state.

## Refresh And Failure Semantics

The in-memory key is `(Node Principal, Operator interface, protocol major 1,
signer Principal)`. Alpha and Beta always have separate entries. Concurrent
callers share one Begin/Complete exchange. Expiry is half-open, with no session
clock skew.

On `Unauthenticated`, the client removes only the exact failed generation,
performs one new Begin/Complete exchange, and replays the RPC once. A second
`Unauthenticated` stops. `PermissionDenied`, invalid input, conflict, resource
exhaustion, cancellation, and transport errors do not trigger login or fallback.
For a server stream, replay is permitted only before its first event. Device
revocation therefore invalidates the live session and makes the single refresh
fail; the client never falls back to a bearer.

## Explicit Legacy Migration Mode

Legacy credentials are selected only with `--legacy-token`,
`--legacy-token-file`, `ARDENTS_LEGACY_API_TOKEN`,
`ARDENTS_LEGACY_TOKEN_FILE`, or explicitly legacy-named context fields. Every
source emits a value-free migration warning. Old ambient
`ARDENTS_API_TOKEN`/`ARDENTS_TOKEN_FILE` and context `token_*` fields are not
CLI authentication selectors and cannot downgrade a Principal context. Principal and
legacy credential selection cannot be combined. An effective HTTP target,
including loopback, never receives `ArdentsOperatorSession` and cannot issue a
Principal challenge through the CLI.
