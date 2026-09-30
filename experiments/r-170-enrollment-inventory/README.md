# R-170 existing enrollment inventory probe

Question: can the present v3 verifier distinguish a complete protected text
inventory from arbitrary independently pinned extra files?

Hypothesis: it authenticates those bytes but does not recognize completeness or
project them separately from Release metadata. Falsification: a complete text
set projects outside metadata, or a re-pinned partial text set is refused by
the existing headless verifier. This is a component probe, not an installed
workflow, valid Release Decision, real independent-pin delivery or candidate
implementation.

`probe.go.txt` appends a test to the existing enrollment test fixture via Go's
overlay. It neither modifies maintained runtime nor creates a project package.
From the repository root in PowerShell:

```powershell
$evidence = Join-Path $env:TEMP ('ardents-r170-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $evidence | Out-Null
$original = (Resolve-Path internal/enrollment/enrollment_test.go).Path
$replacement = Join-Path $evidence 'enrollment_test.go'
$text = [IO.File]::ReadAllText($original) + "`n" + [IO.File]::ReadAllText((Resolve-Path experiments/r-170-enrollment-inventory/probe.go.txt).Path)
[IO.File]::WriteAllText($replacement, $text, [Text.UTF8Encoding]::new($false))
$map = @{ Replace = @{ $original = $replacement } } | ConvertTo-Json -Depth 4
$overlay = Join-Path $evidence 'overlay.json'
[IO.File]::WriteAllText($overlay, $map, [Text.UTF8Encoding]::new($false))
go test -overlay $overlay ./internal/enrollment -run '^TestResearchExistingProtectedInventory$' -count=1 -v *> (Join-Path $evidence 'result.txt')
$LASTEXITCODE
```

Retain exit status, transcript, overlay/source hashes and repository revision
outside Git. Repinning in the probe models delivery of a different independent
pin; it is not an authorized runtime self-repin. Fixture executables and static
bytes are synthetic. The fixture has no valid signed Release metadata.

Result: 2026-09-30, Windows, Go 1.26.8, repository `126761e9` plus the documented
test overlay: exit 0, six observations matched the predeclared hypothesis.
Old v3 headless and re-pinned partial text inventories were accepted; all eight
complete synthetic resources appeared in Release metadata. Predecessor pin,
undeclared extra file and worker digest substitution were refused. Test time
0.30 s; package time 0.640 s. Replacement test source SHA-256:
`8a5d57db4f1554a87d4500fd2410b4bcd7e30d044882c73676cf4f7c8f096476`.
Overlay SHA-256 (includes private absolute paths):
`082d67e9558cf791f58fd17fdaaf7ad2986755a3b2a3ef49125083985e65eec0`.
Raw evidence is retained privately outside Git. The PASS confirms the diagnosed
gap; it does not mean the protected contract passes.

Disposition: retain this disposable source as
reproducible provenance; it supplies no installed producer or accepting bypass.

## Existing Release two-target probe

`release-probe.go.txt` uses the same overlay recipe, substituting
`internal/release/public_vector_test.go`, package `./internal/release`, test
`^TestResearchTwoTargetsShareFloors$`. Predeclared question: does current Release
authenticate a second target after shared floors are committed, with a fresh
opaque authorization on no-update? Refusal/absent proof falsifies that premise.
Negative controls change the second artifact bytes or request an absent target.

2026-09-30 Windows/Go1.26.8 result: exit0; package0.587s, test0.11s. Two
distinct targets in one synthetic signed set returned release-accepted then
no-update, both with opaque authorizations and equal floors. Changed artifact
and absent target returned release-invalid without authorization. Replacement
source SHA-256 `d707417739530c3ea3091e72b97d1cde0fc553b027aecf6dc93b48b7a5631498`.
Evidence retained privately outside Git. The fixture uses ephemeral signing
keys and identical synthetic artifact bytes at two distinct target paths; it
does not validate the proposed generation descriptor, unequal artifacts,
installation composition, a signing service or installed behavior.

## Different target bytes

`release-distinct-probe.go.txt` uses the overlay recipe with
`internal/release/testfixtures_test.go`, package `./internal/release`, test
`^TestResearchDifferentTargetBytes$`. Before evaluation it re-signs one synthetic
metadata set containing a 4096-byte program and a different 54-byte descriptor
target, with matching target-specific builder digest records. The predeclared
oracle requires accepted then no-update with matching floors and distinct
authenticated digests. Controls substitute program bytes or alter one descriptor
byte; both must yield no authorization and preserve committed floors.

2026-09-30 Windows/Go1.26.8: exit0, package0.349s, test0.01s. Positive and
both negative observations matched; each negative returned release-invalid.
Replacement source SHA-256:
`358d3bb21fadf89c40a040ed360c6694c5471a20e1b6e709f42ca380ddd8290a`.
Private raw evidence remains outside Git. This uses in-memory fixture floors,
ephemeral fixture signing keys and synthetic descriptor text. It does not parse
the proposed generation grammar or implement the protected owner. Earlier
equal-byte probe evidence remains retained as a narrower result.
