# Independent offline permission inspection

This finite command
inspects a document; it does not accept live Network State, establish Time
Confidence, authenticate a holder, issue tokens or debit allocation.

The verifier under internal/successor/admission independently implements the
canonical 228-byte permission grammar in private-admission.md. The existing
wire transcript and domain separator are preserved; no legacy package is
imported. The signing interval must be one aligned UTC hour, contained within
the independently selected duty validity. Unix timestamps exceeding signed
64-bit range or overflowing the one-hour end are refused before conversion.

## Review of retained behavior

The implementation was compared with internal/admission/permission.go and its
canonical verification test. It retains the byte grammar, Ed25519 domain,
nonzero identities and duty, hour alignment, nonempty maxima bounded at 65536,
network/issuer/duty binding and the exclusive expiry boundary. It adds explicit
offline holder matching, duty interval containment and requested class/count
checks; these do not establish holder possession or cumulative capacity.
Unsigned timestamps are checked before signed conversion. A canonical hour
beginning at Unix epoch remains valid offline: zero Unix seconds are not a zero
identity or Go's zero time. Signed regression fixtures cover that boundary.
There is no new wire migration.

Source isolation allows reviewed implementation reuse from the maintained dev
code. It prohibits imports of old product packages, not reuse of correct code.
Tests establish selected invariants rather than taking old output as an oracle.

## Command and input

`ardents-next inspect-permission <permission-file> <facts-file> [collector]` accepts one
regular permission file of at most 228 bytes and one regular JSON file of at
most 4096 bytes. Facts use exactly ten string-valued fields: network, issuer,
authority, holder (64 hex characters each), duty, class, count (canonical decimal
unsigned integers), duty_not_before, duty_not_after and now (RFC3339 timestamps).
Duplicate, missing, unknown and trailing fields refuse. Files are read once;
there is no claim of authenticated filesystem custody or owner isolation.

The owner must select facts independently from the document. The facts file is
an explicit offline assertion, not a signature-verified State projection. In
particular holder comparison does not prove possession of its private key.
No profile digest is invented in the permission grammar. Real live profile/time
and holder admission will require a separate production contract.

Output contains only a finite outcome. Exit 0 means accepted-offline, 1 means
document refused, 2 means input/configuration/output unavailable, and 130 means
observed cancellation. Acceptance establishes one request is within the signed
maximum; repeated inspections do not establish cumulative allocation capacity.

## Diagnostics and telemetry

Standard output contains the JSON outcome; standard error contains a structured
operation event, outcome and finite telemetry status. Neither output contains
input bytes, identities, file paths or raw exporter errors.

OpenTelemetry records one root span, an outcome counter and duration histogram.
The only resource attribute is service.name; the only operation attribute is
outcome. No incoming trace parent, participant identifiers or exemplars are
recorded. Without a Collector argument, observations remain local to this finite
process and are discarded at exit.

An explicit HTTP Collector must use a numeric loopback address and a port in
1..65535, with no credentials, query, fragment or custom path. OTLP protobuf
exports use /v1/traces and /v1/metrics. Ambient headers, proxy routing, redirects,
compression and retries are disabled. Requests are capped at 32768 bytes;
response bodies and headers are capped at 4096 bytes each. Each HTTP operation
has a 250 ms timeout and telemetry shutdown has a 750 ms total deadline.
Collector failure reports telemetry unavailable without changing the inspection
outcome. These bounds do not claim anonymity against local process observation
or an independently operated Collector.
