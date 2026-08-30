# Retired fixed-alpha custody assembly reference

Status: **retired historical command/format contract.** ADR-0059 removed both
assembly routes and their implementation after the exact RC1/RC2 ceremonies
completed. The material below preserves the byte and evidence provenance of
those ceremonies; it is not an available command, a procedure, or a contract
for a future candidate.

## Historical commands

The retired `assemble` route accepted only these absolute paths:

```powershell
ardents-release-custody assemble `
  --root ABSOLUTE_OWNER_ONLY_DIRECTORY `
  --request ABSOLUTE_FILE `
  --endpoint ABSOLUTE_FILE `
  --control ABSOLUTE_FILE `
  --output ABSENT_ABSOLUTE_DIRECTORY
```

The retired `assemble-successor` route was only the recorded RC1-to-RC2
continuation and added a complete direct predecessor directory:

```powershell
ardents-release-custody assemble-successor `
  --root ABSOLUTE_OWNER_ONLY_DIRECTORY `
  --request ABSOLUTE_FILE `
  --endpoint ABSOLUTE_FILE `
  --control ABSOLUTE_FILE `
  --predecessor ABSOLUTE_RC1_STATIC_DIRECTORY `
  --output ABSENT_ABSOLUTE_DIRECTORY
```

The retired adapter read an existing passphrase only through its local no-echo
secret input. It accepted neither a secret through flags, environment, configuration,
stdin, receipt, nor request JSON. Request files are direct regular files up to
12 MiB; Endpoint/control inputs are direct regular files up to 64 MiB; output
must be absent. The adapter rejects symlinks, changed files, relative paths,
unknown flags, and all input failures before secret use.

## `ardents-alpha-input-request-v1`

The JSON decoder rejects unknown or trailing fields. Every shown field is
required unless marked optional; byte arrays use standard base64 and times are
UTC whole-second RFC 3339 values.

```json
{
  "schema": "ardents-alpha-input-request-v1",
  "profile": "ardents-h4-alpha-1-v1",
  "cohort": "TOKEN", "release": "TOKEN", "release_version": 1,
  "reference_time": "YYYY-MM-DDTHH:MM:SSZ",
  "not_before": "YYYY-MM-DDTHH:MM:SSZ",
  "not_after": "YYYY-MM-DDTHH:MM:SSZ",
  "build_safety_no_new_work_after": "YYYY-MM-DDTHH:MM:SSZ",
  "build_safety_terminate_after": "YYYY-MM-DDTHH:MM:SSZ",
  "environment": "alpha", "network": "TOKEN",
  "source_revision": "40-lowercase-hex-characters",
  "endpoint_sha256": "64-lowercase-hex-characters",
  "control_sha256": "64-lowercase-hex-characters",
  "build_input_commitment": "TEXT", "build_identity": "TEXT",
  "dependency_identity": "TEXT", "sbom_identity": "TEXT",
  "qualification": "VALUE", "build_state": "VALUE", "protocol_phase": "VALUE",
  "protocol_overlapped_since": "OPTIONAL_RFC3339",
  "capacity_ready": false, "drain_ready": false,
  "emergency_reason": "OPTIONAL_TEXT", "emergency_expiry": "OPTIONAL_RFC3339",
  "builders": ["DISTINCT_TEXT_A", "DISTINCT_TEXT_B"],
  "network_state": {
    "network_id": "64-lowercase-hex-characters",
    "epoch_digest": "64-lowercase-hex-characters",
    "profile": "TOKEN", "threshold": 1,
    "authorities": ["64-lowercase-hex-ed25519-public-key"],
    "epoch": "BASE64", "inputs": ["BASE64"], "materials": ["BASE64"]
  }
}
```

The retired code accepted only its recorded profile, source revision,
Endpoint/control digests, and encrypted-envelope identity. It rejected changed artifacts,
unaccepted lifecycle values, duplicate authorities, invalid time ordering,
stale invocation, out-of-bound network bytes, a caller-selected role/key/path,
or an unaccepted Network State. The ordinary announced phase has no optional
overlap or emergency fields.

## Result and verification

`assemble` atomically exposed exactly fourteen direct static files only after
Release, Network State, catalog, component, enrollment, and H4-6A preflight
acceptance. `assemble-successor` retained `1.root.json`, required the fixed
predecessor, and emitted the fixed generation-2 successor. Both returned only the
public `ardents-alpha-inputs-receipt-v1` digests, source identity, validity,
generations, file inventory, and `preflight` result.

The retired `internal/release/custody` code owned semantic validation; the thin
command owned absolute-path and bounded-file admission. Historical behavior tests covered request
rejection before secret use, deterministic/atomic output, preflight failure,
and public-only receipt rendering. Historical RC2 execution evidence is
retained in R-119/R-120/R-121 and the historical H4-alpha-1 matrix; neither the
record nor the retired implementation qualifies a post-refactor candidate.
