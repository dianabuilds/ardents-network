# Headless Network closed-alpha bundle assembler

This release-workflow adapter creates one deterministic unpackable
Network alpha bundle from already authenticated inputs. It is not a signer,
TUF repository administrator, Authority Vault, key store, downloader, or
release publisher. Private release/control keys never enter this script, its
environment, its output, or the repository.

The caller supplies one explicit `<goos>-<goarch>` and the exact already-built
Endpoint, control, and Authority Custody binaries for it, plus
one direct static directory already prepared by the applicable Release and
alpha-control authority operations. That directory must contain exactly one
matching,
non-zero decimal TUF metadata pair. For the initial generation it is:

```text
1.root.json          1.snapshot.json        1.targets.json
timestamp.json        RELEASE                catalog.ac1
catalog.pub           release.ac1            release.pub
network.ac1           network.pub            compatibility.ac1
compatibility.pub     corpus.pub
```

For a fixed approved successor, only the pair changes together (for example,
`2.snapshot.json` and `2.targets.json`); the trusted root remains
`1.root.json`. The assembler rejects a mixed, duplicated, or unexpected
inventory rather than guessing a metadata generation.

The `RELEASE` descriptor must be enrollment-v4, name the supplied cohort,
release, platform, the Endpoint/control executable names fixed by enrollment
v4, `environment=alpha`, and
the initial trusted root. The assembler copies no unlisted entry, writes the
complete `SHA256SUMS` inventory, and creates one gzip stream with normalized
name order, ownership, and timestamp. The manifest pins the canonical
Custody companion. Enrollment verifies existing v3 headless bundles against
their original manifest, which includes both Node and Custody. Browser
artifacts use separate verification. The output must be a previously absent
absolute path outside the repository.

Run it on an Ubuntu release workstation with GNU `tar`, `gzip`, `sha256sum`,
and `install` available:

```sh
ARDENTS_ALPHA_BUNDLE_COHORT=alpha-1 \
ARDENTS_ALPHA_BUNDLE_RELEASE=usable-alpha-1 \
ARDENTS_ALPHA_BUNDLE_PLATFORM=linux-amd64 \
ARDENTS_ALPHA_BUNDLE_ENDPOINT=/absolute/path/ardents-linux-amd64 \
ARDENTS_ALPHA_BUNDLE_CONTROL=/absolute/path/ardents-control-linux-amd64 \
ARDENTS_ALPHA_BUNDLE_CUSTODY=/absolute/path/ardents-custody-linux-amd64 \
ARDENTS_ALPHA_BUNDLE_STATIC_ROOT=/absolute/path/already-authenticated-static \
ARDENTS_ALPHA_BUNDLE_OUTPUT=/absolute/path/ardents-alpha-usable-alpha-1-linux-amd64.tar.gz \
SOURCE_DATE_EPOCH=1767225600 \
sh ./packaging/alpha-bundle/build.sh
```

`make headless-check` builds real host-named `ardents`, `ardents-control`,
and `ardents-custody` artifacts outside the repository, then supplies the
three exact executable byte strings to the deterministic
inventory test. The test packs,
unpacks, verifies, and byte-compares those artifacts; it does not substitute
fixture text for executable bytes. The Endpoint artifact retains its signed filename and is built from
`cmd/ardents-next`. The target passes the same Endpoint artifact to the selected enrollment and process scenarios.

```sh
make headless-check
```

The printed archive digest is transport/provenance evidence. The Product Owner
still delivers the digest of the **unpacked bundle's `SHA256SUMS`** as the Alpha
Enrollment Pin through the declared authenticated direct message. Publish only
after the signer operation, input provenance, artifact digest, and release
record have been reviewed; GitHub Release/HTTPS is not bootstrap authority.

## Protected generation inventory

For the selected linux-amd64 successor, additionally set
`ARDENTS_ALPHA_BUNDLE_TEXT` to the already-built text executable. The same
static root must then contain `protected-endpoint.json`, the two worker service
units, two worker sockets, `50-ardents-text.rules`, `ardents-text.conf` and
`ardents-endpoint.service`. Any one of these inputs selects the whole required
group; partial presence refuses. The headless bundle has 17 manifest entries. A complete protected bundle has
26, within the existing cap.

The descriptor must already exist before its exact bytes are signed as the
protected target in the same Release set as the Endpoint executable. The
assembler never regenerates it after signing. Its thin Go validation adapter
uses the enrollment owner's canonical grammar and checks all nine digests,
both before copying and against the complete staged inventory before manifest
publication. This validates content consistency, not signing provenance,
fresh Release authorization, execution permission or installed readiness.

The packaging test also produces two identical protected archives, checks
their complete manifests and unchanged descriptor, and refuses substituted
or missing resources without an output archive. Those controls use synthetic
unit/resource bytes and a real command binary as the text-byte fixture;
they do not run or qualify a confined worker or the installed Service route.
