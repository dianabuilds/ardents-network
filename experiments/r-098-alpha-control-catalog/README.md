# R-098 — Alpha-control disclosure catalog experiment

## Question

Can a small signed catalog give one inspectable alpha-cohort view while each
release and network component remains subject to its own signing key, digest,
expiry, and floor check?

## Hypothesis

The read-only synthetic reader accepts a valid catalog with independently
signed `release` and `network-profile` components. It reports failures per
component for changed bytes, expiry, and lower floor; it reports withheld and
conflicting catalogs before attempting to accept components. The catalog's own
key cannot make a changed component acceptable.

The follow-up fixture also writes a short-lived private directory containing
three bounded regular files: `catalog.json`, `release.json`, and
`network-profile.json`. Each signed envelope contains base64-encoded body bytes
and a detached Ed25519 signature over those exact bytes; it does not depend on
JSON re-serialization for signature verification. The catalog binds the digest
of each complete component envelope and the reader derives component filenames
only from the two fixed classes. It rejects an oversized catalog before JSON
parsing.

## Run

```sh
go run main.go file_reader.go

# Runs the five file-reader tests without placing this experiment in `go test ./...`.
go test main.go file_reader.go file_reader_test.go
```

The program creates fresh Ed25519 keys and synthetic component bytes in memory.
Its bounded-file subfixture writes only to a newly created OS temporary
directory, removes that exact directory on exit, uses no Ardents Authority, and
starts no Endpoint or network listener.

## Evidence

Capture the Go version and all output in R-098. Passing shows only that the
proposed separation is mechanically expressible in a small synthetic reader.
It does not solve first-artifact bootstrap, choose actual alpha roots, define
Network State or Namespace materialization, or authorize a maintained format.
The fixture's parser rejects duplicate object keys at every JSON nesting level;
the selected maintained format still needs its own independently reviewed parser
and resource policy.

## Result

On 2026-08-24 the in-memory matrix accepted the exact release and network
components and separately rejected changed bytes, expiry, lower component and
catalog floors, conflicting or unavailable catalogs, and an unknown required
component. The bounded-file follow-up accepted three exact signed files,
rejected a 4,097-byte catalog before parsing, and its five tests also covered
changed component bytes plus direct and signature-valid duplicate-key JSON.
This supports authority separation and bounded parsing only; it selects no real
root, component identity, wire format, or maintained reader.

## Disposition

Disposable research code for open implementation-linked R-098. Delete it after
the selected H4-6A format/reader supersedes its unique evidence or the candidate
is rejected.
