# Closed-alpha corpus cohort notice template

Status: **template only; filling it with real values is an explicit Product
Owner publication action.**

Use one immutable notice per Alpha Name Corpus serial. Deliver its location to
the invited participant through the same authenticated contact class as their
Alpha Enrollment Pin. The notice tells the participant where to obtain the two
explicit bytes; it is not a name authority, a browser setup instruction, a
registrar, or an update service.

```text
Ardents closed-alpha corpus notice v1

Cohort: <lowercase cohort identifier>
Network: <lowercase hexadecimal 32-byte Network ID>
Corpus serial: <positive decimal integer>
Valid from: <RFC3339 UTC timestamp>
Valid until: <RFC3339 UTC timestamp>
Previous accepted serial: <none for first intake | positive decimal integer>
State: <active | withdrawn>

Catalog file: catalog.ac2
Catalog location: <immutable HTTPS/GitHub-release/removable-media location>
Catalog SHA-256: <lowercase hexadecimal digest>

Corpus file: corpus.anc
Corpus location: <immutable HTTPS/GitHub-release/removable-media location>
Corpus SHA-256: <lowercase hexadecimal digest>

Replacement instruction:
  Obtain exactly the listed pair and repeat the documented
  `ardents-control accept-alpha-corpus` procedure with the same local floors.

Failure instruction:
  Do not use a Target Link, another corpus source, DNS, or a browser setting as
  a fallback. Report an unavailable, changed, expired, lower-serial, or
  same-serial-different pair through the authenticated alpha contact class.
```

## Verification boundary

The listed SHA-256 values help a human recognize download damage or a changed
location. They do **not** authorize a name. Before retaining either byte,
`ardents-control accept-alpha-corpus` verifies the exact enrolled artifact and
control command, accepted ACA1 evidence, independently pinned corpus root,
ACA2 binding, corpus signature, cohort, Network, validity, serial, and durable
floor. A changed notice, hosting account, URL, checksum, or contact message
cannot make a new name binding valid.

Do not publish a notice until all values refer to real immutable files and the
matching enrollment-v3-or-later bundle manifests the exact `ardents-control` companion.
Publishing a later serial requires a new notice; editing an earlier notice is
not a replacement mechanism. A total withdrawal uses `State: withdrawn` and a
signed withdrawn `corpus.anc`; it does not direct participants to delete their
floor or use an alternate destination.

See [Closed-alpha Alpha Name Corpus intake](closed-alpha-name-corpus.md) for
the participant procedure and [R-113](../research/records/r-113-alpha-corpus-distribution-floor.md)
for the authority boundary.
