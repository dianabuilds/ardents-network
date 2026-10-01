# Operator-prepared qualification release

[ADR-0120](../adr/0120-authorize-operator-prepared-qualification-release.md)
selects this isolated operation. It does not qualify installation or independent
distribution. The Release verifier and participant first-pin contract remain
unchanged. Never use diagnostic keys as the operational release authority.

## Prepare exact public inputs

Build the five linux-amd64 commands twice from the exact recorded source tree
with `CGO_ENABLED=0`, `-trimpath` and `-buildvcs=false`. Retain byte comparisons,
toolchain, module/build settings and build-input commitments. Generate the
protected descriptor twice from the actual nine resources; retain matching
bytes. The descriptor's canonical grammar is owned by enrollment. Validate it
with `scripts/protected-generation-check.go` before signing.

Prepare a bounded public JSON signing plan with schema
`ardents-qualification-release-signing-plan-v1`, `network`, `reference_time`,
`expires`, absolute `endpoint`, `descriptor`, `resource_root` paths, and
`endpoint_custom`/`generation_custom`. Custom values use the existing Release
grammar. The two values must agree on all common policy/build facts; only
their actual target-digest rebuild records differ. Version is 1 for this fresh
cohort. Use two actual project-controlled execution receipts, not fabricated
identities presented as independent builders. Record the component inventory's
scope and the exact checks behind the proposed artifact admission.

The fixed targets are `ardents/linux-amd64/endpoint` and
`ardents/linux-amd64/protected-endpoint`. Metadata expiry is finite, at most
seven days from the current reference time. The signer rejects a reference
older than five minutes or more than one minute in the future. Refresh an
unsigned plan before execution; never edit signed descriptor or metadata bytes.
Use `announced` for the isolated initial protocol, without invented overlap
history or emergency grounds.

## Protected operation on Linux

Build the two adapters explicitly from their source files:

`make release-operation-compile-check` vets and cross-compiles both Linux
adapters. It is required by `quick-check` and the complete `check` gate;
compilation is not a key/signing-operation or installed qualification result.

```sh
go build -trimpath -buildvcs=false -o /absolute/new/prepare-release-keys ./scripts/prepare-qualification-release-keys.go
go build -trimpath -buildvcs=false -o /absolute/new/sign-release ./scripts/sign-qualification-release.go
```

Run on the selected root-controlled release host. Supply previously absent
absolute output directories under root-owned direct ancestors with no group or
other write permissions. Private keys are PKCS#8 files protected by root-only
filesystem permissions; these files are not encrypted Custody Vaults and add
no Custody unlock or general Authority signing interface.

```sh
/absolute/new/prepare-release-keys /root/new-release-keys > /root/new-public-key-receipt.json
/absolute/new/sign-release /root/public-plan.json /root/new-release-keys /root/new-release-metadata > /root/new-public-signing-receipt.json
```

Key creation uses a new `0700` directory and exclusive `0600` files, syncs
outputs and ancestors before a public acknowledgement, and never overwrites an
existing directory. Signing admits exactly five distinct keys with the existing
ordinary 3-of-5 roles. It checks descriptor/resources and common target facts,
then verifies both targets through separate fresh diagnostic Release floor
roots before writing the four metadata files. Those verification roots do not
alter participant floors and are not bundle entries.

Any error or interruption leaves its partial output for inspection and emits
no success acknowledgement. Do not reuse that directory or infer completion
from its existence. Preserve the original refusal before an explicit new
operation. A file-size-limit failure exercises bounded write refusal; it is
not a power-loss durability qualification.

## Assembly and acceptance boundary

The explicit `scripts/prepare-qualification-alpha-evidence.go` adapter prepares
initial unsigned disclosure payloads and component signing inputs. Build it
with the same canonical Linux build flags above. Pass an absolute public JSON
plan and a previously absent absolute output directory. The plan schema is
`ardents-qualification-alpha-evidence-plan-v1`; its other top-level fields are
`not_before`, `not_after`, `release`, `network`, and `compatibility`. Times are
UTC whole-second RFC3339 values. Nested evidence fields use the exported field
names of the current `inspection` evidence contracts: byte slices are JSON
base64 strings and fixed 32-byte arrays are JSON numeric arrays. Unknown fields
and trailing JSON values refuse; the public plan is bounded to 32 MiB.

Preparation checks canonical evidence grammar and shared Release/Network/
Compatibility bindings, including the inspected Epoch identity. It writes
`release.payload`, `network.payload`, and `compatibility.payload`, each with
a `.signing-input` companion containing the exact domain-separated initial
ACS1 signing message. Generation is fixed to one; existing signatures, negative
or fractional timestamps and invalid validity intervals refuse. The new output
directory and files use `0700`/`0600`, exclusive creation and synchronization
before acknowledgement. Preserve partial outputs on failure. This public-input
adapter does not enforce the private-key ancestor policy of the Release signer.
It creates no keys or signatures, authenticates no State Epoch, and grants no
Release, Network or Endpoint acceptance. Separate role signatures and ordinary
complete-bundle inspection remain required.

After separately signing the component bytes, the explicit
`scripts/prepare-qualification-alpha-catalog.go` adapter prepares the initial
ACA1 signing message. Its absolute public JSON plan uses schema
`ardents-qualification-alpha-catalog-plan-v1` and a `catalog` object with the
exported `alphacontrol.Catalog` fields. Use the same JSON array/time conventions
above. It bounds the plan to 1 MiB, rejects unknown fields/trailing values,
fixes catalog and component generations to one, refuses a predecessor digest
or existing signature, and requires every component expiry after catalog start
and no later than catalog expiry. References must name the exact signed
component sizes/digests and their separately pinned roots. Preparation checks
grammar only; it does not verify the referenced bytes or grant their authority.
The new exclusive synchronized output is `catalog.signing-input`, with the
same output permissions and partial-output retention as evidence preparation.
Signing this message under the separate disclosure key and ordinary inspection
of the resulting full bundle remain mandatory.

The four metadata files alone are not the complete bundle. Prepare the real
current alpha-control and Network companions with their own maintained owners;
keep authority roles separate. Supply exact signed descriptor/resources and
all required static companions to the maintained alpha-bundle assembler.
Verify the complete 27-entry unpacked inventory and collect its manifest digest.
Wrong-pin, mixed/expired metadata, resource substitution and incomplete inventory
must refuse. Do not copy the signing verification roots or private keys into
the bundle.

Present the completed public root commitments, artifact digests, bounded
artifact-admission evidence and manifest digest to the Product Owner. Installed
execution waits for that exact bootstrap receipt to be accepted. Record the
same-operator trust boundary honestly; this does not prove independent initial
delivery. Two installed Endpoint roots, public Custody/permission operations,
both Carrier journeys and actual containment/empty-scope observations remain
separate acceptance evidence.
