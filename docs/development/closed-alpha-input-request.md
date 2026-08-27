# Closed-alpha static-input request

Status: **maintained ADR-0052 request contract. No synthetic value or example
below is release material.**

For H4-alpha-1, the real `network_state` object is copied from the public
`alpha-network-state.json` emitted by the separate ADR-0053 State-custody
operation. Its `schema`, `topology`, validity, and encrypted-envelope digest
are provenance fields outside the nested request object; the eight fields
accepted below are copied exactly. The current fragment declares an empty
persistent candidate view and therefore carries empty `inputs` and
`materials` arrays.

`ardents-release-custody assemble` consumes one strict JSON request outside the
repository. The request is public and contains no password, private key,
Authority material, source URL, upload target, shell command, signer choice, or
arbitrary message. Its exact bytes are committed by the public assembly
receipt.

## Invocation

```powershell
ardents-release-custody assemble `
  --root C:\Users\vitek\Ardents-Release\keys `
  --request C:\absolute\external\alpha-input-request.json `
  --endpoint C:\absolute\external\ardents-linux-amd64 `
  --control C:\absolute\external\ardents-control-linux-amd64 `
  --output C:\absolute\external\prepared-static
```

Every path is absolute. The encrypted record and three inputs are bounded
direct regular files or directories; the output parent already exists and the
output itself is absent. The password is requested only through the trusted
local adapter. It never belongs in the request, command line, environment,
repository, log, receipt, bundle, or chat.

## Request shape

The complete v1 object has exactly these fields. JSON `[]byte` values below use
standard base64 encoding. Times are UTC RFC 3339 values at whole-second
precision.

```json
{
  "schema": "ardents-alpha-input-request-v1",
  "profile": "ardents-h4-alpha-1-v1",
  "cohort": "DECLARED_COHORT",
  "release": "DECLARED_RELEASE",
  "release_version": 1,
  "reference_time": "YYYY-MM-DDTHH:MM:SSZ",
  "not_before": "YYYY-MM-DDTHH:MM:SSZ",
  "not_after": "YYYY-MM-DDTHH:MM:SSZ",
  "build_safety_no_new_work_after": "YYYY-MM-DDTHH:MM:SSZ",
  "build_safety_terminate_after": "YYYY-MM-DDTHH:MM:SSZ",
  "environment": "alpha",
  "network": "DECLARED_NETWORK",
  "source_revision": "70bf425eec937edcc22e8f0534db992aa2002a16",
  "endpoint_sha256": "33473599f7902508d1ca9cb9d09eb6777aff05d9c7c652e96f841b196bfd1fe1",
  "control_sha256": "d69b4c5d5f6fae76cbeacfb6acee8abaec9b6cbb56afd339982ea6d55ef9449c",
  "build_input_commitment": "DECLARED_BUILD_INPUT_IDENTITY",
  "build_identity": "DECLARED_BUILD_IDENTITY",
  "dependency_identity": "DECLARED_DEPENDENCY_IDENTITY",
  "sbom_identity": "DECLARED_SBOM_IDENTITY",
  "qualification": "qualified",
  "build_state": "current",
  "protocol_phase": "announced",
  "capacity_ready": false,
  "drain_ready": false,
  "builders": ["DECLARED_BUILDER_A", "DECLARED_BUILDER_B"],
  "network_state": {
    "network_id": "64-lowercase-hex-characters",
    "epoch_digest": "64-lowercase-hex-characters",
    "profile": "DECLARED_ACCEPTED_PROFILE",
    "threshold": 1,
    "authorities": ["64-lowercase-hex-ed25519-public-key"],
    "epoch": "BASE64_CANONICAL_EPOCH",
    "inputs": ["BASE64_CANONICAL_INPUT"],
    "materials": ["BASE64_CANONICAL_MATERIALIZATION"]
  }
}
```

`protocol_overlapped_since`, `emergency_reason`, and `emergency_expiry` are
permitted only when the maintained Release state machine accepts that exact
transition. The ordinary first-alpha `announced` phase carries none of them.
Unknown fields, a caller-selected role/key/path, changed artifact bytes,
duplicate Network authorities, invalid time bounds, unsupported lifecycle
values, or an unaccepted Network State fail before output publication.

The public operation is not a reusable alpha signer. It accepts only the
recorded `ardents-h4-alpha-1-v1` profile, source revision, Endpoint digest,
control digest, and selected encrypted-envelope digest from the owning release
profile. A different self-consistent artifact, control companion, custody
record, or source revision fails before the passphrase is requested. The
recorded `reference_time` keeps verifier evidence reproducible; the adapter's
actual invocation time separately must still precede both `not_after` and
`build_safety_no_new_work_after` (and an emergency expiry when present). That
freshness is checked both before secret use and immediately before the atomic
output rename.

The two builder names disclose two distinct project-controlled build
observations. They do not claim independent builders. Their target digest and
shared source/build-input facts are derived into both TUF attestations from the
request and exact Endpoint bytes; they cannot disagree silently.

## Fixed output and preflight

The operation constructs exactly the fourteen files named by
[the alpha-bundle assembler](../../packaging/alpha-bundle/README.md): initial
TUF Root/Targets/Snapshot/Timestamp, `RELEASE`, ACA1 catalog, three ACS1
components, four corresponding public roots, and `corpus.pub`.

Before the directory becomes visible at `--output`, the operation:

1. evaluates the exact Endpoint and TUF bytes through Release Decision;
2. accepts the complete offline epoch through Network State;
3. constructs Release, Network, and cross-bound Compatibility evidence;
4. creates a temporary enrollment-v3 inventory containing the exact Endpoint
   and control artifacts; and
5. requires every H4-6A inspection result to be `accepted`.

The successful JSON receipt records the encrypted-envelope, request, Endpoint,
control, output, and per-file digests; source revision; validity interval; fixed
TUF/catalog generations; and `preflight=accepted`. It is public provenance
evidence, not an Alpha Enrollment Pin or release acceptance.
