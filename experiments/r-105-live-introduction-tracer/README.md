# R-105 — Live Introduction control tracer

## Question

Can a finite live Publisher slot deliver exactly one HPKE-sealed C-2
introduction through a separate Introduction process, with no Service Target
visible to that process and no replayed JoinHandle delivery?

## Hypothesis and falsifiers

The Publisher first opens an outbound mutually-authenticated TLS attachment,
registers an opaque slot, and remains connected. The User separately opens an
Introduction attachment, submits a sealed record, and the Introduction process
forwards only the exact canonical bytes after consuming its JoinHandle. The
Publisher decrypts and compares the plaintext to its current synthetic
publication.

The hypothesis fails if a replay produces a second delivery, a modified
visible header/ciphertext decrypts, a stale/unknown slot delivers, the
Introduction output contains a Target or plaintext, or a process fails to join
its finite owned work.

## Scope and claim boundary

This is build-ignored disposable evidence for R-105. It uses deterministic
synthetic identities, a loopback endpoint, and no real Authority, Network
State, Service Name, browser, application data, or persistent credential. It
does not implement a Node duty or claim a complete User-to-Service route.
The follow-on cell must compose this proven control slice with separate
Initiator, Rendezvous, and Responder processes before retained C-2 runtime is
added.

## Run

From the repository root:

```powershell
pwsh -NoProfile -File experiments/r-105-live-introduction-tracer/run-local.ps1
```

The script builds outside the repository, starts a Publisher, Introduction,
and User process for each cell, checks their JSON evidence, and deletes only
its fresh exact temporary directory. Inputs and outputs are synthetic. It
records no packet capture, private key, or generated binary in Git.

## Predeclared matrix

| Cell | Required result |
|---|---|
| `exact` | One registered slot delivers one canonical sealed record; Publisher validates current publication. |
| `replay` | A second submission with the spent JoinHandle is refused before a second Publisher delivery. |
| `header-tamper` | Introduction forwards the raw record, but Publisher HPKE verification refuses the modified visible header. |
| `ciphertext-tamper` | Introduction forwards the raw record, but Publisher HPKE verification refuses the modified ciphertext. |
| `withdrawn-slot` | A User cannot deliver after the Publisher slot has closed. |

## Disposition

Retain only until a maintained Introduction duty and its qualification suite
supersede this evidence. Record successful and failed cells in R-105; do not
promote this implementation to a product API.
