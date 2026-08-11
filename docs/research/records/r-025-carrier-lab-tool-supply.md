---
id: R-025
title: How are Carrier Lab shaping and capture tools supplied immutably?
status: decided
owner: product research
started: 2026-08-09
reviewed: 2026-08-09
---

# R-025 — Carrier Lab tool supply

## Decision this unlocks

Permit the native R-013 Carrier Lab candidate to use real `tc netem` and real
packet capture without adding network access or package mutation to normal
build/run, granting the Application runtime laboratory privileges, or making
laboratory tooling a product dependency. This record unlocks only supply and
smoke verification. It does not implement C-5/C2 or select a production Route,
Carrier Channel, packaging system, or deployment contour.

## Current contract

[Product scope](../../product/scope.md) authorizes only Carrier Lab.
[R-013](r-013-carrier-lab-technology-candidates.md) fixes Ubuntu 26.04
`linux/amd64`, the controlled `100 Mbit/s` links, an `80 ms` endpoint RTT
floor, real `tc netem`, bounded per-link captures, immutable tool identity,
external raw evidence, and cleanup. ADR-0009 keeps first-party Go free of cgo
and `unsafe`; ADR-0010 forbids product Modules from importing laboratory code.
The existing pinned Application image remains unprivileged and contains no
shaping or capture binaries.

The open point was the supply path. An image name or package popularity was
not evidence of provenance, license fit, maintenance, or threat-model fit.

## Hypotheses

- **H1:** A fixed external bundle of official Ubuntu `.deb` artifacts can be
  checked by SHA-256, extracted without maintainer scripts into a locally built
  laboratory-only image, and used by separate namespace-sharing shaping and
  capture roles with only `NET_ADMIN` and `NET_RAW` respectively.
- **H2:** A prebuilt public OCI tooling image provides a smaller or safer trust
  and maintenance surface while meeting the same provenance, license, offline,
  privilege, and removal conditions.
- **H0:** No evaluated external path meets every condition; R-025 remains open
  and native C-5/C2 stays blocked.

## Evaluation criteria

The criteria below were fixed before preparing or executing the bundle:

- every supplied file has an exact version, filename, SHA-256, official source,
  architecture, and reviewed license record;
- the base image and normal build/run are offline; a missing file, extra file,
  wrong digest, wrong package identity, wrong tool version, wrong runtime binary
  digest, or absent capability fails closed;
- no package manager, maintainer script, download fallback, cgo, first-party
  `unsafe`, hand-written netlink, or hand-written packet parser runs;
- the Application roles keep all capabilities dropped; a shaping role receives
  only `NET_ADMIN`; a capture role receives only `NET_RAW`; no role is
  `privileged`;
- tooling shares only the exact laboratory role's network namespace, on the
  Compose-internal network, and sees no Service, Target, Application Data, Route,
  discovery, or topology information beyond its fixed synthetic link;
- raw capture is created only below the external run-owned temporary directory,
  hashed and checked through `tcpdump`, then removed before a pass can be
  issued; cleanup validates path ownership and is repeatable after failure;
- a Product Owner plus Codex can update or remove the path by replacing one
  lock and tooling target without changing a future Route Module Interface.

### Falsification criteria

Stop implementation and leave R-025 open if any of the following occurs:

- an artifact cannot be obtained from an official Ubuntu archive with a stable
  content digest or its license/redistribution terms are unclear;
- a floating repository/tag, normal-run Internet access, unrestricted
  `privileged` mode, product import, cgo, or `unsafe` is required;
- an unresolved Ubuntu Critical/High vulnerability is reported for the exact
  selected source package;
- `tc netem` cannot establish and report the fixed impairment with only
  `NET_ADMIN`, or capture cannot open a packet socket with only `NET_RAW`;
- filtering cannot bound capture to the fixed synthetic link/port, or the raw
  capture cannot remain outside the repository and be removed reliably;
- replacing the tooling would require changing C-5/C2, naming, discovery,
  bootstrap, production runtime, or a future Route Module Interface.

## Evidence plan

### Primary sources

All sources were accessed on 2026-08-09.

- iproute2 `tc` and `netem` manuals: [tc(8)](https://man7.org/linux/man-pages/man8/tc.8.html)
  and [netem(8)](https://man7.org/linux/man-pages/man8/tc-netem.8.html);
- tcpdump/libpcap upstream: [tcpdump](https://github.com/the-tcpdump-group/tcpdump)
  and [libpcap](https://github.com/the-tcpdump-group/libpcap);
- Ubuntu package/source metadata: [iproute2 6.19.0-1ubuntu1.1](https://packages.ubuntu.com/resolute-updates/iproute2),
  [tcpdump 4.99.6-1](https://packages.ubuntu.com/resolute/tcpdump), and
  [libpcap 1.10.6-1ubuntu1](https://packages.ubuntu.com/source/resolute/libpcap);
- exact binary archive locations and package-owned copyright records from the
  [Ubuntu archive](https://archive.ubuntu.com/ubuntu/pool/main/) and each
  selected `.deb` under `/usr/share/doc/<package>/copyright`;
- [Ubuntu CVE export](https://ubuntu.com/security/cves) for the selected source
  packages and Resolute Critical/High priorities;
- Docker Compose [service attributes](https://docs.docker.com/reference/compose-file/services/),
  [internal networks](https://docs.docker.com/reference/compose-file/networks/#internal),
  and Docker [named build contexts](https://docs.docker.com/build/concepts/context/#named-contexts);
- Docker's [Official Images source of truth](https://github.com/docker-library/official-images),
  its current [`library/ubuntu` build metadata](https://github.com/docker-library/official-images/blob/master/library/ubuntu),
  and the [Ubuntu Official Image record](https://hub.docker.com/_/ubuntu),
  which traces the image to Canonical rootfs inputs;
- the concrete public-toolbox comparator, upstream
  [`nicolaka/netshoot`](https://github.com/nicolaka/netshoot), and its
  [Docker Hub artifact records](https://hub.docker.com/r/nicolaka/netshoot/tags);
- Linux [capabilities(7)](https://man7.org/linux/man-pages/man7/capabilities.7.html),
  which assigns network administration to `CAP_NET_ADMIN` and raw/PACKET
  sockets to `CAP_NET_RAW`.

### Experiment

Environment: pinned Ubuntu 26.04 `linux/amd64` image
`sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960`,
Docker 29.1.3 under Docker Desktop, and an external system-temporary directory.
The online preparation step used the image's signed official Resolute,
Resolute Updates, and Resolute Security indices only to obtain exact files.
Normal verification then used `--network none`.

Procedure:

1. Download the exact direct packages plus APT's no-recommends closure into the
   external directory during an explicit preparation step.
2. Hash and inspect every `.deb`, its control identity, and its installed
   copyright record.
3. Compare the full install closure with the ELF runtime closure of only
   `/usr/sbin/tc` and `/usr/bin/tcpdump` on the pinned base.
4. Extract the minimal closure with `dpkg-deb --extract`, without invoking a
   package manager or maintainer script; run `ldd`, version commands, and
   runtime SHA-256 checks with no network.
5. Query Ubuntu's official CVE JSON export for Critical/High entries for every
   selected source package on Resolute.
6. Build the derived tooling image offline, then execute shaping, capture,
   forced-failure, and cleanup smoke. The implementation section records those
   results; a Docker Desktop result remains development evidence only.

The prepared `.deb` files, image layers, captures, and generated evidence stay
outside Git. Binary inputs, derived images, and raw captures are disposable;
bounded retained summaries may remain in the external run-owned evidence
directory. Reproduction starts from the exact URLs and hashes in
[`lab/carrier/tools.lock`](../../../lab/carrier/tools.lock).

### Failure scenarios

- missing/extra artifact, digest mismatch, package/version mismatch, or wrong
  runtime binary identity;
- absent `NET_ADMIN`/`NET_RAW`, `tc` failure, or `tcpdump` startup failure;
- empty capture, capture outside the owned directory, unexpected peer/network,
  or capture of traffic outside the fixed filter;
- sidecar or controller interruption after only part of the contour starts;
- evidence write or raw-capture deletion failure;
- mutable source update, superseded security package, or base-image replacement.

## Findings

### Sourced facts

- `netem` is an iproute2 queue discipline for applying delay, rate, loss, and
  related impairments to outgoing packets on a selected device; `tc` reports
  effective qdisc state.
- libpcap uses Linux packet sockets and can apply a kernel BPF filter; tcpdump
  writes and reads pcap files. The lab can therefore verify marker traffic
  through the external tool rather than a first-party packet parser.
- Compose `network_mode: service:<name>` joins the named service namespace;
  `internal: true` creates an externally isolated network; `pull_policy: never`
  fails if an image is absent instead of downloading it.
- Linux separates the required powers: queue/interface administration is under
  `CAP_NET_ADMIN`; raw and PACKET sockets are under `CAP_NET_RAW`.
- Ubuntu publishes `iproute2 6.19.0-1ubuntu1.1` in Resolute Updates. The earlier
  `6.19.0-1ubuntu1` is not the selected input.

### Measurements

- The pinned base contains neither `tc` nor `tcpdump`. A conventional
  no-recommends APT installation selected 25 artifacts (`9,846 kB`) and would
  add systemd/package-maintainer behavior irrelevant to the two required
  binaries.
- For OCI candidate A, the Ubuntu Official Image has a public build/provenance
  chain but is only a base and contains neither required tool. The concrete
  prebuilt comparator `nicolaka/netshoot:v0.16` publishes per-platform content
  digests and includes both tcpdump and iproute2, but is a roughly 209 MB
  general-purpose toolbox containing scanners, packet constructors,
  interpreters, Kubernetes utilities, and many other tools. Its published
  artifact identity does not provide the exact Ubuntu package closure and
  package-level license record required here. An Ardents-published minimal OCI
  image could close that gap only by reusing option B's verified package supply
  and adding registry provenance, signing, redistribution, retention, and
  revocation operations that the actual two-person collaboration model does
  not otherwise need.
- ELF inspection reduced the external runtime closure to the 12 artifacts in
  the lock. Offline extraction produced a complete `ldd` closure on the exact
  base and reported `iproute2-6.19.0`, `tcpdump 4.99.6`, and `libpcap 1.10.6`.
- Runtime SHA-256 values are `tc`
  `2444a535db549341d3fd7caeba73f20bdb188e196a59ee9ca5b6deb08738dea1`,
  `tcpdump`
  `b19250abc948f03f637ecf8845114ee03132ad0ce12526c180388b2ae6680fd3`,
  and `libpcap.so.1.10.6`
  `d6bee52c78494de5b9eb87998e66395c2cbb52b567da00c236f6f5efea233c5e`.
- Ubuntu's CVE JSON export returned zero Critical and zero High records for
  `iproute2`, `libbpf`, `libcap2`, `dbus`, `elfutils`, `rdma-core`, `libmnl`,
  `libnl3`, `libpcap`, `iptables`, and `tcpdump` on Resolute at access time.
  This is a dated maintenance signal, not a permanent safety claim.
- Docker Desktop cleared the requested effective capability after exec for
  numeric user 65532, both with and without `no-new-privileges`. The dedicated
  sidecars therefore use UID 0 with `cap_drop: ALL` and exactly one added
  capability. Observed `CapEff` was `0x1000` for each shaper and `0x2000` for
  capture; tracers observed zero. All roles remained read-only,
  `no-new-privileges`, resource-bounded, internally networked, and
  non-privileged.
- The mandatory pre-build verifier first found the locked Ubuntu base locally
  as image ID
  `sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960`;
  a missing base fails before Docker receives a build request. The subsequent
  build with `--pull=false --network=none` verified all 12 package
  hashes/control records plus all three runtime hashes and produced local image
  ID `sha256:7044a145123e64fa5b2264bfac6ecd5f71c1346bae692f7560e2b39674c70021`.
  Before Compose, the controller bound that runnable image to source digest
  `c45556c69526a4bb4eabdaed340b008038c1e2a3979d0d0e8b087aa0e03da938`,
  Carrier Lab binary digest
  `2b277c93a6633c40f4401ef48eabc77e0b9afe119757908cdf7a148f7a046a7f`,
  the locked base reference, and lock digest. The prior syntactically valid
  image ID was rejected before Compose because it lacked this receipt contract.
- Development smoke recorded both real qdiscs as `limit 1000 delay 40ms rate
  100Mbit`, produced a 2,180-byte pcap containing the
  run-local tracer, recorded pcap SHA-256
  `4dd0af6057129474774683e8a8683aad7df8517dddd55387a0047ab3f46f221f`,
  deleted the raw pcap, and proved the exact five-container project, the exact
  two tracer network attachments, reciprocal alpha/beta peer observations,
  isolation, and cleanup. The
  retained tooling manifest SHA-256 was
  `7b71b7e3d9a1df8ae1fb296204b377afea64cc3e8b311a9898cc74629ce8c7e1`
  and the separate passing verdict bound that exact digest.
- Injected `capture-start` failure exited non-zero, retained `status: failed`,
  removed the run directory and raw capture, and reported
  `cleanup_complete: true`. No retained failure contained a host path. The
  application image
  `sha256:79cb507961983616182e3bab6eb2766cc091df7ec01f1ce22cd36efe14860257`
  also passed the separate two-role isolation smoke, and the native Direct TLS
  control passed. No Compose container, network, or volume remained after any
  run.

### Assumptions

- The exact Ubuntu base digest remains the ABI boundary for its already present
  libc, OpenSSL, systemd, zlib, and zstd libraries. Changing that digest forces
  the full closure and smoke to be repeated.
- The fixed synthetic port and one namespace-shared interface are sufficient
  for supply smoke; C-5/C2 will define its own per-link bindings later.
- Locally building the tooling image is supportable by the actual one-to-one
  team; publishing that image would add separate GPL/source-notice obligations.

### Inferences

- Extraction is smaller and safer for this bounded role than installing the
  complete packages: it executes no maintainer code, has no resolver behavior,
  and copies only content whose package and runtime identities are locked.
- A package digest alone is insufficient evidence. Binding the final runnable
  image to the base, lock, source snapshot, executable hash, observed versions,
  effective capabilities, complete qdisc contract, exact peer set, and capture
  hash closes independent substitution points.

## Options

| Option | Product/security fit | Disposition |
|---|---|---|
| A. External OCI tooling image(s) | The official Ubuntu digest has traceable Official Images/Canonical provenance but no tools. The evaluated `nicolaka/netshoot:v0.16` artifact has platform digests and the tools, but is a large third-party toolbox without the exact package/license closure required here. Publishing an Ardents-minimal image would still depend on B and add registry signing, retention, redistribution, and revocation work. | Rejected as the external supply root. The locally derived image is only a disposable result of option B. |
| B. External locked Ubuntu `.deb` bundle | Official artifacts have exact identities and copyright records; a named build context keeps binaries outside Git; offline extraction avoids mutable repositories and scripts. | **Chosen supply path.** |
| C. Namespace-sharing shaping/capture sidecars | Provides a real privilege boundary and makes tooling removable without product imports. It is an execution contour rather than an artifact source. | **Chosen with B:** separate shaping and capture roles. |
| D. First-party netlink/packet sockets | Adds security-sensitive Linux APIs, packet parsing, maintenance, and likely `unsafe`/cgo pressure without evidence that external tools fail. | Rejected. |

## Exact selected artifacts

The normative filenames, versions, SHA-256 values, official URLs, license
summaries, target paths, tool versions, and runtime hashes are in
[`lab/carrier/tools.lock`](../../../lab/carrier/tools.lock). The 12 artifacts
are `iproute2`, `libbpf1`, `libcap2`, `libdbus-1-3`, `libelf1t64`,
`libibverbs1`, `libmnl0`, `libnl-3-200`, `libnl-route-3-200`,
`libpcap0.8t64`, `libxtables12`, and `tcpdump`.

License review is acceptable for local laboratory use. The lock records the
package-level summary; the exact Debian-format copyright text remains inside
each artifact. No package is redistributed by Git. Any future distribution of
the derived image must preserve the applicable notices and satisfy source-code
obligations, especially the GPL-covered packages; that future distribution is
not authorized by this decision.

## Recommendation

Choose option B with option C as one minimal path: an external exact `.deb`
bundle, verified against the committed lock, is extracted into a locally built
laboratory-only image; separate namespace-sharing roles run shaping and capture
with `NET_ADMIN` and `NET_RAW`. The Application image keeps all capabilities
dropped and never imports or invokes those tools.

Confidence is **high for supply smoke** and **not a C-5/C2 or security
qualification claim**. The strongest counterargument is that ELF closure
selection bypasses the distribution package manager's whole-package dependency
model. Binding it to one base digest, verifying every artifact and runtime
binary, running `ldd`/version smoke, and invalidating it on any base/package
change bounds that risk without executing unrelated maintainer code.

Update/removal plan: a security or maintenance signal opens a new R-025 review,
replaces the complete lock, changes the derived image identity, and reruns all
tooling smoke. Removal deletes the tooling target, Compose profile, roles, and
lock from the three shared Carrier Lab container sources while leaving the
Application target and future Route Module Interface unchanged. No ADR is
created: this is a replaceable laboratory input with an explicit removal seam,
not consequential product lock-in.

## Disposition

R-025 is **decided and implementation-smoke complete**. It no longer blocks a
separate native C-5/C2 implementation task. C-5/C2 itself remains unimplemented
here. The committed repository retains only the research record, text lock,
Docker/Compose source, Go orchestration/tests, and documentation; all binary
inputs and generated evidence remain external and disposable.

The smoke classification is `development` because its controller ran on
Windows/Docker Desktop. It establishes tool supply and lifecycle behavior, not
an official R-013 Route result. The future native task must repeat its complete
condition on the official Ubuntu 26.04 `x86-64` runner. Root inside the two
tool-only sidecar contours is the principal remaining privilege limitation;
the exact observed capability mask, read-only filesystem, isolated namespace,
mount scope, and absence of `privileged` mode bound it explicitly.
