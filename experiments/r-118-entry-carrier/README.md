# R-118 — real Entry carrier prototype

## Question

Can the current public Entry and Route APIs establish a real State-pinned,
mutually authenticated Entry attachment and then carry one opaque post-admission
blob, without exposing the Entry Invite to that post-admission handler?

This is a throwaway logic prototype for R-118. It is deliberately smaller than
the separate-process Credential Relay experiment: it proves only whether the
existing Entry TLS boundary can replace that experiment's synthetic local
admission. It does not define `CredentialRelaySetup`, select an issuer, or add
a maintained API.

## Preconditions and falsifiers

One process constructs an authenticated current Entry view, issues/imports one
Invite, opens `route.OpenEntryAttachment`, and accepts it with
`route.AcceptEntryAttachment` plus the durable `entry.Admitter`. After
acceptance, the prototype writes one fixed opaque blob.

The hypothesis fails if the Invite is not accepted by both Entry owners, if
the Route attachment cannot complete mutual TLS and durable admission, if the
post-admission handler receives the Invite bytes, or if it cannot receive the
opaque blob after successful acceptance.

## Run

From the repository root:

```powershell
go run experiments/r-118-entry-carrier/main.go
```

The command prints the terminal admission and carrier state as non-secret JSON
and removes its temporary Entry roots. It stores no keys, captures, or
database in the repository.

## Result and disposition

Passed locally on 2026-08-26. The prototype printed `accepted` for the User
Invite, completed the durable real Entry attachment, delivered the opaque blob,
and reported no Invite marker in the post-admission input.

The answer is retained in R-118. This prototype still does not select a
Credential Relay, prove OHTTP forwarding, issue a Transit Grant, or establish
an issuance budget.
