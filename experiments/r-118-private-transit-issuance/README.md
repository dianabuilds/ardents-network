# R-118 — Private Transit Grant issuance data-flow experiment

## Question

Can an Endpoint create a fresh attachment and TLS key, request one exact
State-bound Introduction Grant without putting a Service Name or Target in the
issuer request, and then have a Node reject substitution or replay?

This is a disposable data-flow experiment for
[R-118](../../docs/research/records/r-118-participant-transit-credential-lifecycle.md).
It does not select a live issuer transport or add a product API.

## Hypothesis and falsifiers

The request grammar contains only Network ID, State digest/Epoch, selected
Introduction Node/role, attachment, TLS public-key digest, and expiry. The
issuer signs exactly those fields. A Node accepts only that signature with the
matching local client key and spends its Grant ID once.

The hypothesis fails if the raw issuer request contains the synthetic Target
or `reference.ard`, accepts a trailing Target field or a changed Node, accepts
a replacement TLS key, or admits the same Grant twice.

## Scope and limits

All roles run in one local process with synthetic keys. The experiment models
the minimum disclosure transcript and exact tuple checks only. It does **not**
prove OHTTP/Entry transport privacy, issuer admission, rate limits, durable
Node replay persistence, availability, State distribution, Endpoint key-store
durability, or a participant command. No generated key, capture, cache, or
binary is stored in the repository.

## Run

From the repository root:

```powershell
go run experiments/r-118-private-transit-issuance/main.go -case exact
go run experiments/r-118-private-transit-issuance/main.go -case target
go run experiments/r-118-private-transit-issuance/main.go -case node-substitution
go run experiments/r-118-private-transit-issuance/main.go -case expiry-substitution
go run experiments/r-118-private-transit-issuance/main.go -case wrong-key
go run experiments/r-118-private-transit-issuance/main.go -case replay
```

Each invocation prints one non-secret JSON result. A zero exit status means
the positive cell or its required refusal behaved as declared.

## Predeclared matrix

| Cell | Required result |
| --- | --- |
| `exact` | Issuer sees no synthetic Name/Target and Node admits the matching fresh key once. |
| `target` | The strict request decoder refuses a trailing Target. |
| `node-substitution` | Issuer refuses a Node not present in the supplied State-duty fact. |
| `expiry-substitution` | Issuer refuses an expiry longer than the supplied State-duty fact. |
| `wrong-key` | Node refuses a certificate whose public-key digest differs from the Grant. |
| `replay` | Node refuses a second spend of the same Grant ID. |

## Result and disposition

All six cells passed locally on 2026-08-26. The positive record reported no
synthetic Target or Service Name in the issuer request; every negative cell
reported its declared refusal. Retain only while R-118 compares this
transcript against a real Entry/OHTTP issuer transport. Delete or replace it
once a selected maintained H4-2 issuance boundary supersedes the experiment.
