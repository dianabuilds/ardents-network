# Headless Network closed-alpha bundle assembler

This Ubuntu-first release-workflow adapter creates one deterministic unpackable
Network alpha bundle from already authenticated inputs. It is not a signer,
TUF repository administrator, Authority Vault, key store, downloader, or
release publisher. Private release/control keys never enter this script, its
environment, its output, or the repository.

The caller supplies the exact Linux `amd64` Endpoint and control binaries, plus
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

The `RELEASE` descriptor must be enrollment-v3, name the supplied cohort,
release, `linux-amd64`, both fixed executable names, `environment=alpha`, and
the initial trusted root. The assembler copies no unlisted entry, writes the
complete `SHA256SUMS` inventory, and creates one gzip stream with normalized
name order, ownership, and timestamp. Enrollment v3 deliberately contains no
Browser native host or XPI; those remain separately verifiable Adapter
artifacts. The output must be a previously absent absolute path outside the
repository.

Run it on an Ubuntu release workstation with GNU `tar`, `gzip`, `sha256sum`,
and `install` available:

```sh
ARDENTS_ALPHA_BUNDLE_COHORT=alpha-1 \
ARDENTS_ALPHA_BUNDLE_RELEASE=usable-alpha-1 \
ARDENTS_ALPHA_BUNDLE_ENDPOINT=/absolute/path/ardents-linux-amd64 \
ARDENTS_ALPHA_BUNDLE_CONTROL=/absolute/path/ardents-control-linux-amd64 \
ARDENTS_ALPHA_BUNDLE_STATIC_ROOT=/absolute/path/already-authenticated-static \
ARDENTS_ALPHA_BUNDLE_OUTPUT=/absolute/path/ardents-alpha-usable-alpha-1-linux-amd64.tar.gz \
SOURCE_DATE_EPOCH=1767225600 \
sh ./packaging/alpha-bundle/build.sh
```

The contained deterministic-inventory test uses only synthetic non-authority
files and may be run on a Linux host with the same utilities:

```sh
sh ./packaging/alpha-bundle/test.sh
```

The printed archive digest is transport/provenance evidence. The Product Owner
still delivers the digest of the **unpacked bundle's `SHA256SUMS`** as the Alpha
Enrollment Pin through the declared authenticated direct message. Publish only
after the signer operation, input provenance, artifact digest, and release
record have been reviewed; GitHub Release/HTTPS is not bootstrap authority.
