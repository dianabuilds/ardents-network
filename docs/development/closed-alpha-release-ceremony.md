# Closed-alpha release ceremony

Status: **active H4-1/H4-6A preparation. The bounded static-input operation is
maintained; its real invocation remains inactive until the Product Owner records
the exact public alpha profile and topology.**

This procedure prepares one bounded Ubuntu Portable alpha artifact. It derives
from H4-1, ADR-0038, the H4-8A matrix, and the closed-alpha enrollment
procedure. It is not a public-release, independent-control, reproducible-build,
availability, or independent-participant claim.

## Preconditions

Before any signing or publication, retain an external, access-controlled
release workspace that is outside the repository and its working tree. It must
hold private release/control material only for the Product Owner's declared
custody process. No private key, passphrase, decrypted copy, shell history,
environment variable, CI log, GitHub Release asset, or repository file may be
used as a substitute for that process.

The Product Owner records, outside the public bundle:

- the accountable key custodian and recovery/rotation owner;
- the approved release identity, cohort, version, expiry and emergency-stop
  times;
- the exact Ubuntu Portable platform and source revision;
- the actual alpha Network State, Node/operator topology, and corpus authority
  that the bundle will disclose; and
- the authenticated direct-message recipient class for the independently sent
  Alpha Enrollment Pin.

One project-operated key holder is visible provisional alpha control. It is not
threshold custody, independent control, or a Public Beta gate.

The Product Owner initializes the local seed record once with
`ardents-release-custody initialize --root C:\Users\vitek\Ardents-Release\keys`.
On Windows the command presents two local password dialogs, which support paste;
on other platforms it uses a local no-echo terminal. The passphrase contains 16
to 1024 bytes; the command reports only a length-policy failure or a
nonmatching confirmation, never the secret. Neither passphrase nor private
material belongs in a command line, environment, repository, chat, shell
history, bundle, CI log, release asset, or VPS. Initialization returns a public
receipt only; it does not yet sign metadata or publish a release.

When the initialization receipt must be recovered for companion recording, run
`ardents-release-custody inspect --root C:\Users\vitek\Ardents-Release\keys`.
It asks once for the existing passphrase and prints only the encrypted-record
digest and fixed public keys. It does not alter the record or create a signed
release input.

The fixed public request is prepared under
[`closed-alpha-input-request.md`](closed-alpha-input-request.md). After its
facts and the two exact artifacts are reviewed, the Product Owner invokes
`ardents-release-custody assemble` locally. That command performs the complete
Release/Network/H4-6A preflight and publishes one previously absent static
directory. It does not assemble or upload the bundle.

## Signed input set

The release workspace produces and independently verifies one static set before
any bundle assembly:

1. Initial trusted TUF root and the timestamp, snapshot, and targets metadata
   that authorize the exact Endpoint binary and its H4-1 release facts.
2. A signed H4-6A catalog and separate signed Release, Network, and
   Compatibility statements, with their four independent public companions.
3. The public alpha-corpus authority companion and the exact declared corpus
   lifecycle input, when the selected cohort includes H4-4A.
4. The canonical `RELEASE` descriptor matching the selected cohort, release,
   `linux-amd64`, exact target path, Endpoint and control artifact names, and
   all fixed component names.
5. The cross-built `ardents-linux-amd64` and `ardents-control-linux-amd64`
   bytes for the recorded source revision.

The signer operation must use maintained Release/TUF and H4-6A codecs; test
fixtures that generate ephemeral keys or test identities are not release
inputs. The prepared static directory is checked with the actual verifier
before assembly. Any invalid signature, expired input, incomplete metadata,
descriptor mismatch, changed binary, unrecorded authority, or failed Network
State/corpus verification stops the ceremony.

## Assembly and publication

With all inputs already authenticated, use
[`packaging/alpha-bundle/build.sh`](../../packaging/alpha-bundle/build.sh) to
assemble a deterministic archive outside the repository. Its generated
`SHA256SUMS` is the item pinned independently to the participant; the archive
digest is additional transport/provenance evidence.

Before GitHub publication, record the source revision, toolchain, assembler
version/digest, exact archive digest, unpacked `SHA256SUMS` digest, static-input
digests, signing timestamps/expiry, and the H4-8A result. Publish only one
explicitly labelled closed-alpha prerelease with the archive and this public
non-secret receipt. GitHub, HTTPS, a release-page checksum, and a project
signature delivered through the same channel are not first-install trust.

## Enrollment evidence

The Product Owner independently sends the chosen participant the cohort,
release, platform, and unpacked-`SHA256SUMS` digest through the declared
authenticated direct message. The participant follows
[`closed-alpha-enrollment.md`](../product/closed-alpha-enrollment.md) before
any downloaded executable runs.

The retained receipt distinguishes:

- the Product Owner's own walkthrough from an independent participant run;
- successful pre-execution inventory verification from Endpoint readiness;
- successful Endpoint lifecycle observation from H4-6A control inspection; and
- an observed limitation or failure from a passing release gate.

No row becomes green merely because a bundle was assembled, uploaded, or
downloaded. The H4-8A matrix is updated only with the concrete immutable URL,
digests, authority record, platform observation, and participant evidence.
