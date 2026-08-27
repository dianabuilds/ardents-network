# R-118 — credential-relay transport experiment

## Question

Can an Endpoint obtain one fresh, exact Transit Grant through separate local
Endpoint, Initiator, and issuer processes such that the issuer receives an
OHTTP plaintext containing no Service Name, Target, Descriptor, Invite, or
Endpoint network address, while the Initiator consumes one local admission
budget and cannot alter the requested transit tuple?

This is the second disposable experiment for
[R-118](../../docs/research/records/r-118-participant-transit-credential-lifecycle.md).
It evaluates the proposed Credential Relay shape only. It neither selects the
operation nor changes the maintained State, Entry, Route, or Endpoint APIs.

## Topology and scope

The parent process starts three short-lived child processes:

```text
Endpoint -- one local admission proof + opaque OHTTP --> Initiator -- OHTTP --> issuer
```

The Initiator accepts one synthetic, finite Entry-equivalent admission proof,
records it locally, strips that proof, and forwards the opaque OHTTP envelope
to the issuer. The issuer has a synthetic State-selected identity and a
Transit Grant signing key. It decapsulates a fixed-capacity request, verifies
the exact State-bound Introduction tuple, and returns a signed Grant in a
fixed-capacity OHTTP response.

The admission proof models the effect of a completed Entry TLS binding; it is
not a replacement for the maintained Entry TLS protocol. The experiment also
does not model State distribution, a real State `transit-issuance` duty,
Initiator compromise or collusion, multi-Initiator global quotas, durable
production storage, transport anonymity, Endpoint installation, or service
opening. No key, capture, or binary is retained in the repository.

## Hypothesis and falsifiers

The accepted Endpoint plaintext is exactly Network ID, State digest/Epoch,
Introduction Node/role, fresh attachment, fresh client-key digest, and a
bounded expiry. The issuer is reachable only through the Initiator and sees
none of the local admission proof or endpoint peer address. The Initiator
permits one request for one admission proof and forwards no caller-selected
destination.

The hypothesis fails if:

- the issuer sees the synthetic Service Name, Target, Descriptor, or admission
  proof;
- a trailing Target is accepted by the exact request grammar;
- an already-consumed admission proof obtains a second result;
- a changed Introduction Node, expiry, or client-key binding produces a Grant;
- the issuer sees the Endpoint's connection address instead of the Initiator's
  adjacent connection; or
- the received Grant does not verify under the State authority and exactly
  match the local attachment/key tuple.

## Predeclared matrix

| Cell | Required result |
| --- | --- |
| `exact` | A fresh one-use admission is consumed; the Endpoint verifies an exact signed Grant; the issuer's plaintext and adjacent peer satisfy the declared disclosure boundary. |
| `target` | The issuer's strict grammar rejects an appended synthetic Target. |
| `replay-admission` | The Initiator refuses the second use of the same admission proof before forwarding it. |
| `node-substitution` | The issuer refuses a non-selected Introduction Node. |
| `expiry-substitution` | The issuer refuses an expiry outside its selected State window. |
| `wrong-key` | The Endpoint refuses a Grant whose key digest does not match its retained fresh key. |

## Run

From the repository root:

```powershell
go run experiments/r-118-credential-relay/main.go experiments/r-118-credential-relay/roles.go -case exact
go run experiments/r-118-credential-relay/main.go experiments/r-118-credential-relay/roles.go -case target
go run experiments/r-118-credential-relay/main.go experiments/r-118-credential-relay/roles.go -case replay-admission
go run experiments/r-118-credential-relay/main.go experiments/r-118-credential-relay/roles.go -case node-substitution
go run experiments/r-118-credential-relay/main.go experiments/r-118-credential-relay/roles.go -case expiry-substitution
go run experiments/r-118-credential-relay/main.go experiments/r-118-credential-relay/roles.go -case wrong-key
```

Each command emits one non-secret JSON result. A zero exit code means its
positive outcome or predeclared refusal occurred.

## Result and disposition

All six cells passed locally on 2026-08-26. The exact cell used separate
Endpoint, Initiator, and issuer child processes: the issuer observed the
Initiator's adjacent connection and no forwarded admission proof, while its
decapsulated request contained none of the synthetic Name, Target, or
Descriptor markers. The replay cell forwarded only the first request; the
Initiator refused the second before it reached the issuer. The malformed
Target, Node, and expiry cells were rejected by the issuer; the Endpoint
refused the issuer's deliberately valid-but-wrong-key Grant.

The Target cell intentionally delivers an invalid trailing Target to the
issuer so that its exact parser can reject it. That cell is not a disclosure
claim; the exact cell is the only successful-transcript disclosure evidence.

This proves only the narrow three-process grammar and disclosure experiment.
It does not authorize a maintained Credential Relay, claim that an issuer
cannot collude with an Initiator, establish real Entry TLS admission or a
durable quota, or close R-118.
