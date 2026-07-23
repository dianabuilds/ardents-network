# Distribution Model

Ardents has two public executable roles, independent of deployment mechanism.

| Artifact | Role | Required where |
| --- | --- | --- |
| `ardentsd` | Node daemon, native bootstrap, network and optional workload control | Every node host |
| `ardentsctl` | Operator CLI/TUI over the authenticated local control API | Operator workstation or node host |
| `ardents/node` image | Optional packaging of `ardentsd` and `ardentsctl` | Docker deployments only |
| `ardents-ingress-proxy` image | Optional isolated forwarding adapter | Only Docker workloads that publish admitted ingress |

The proxy is not a third public installation prerequisite. It belongs to the
Docker workload adapter and is acquired as an image only when an admitted
hosted service needs isolated ingress. Nodes with workload execution disabled,
native applications that expose their own endpoints, and workloads without
published ingress do not need it.

The proxy is versioned and attested with the node release, but remains a
separate minimal image so its exposed container does not carry daemon or
operator tooling. `release-manifest.json` binds the node and proxy exports to a
single release and declares the supported ingress protocol. The Docker executor
rejects a locally available proxy image whose protocol label is incompatible.
After registry publication, the separately attested `published-images.json`
maps both component names to their exact GHCR digests for operator configuration.

## Runtime Selection

`workloads.executor=disabled` is the default and keeps the daemon independent
of Docker. `docker` is an explicit operator choice and requires a configured
Docker Engine. `trusted-process` is restricted to local development and is not
a production substitute for container isolation.

The daemon must remain useful in all three shapes: network-only node, native
node plus a separately managed application, and Docker node controlling Docker
workloads. Packaging may add convenience but must not change the node's core
protocol or require an additional public executable.

## Bootstrap

`ardentsd init` owns first-node initialization. It creates or restores the node
identity, protected capability and replay state, an operator configuration, and
an API token. Native paths default to the supplied data and secret directories;
container provisioning explicitly maps those paths to the runtime mount points.

Remote operator access is provided by `ardentsctl --ssh`, using the system
OpenSSH client to reach the daemon's loopback control API without exposing that
API on the network. Same-host automation may use the private Unix socket emitted
by `ardentsd init`. A public application SDK remains a separate delivery layer;
it should build on versioned application protocols rather than introducing
another daemon-shaped binary.

## Release Channels And Verification

Release builds use the current checked-out Git `HEAD` as their only source tree.
The optional `-Commit` argument is an assertion that must resolve to that exact
commit; it does not select a historical tree. Packaging fails before Docker is
invoked when tracked or untracked source changes are present. Ignored build
outputs do not change source identity, but there is no dirty-build override.
After validation, packaging exports the exact commit with `git archive`; every
bundle, image, metadata, and verification step reads that immutable snapshot
instead of the mutable worktree. Ignored files are therefore never build input.

Version tags publish the native archive, standalone binaries, node and optional
ingress-proxy Docker image exports, `release-manifest.json`, `SHA256SUMS`,
attestation subject manifest `ARTIFACTS.sha256`, CycloneDX SBOM, and explicitly
unsigned local provenance statement as GitHub Release assets. The node and
proxy images are published separately to GHCR under their release versions and
version-plus-commit candidate identities; no mutable `latest` tag is part of the
release contract. Candidate tags also include the workflow run identity and are
retained as immutable publication evidence rather than reused as moving
channels. Published GitHub Releases additionally contain
`published-images.json`; release-candidate artifacts created without registry
access intentionally do not.

GitHub Actions signs build-provenance and SBOM attestations with its OIDC-backed
Sigstore identity. Release files use the Go application SBOM shipped with the
assets; the published image digest receives a separate container-aware SBOM that
also inventories its operating-system packages. Consumers must first verify
`SHA256SUMS`, then verify the downloaded artifact against the repository identity
with `gh attestation verify`. The `provenance.unsigned.intoto.json` file inside
the release is useful offline metadata, but it is not by itself a signature and
must not be treated as one.
