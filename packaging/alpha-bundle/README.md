# Headless Network closed-alpha bundle assembler

This release-workflow adapter creates one deterministic unpackable
Network alpha bundle from already authenticated inputs. It is not a signer,
TUF repository administrator, Authority Vault, key store, downloader, or
release publisher. Private release/control keys never enter this script, its
environment, its output, or the repository.

The caller supplies one explicit `<goos>-<goarch>` and the exact already-built
Endpoint and Authority Custody binaries. Participants run the Endpoint
executable as `ardents`. The offline Authority operator uses
`ardents-custody`. [ADR-0130](../../docs/adr/0130-retire-control-and-aca1-from-alpha-bundles.md)
requires both in the enrollment-v5 inventory. The static directory contains the
descriptor, trusted root and one matching, non-zero decimal TUF metadata
pair with its timestamp. For the initial generation it is:

```text
1.root.json          1.snapshot.json        1.targets.json
timestamp.json        RELEASE
```

For a fixed approved successor, only the pair changes together (for example,
`2.snapshot.json` and `2.targets.json`); the trusted root remains
`1.root.json`. The assembler rejects a mixed, duplicated, or unexpected
inventory rather than guessing a metadata generation.

The `RELEASE` descriptor uses enrollment-v5. Its ordered fields name the
cohort, release, platform, environment, network, target path, Endpoint
executable and trusted root. The assembler writes the complete `SHA256SUMS`
inventory and creates one gzip stream with normalized names, ownership and
timestamp. The manifest pins the platform-named Authority Custody
artifact. The output is a previously absent absolute path outside the
repository.

Run it on an Ubuntu release workstation with GNU `tar`, `gzip`, `sha256sum`,
and `install` available:

```sh
ARDENTS_ALPHA_BUNDLE_COHORT=alpha-1 \
ARDENTS_ALPHA_BUNDLE_RELEASE=usable-alpha-1 \
ARDENTS_ALPHA_BUNDLE_PLATFORM=linux-amd64 \
ARDENTS_ALPHA_BUNDLE_ENDPOINT=/absolute/path/ardents-linux-amd64 \
ARDENTS_ALPHA_BUNDLE_CUSTODY=/absolute/path/ardents-custody-linux-amd64 \
ARDENTS_ALPHA_BUNDLE_STATIC_ROOT=/absolute/path/already-authenticated-static \
ARDENTS_ALPHA_BUNDLE_OUTPUT=/absolute/path/ardents-alpha-usable-alpha-1-linux-amd64.tar.gz \
SOURCE_DATE_EPOCH=1767225600 \
sh ./packaging/alpha-bundle/build.sh
```

`make headless-check` builds the real host-named `ardents` and
`ardents-custody` artifacts outside the repository. Its deterministic packaging
test packs, unpacks, verifies and byte-compares both executables. The Endpoint
artifact comes from `cmd/ardents-next`. The same Endpoint bytes reach the
selected enrollment and process scenarios.

```sh
make headless-check
```

The packager runs the Enrollment operations from reviewed `cmd/ardents-next`
source. The supplied programs are inputs to the builder. The packaging test
also exercises the separate installed-program verification command.

The printed archive digest is transport/provenance evidence. The Product Owner
still delivers the digest of the **unpacked bundle's `SHA256SUMS`** as the Alpha
Enrollment Pin through the declared authenticated direct message. Publish only
after the signer operation, input provenance, artifact digest, and release
record have been reviewed; GitHub Release/HTTPS is not bootstrap authority.

## Protected generation inventory

For the protected linux-amd64 generation, build the document worker from
`./internal/application/document/worker` and supply it through
`ARDENTS_ALPHA_BUNDLE_TEXT`. The protected inventory records it as
`ardents-text-linux-amd64`. The installed systemd units launch it. The same
static root must contain `protected-endpoint.json`, the two worker service
units, two worker sockets, `50-ardents-text.rules`, `ardents-text.conf` and
`ardents-endpoint.service`. Any one of these inputs selects the whole required
group. Partial presence refuses. The headless bundle has seven manifest entries.
A complete protected bundle has 16, within the existing cap.

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
unit/resource bytes and a real command binary as a worker-byte fixture.
Installed worker behavior has its own
[network qualification profile](../../tests/qualification/document-worker-network/README.md).
