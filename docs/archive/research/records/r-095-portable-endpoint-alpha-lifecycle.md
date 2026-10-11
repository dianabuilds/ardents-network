---
id: R-095
title: Ubuntu-first Portable Endpoint alpha lifecycle and experimental Windows companion
status: decided
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-28
---

# R-095 — Can an Ubuntu-first authenticated Portable Endpoint profile, plus a best-effort Windows companion, give a normal user an explicit obtain, run, stop, restart, and manual-replacement journey without elevation, implicit system integration, or protected-state loss?

## Decision this unlocks

Select or reject the exact Ubuntu-first H4-1A Portable alpha lifecycle: artifact layout,
first-bundle enrollment verification, first-run state creation, explicit start/stop/restart,
Ubuntu per-user session behavior, Windows visible foreground behavior, failure
reporting, and manual verified replacement. Separately verify the explicitly
unsigned/best-effort Windows artifact journey while retaining the same Endpoint
product contract; optional later no-cost signing does not reopen Ubuntu scope.

It does not select a supported Installed profile, system service, autostart,
auto-updater, browser registration, Authority recovery flow, or Public Beta
platform qualification.

## Current contract

- H4-1 selects Ubuntu LTS `x86-64` as the first release-gating Portable alpha
  cell, with unprivileged execution, explicit local state, one release per
  cohort, and manual verified replacement only. Windows 11 is a best-effort
  experimental companion and does not block the Ubuntu alpha.
- Ubuntu may use explicitly enabled `systemd --user` session behavior; Windows
  runs an explicit visible unprivileged binary. Neither alpha path may install a
  system service, enable linger, elevation, autostart, proxy/DNS/VPN settings,
  or background contribution by implication.
- Immutable release artifact, Authority Vault, configuration/Local Grants,
  floors, disposable cache, live state, and diagnostics are separate lifecycle
  classes. Removal and replacement must not silently erase protected state.
- The maintained release/update/custody modules are technical inputs rather
  than evidence of this user-facing platform lifecycle.

Relevant owners: [current C0 scope](../../product/scope.md),
[operating model](../../product/operating-model.md),
[release/update/custody](../../technical/release-update-custody.md), and the
threat model's release and endpoint-compromise boundaries.

## Hypotheses

- **H1:** One documented authenticated Ubuntu Portable flow can meet the H4-1A
  alpha journey with no administrator elevation or implicit system integration,
  while retaining protected state across explicit stop/restart and manual
  verified binary replacement.
- **H2:** A Windows companion can preserve the same Endpoint/state contract but
  needs a visibly separate signing, execution-policy, launch, and verification
  classification; it must not weaken or delay Ubuntu H4-1A.
- **H0:** Ubuntu itself needs background privileged integration, unsafe state
  handling, or an unworkable participant-verification journey; H4-1A must
  change or defer rather than hide that failure in an installer.

## Evaluation criteria

- **Exact outcome:** a non-administrator user can obtain an authenticated
  artifact, verify it before first execution, create protected state in a
  documented owner-chosen location, start the Endpoint, observe readiness or a
  classified failure, stop/restart it, and replace the executable only while
  stopped after verifying the new artifact.
- **State safety:** the flow never writes protected state inside the replaceable
  artifact directory by default, overwrites a Vault/floor on replacement,
  treats a cache as authority, or performs destructive cleanup without an
  explicit enumerated action.
- **Platform fit:** every required operation uses normal user permissions and
  supported native mechanisms; the exact Windows 11 and Ubuntu LTS versions,
  shells, file-system assumptions, and session semantics are recorded.
- **Failure and recovery:** missing/wrong enrollment input, invalid release
  proof, altered artifact, unsupported architecture, occupied state location,
  interrupted stop, crash/restart, incompatible candidate, and failed
  replacement have explicit outcomes that do not silently start unverified/
  older code or destroy state.
- **Usability/maintenance:** the flow is small enough for one person to
  document, test, support, and reverse. It introduces no mandatory signing
  service, package repository, auto-update channel, or browser integration.
- **Claim boundary:** this is an alpha distribution/lifecycle result only; it
  does not demonstrate hostile update resistance after endpoint compromise or a
  qualified installed-platform claim.

## Evidence plan

### Primary sources

- Official Microsoft documentation for Authenticode/code-signing verification,
  Windows Smart App Control/Defender execution behavior, and non-administrator
  executable operation, accessed 2026-08-24:
  [SignTool verification](https://learn.microsoft.com/en-us/windows/win32/seccrypto/using-signtool-to-verify-a-file-signature),
  [Get-AuthenticodeSignature](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.security/get-authenticodesignature?view=powershell-7.5),
  and [Windows app-signing/reputation guidance](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation).
- Official systemd and Ubuntu documentation for `systemd --user`, session
  lifetime, and the absence/presence of linger, accessed 2026-08-24:
  [loginctl](https://www.freedesktop.org/software/systemd/man/252/loginctl.html).
- Official Go release/build documentation for reproducible cross-platform
  artifact constraints where relevant, accessed 2026-08-24:
  [reproducible Go toolchains](https://go.dev/blog/rebuild).
- OpenSSH's SSHSIG protocol and verifier policy, accessed 2026-08-24. The
  signature format binds a mandatory non-empty namespace to prevent
  cross-protocol use; `ssh-keygen -Y verify` separately requires an allowed
  signer, principal, and namespace, and may apply allowed-signer validity or
  revocation policy:
  [PROTOCOL.sshsig](https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig)
  and [OpenBSD `ssh-keygen(1)`](https://man.openbsd.org/ssh-keygen.1).
- The Update Framework specification v1.0.35, accessed 2026-08-24. A TUF client
  starts from trusted root metadata supplied out of band; consecutive root
  rotation is then verified by both the predecessor and successor thresholds:
  [TUF specification](https://theupdateframework.github.io/specification/v1.0.35/).
- Current Ardents product, operating, technical, and threat contracts,
  accessed 2026-08-24.

### Experiment

Before maintaining a profile, create an isolated disposable experiment or
purpose-named test harness for each platform. It must build or use a harmless
signed fixture, execute the exact first-run/stop/restart/manual-replacement
flow under an ordinary user account, and record commands, platform version,
artifact digests, state-tree observations, exit statuses, and deliberately
altered/interrupted cases. The harness must not use or generate real Authority
material.

### Failure scenarios

- A user launches an unsigned, altered, or wrong-architecture binary.
- An artifact is copied/moved/replaced while the Endpoint is running.
- A stopped Endpoint is replaced with an incompatible, revoked, or older build.
- The process crashes, the user logs out, or the host reboots while live state
  exists; protected state must retain only its declared durable portions.
- State path permissions, ownership, or disk space are wrong; a cache is lost;
  a user asks to remove the program but not authority.
- Ubuntu session shutdown stops a user unit; Windows foreground window/process
  exits; neither outcome silently creates persistence or a Contributor duty.

## Findings

- **Sourced fact:** Windows `SignTool verify` has distinct success, failure,
  and warning exit statuses. It is a developer/SDK tool, rather than an
  appropriate required dependency for an ordinary alpha participant.
  [Microsoft SignTool documentation](https://learn.microsoft.com/en-us/windows/win32/seccrypto/using-signtool-to-verify-a-file-signature)
  (accessed 2026-08-24).
- **Sourced fact:** Windows PowerShell's `Get-AuthenticodeSignature` returns a
  signature object for a file and can select only `Status -eq "Valid"`; it is
  documented as Windows-only. A profile must additionally bind the expected
  signer identity or immutable artifact digest—mere presence of a signature is
  not the Ardents release-identity check. [Microsoft PowerShell
  documentation](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.security/get-authenticodesignature?view=powershell-7.5)
  (accessed 2026-08-24).
- **Measurement:** on the current Windows host (Windows NT 10.0.26200.0,
  PowerShell 7.6.4) `Get-AuthenticodeSignature` is available from
  `Microsoft.PowerShell.Security`; `signtool.exe` is absent. The measurement
  establishes only this host's baseline, not Windows 11 support.
- **Sourced fact:** Windows Smart App Control can block unknown or unsigned
  applications. Microsoft recommends signing non-Store distributed releases
  with a certificate from the Trusted Root Program, but a newly signed program
  can still show reputation prompts. [Microsoft Windows app-signing/reputation
  guidance](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation)
  (accessed 2026-08-24).
- **Sourced fact:** Microsoft's current comparison says an unsigned executable
  and a self-signed executable both receive the `Windows protected your PC`
  experience; enterprise policy or Smart App Control may prevent proceeding.
  Even a valid OV/EV signature can show an unrecognized-app warning until
  reputation accumulates. Microsoft's hosted Artifact Signing Basic service is
  currently priced at USD 9.99 per account per month. [SmartScreen reputation](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation)
  and [Artifact Signing tiers](https://learn.microsoft.com/en-us/azure/artifact-signing/how-to-change-sku)
  (accessed 2026-08-24).
- **Sourced fact:** SignPath Foundation offers no-cost Authenticode signing to
  qualifying open-source projects. Eligibility is discretionary and requires,
  among other things, an OSI-approved fully open-source project, active
  maintenance, an existing documented release, a public code-signing policy,
  verified builds, MFA, named signing roles, and manual release approval. The
  certificate publisher is SignPath Foundation, not the Ardents project or its
  maintainer. [SignPath Foundation service](https://signpath.org/) and
  [eligibility/operating conditions](https://signpath.org/terms.html)
  (accessed 2026-08-24).
- **Sourced fact:** GitHub artifact attestations cryptographically connect a
  binary to its repository, workflow, commit and triggering event, and can be
  verified with `gh attestation verify`. GitHub explicitly says an attestation
  is provenance, not a guarantee that the artifact is secure. It is also not
  an Authenticode signature trusted by Windows execution policy. [GitHub
  artifact attestations](https://docs.github.com/en/actions/concepts/security/artifact-attestations)
  and [binary verification](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations)
  (accessed 2026-08-24).
- **Product decision (2026-08-24):** Ubuntu LTS may be the first alpha platform.
  The first Windows artifact is explicitly unsigned. The project will not
  purchase an OV/EV certificate or paid hosted signing service for an alpha
  without users, and release will not wait for acceptance by a no-cost OSS
  service. Windows work continues within available project capacity with
  truthful execution limitations; free signing may be reconsidered later.
  Initial network distribution may use GitHub Releases; an installer or other
  client-facing packaging is added only when observed need justifies it.
- **Inference:** a selected *supported Windows* cell would still need a real
  Windows code-signing/provenance path and a tested blocked-execution outcome.
  It is no longer a prerequisite for Ubuntu H4-1A. A detached digest or GitHub
  attestation can establish artifact identity/provenance under its stated
  verifier, but cannot make an unsigned executable acceptable to Windows
  execution-control policy. A self-signed CA adds trust-store work without
  improving the default SmartScreen outcome and is rejected.
- **Sourced fact:** enabling `loginctl enable-linger` causes a user manager to
  be started at boot and survive logout; it may also require PolicyKit
  authorization. [systemd loginctl](https://www.freedesktop.org/software/systemd/man/252/loginctl.html)
  (accessed 2026-08-24).
- **Inference:** H4-1A's no-linger rule is technically meaningful: the Ubuntu
  user-unit profile must record session/logout as an ordinary stop condition,
  not promise background availability.
- **Sourced fact:** for Go programs without cgo, `CGO_ENABLED=0 go build
  -trimpath` removes the host C toolchain and build-directory path from common
  build inputs, but reproducibility still depends on the complete declared
  build inputs. [Go toolchain reproducibility
  guidance](https://go.dev/blog/rebuild) (accessed 2026-08-24).
- **Inference:** reproducible build work can strengthen H4-6 release evidence,
  but it does not replace Windows code signing or the H4-1 participant
  verification journey.
- **Sourced fact:** the current `ardents endpoint run <endpoint-plan.json>`
  command is a bounded local Endpoint adapter, but its input is explicitly
  temporary and not a supported service-management format. [Current command
  reference](../../reference/commands.md) (accessed 2026-08-24).
- **Inference:** the existing command can inform a harmless lifecycle harness,
  but cannot itself be documented as the H4-1A participant contract. A selected
  alpha lifecycle will need a purpose-owned adapter/configuration boundary and
  its own behavior evidence.
- **Sourced fact:** the present Endpoint plan has no state-root field;
  `endpoint.Run` creates Unix-domain application/result/route listeners and
  removes those paths when its process ends. The retained process test confirms
  readiness followed by bounded failure and socket cleanup, rather than durable
  restart state. [Historical Endpoint runtime](https://github.com/dianabuilds/ardents-network/blob/c6e4222a54ddb49980a5584838a28dcd495118c1/internal/endpoint/endpoint.go),
  [historical Endpoint plan](https://github.com/dianabuilds/ardents-network/blob/c6e4222a54ddb49980a5584838a28dcd495118c1/internal/endpoint/config.go), and [historical process
  test](https://github.com/dianabuilds/ardents-network/blob/c6e4222a54ddb49980a5584838a28dcd495118c1/tests/e2e/service/command_process_test.go) (accessed
  2026-08-24).
- **Inference:** current `endpoint run` cannot be wrapped into an H4-1A
  lifecycle: it provides no protected-state layout or declared supported
  Windows attachment profile. H4-1 needs a selected cross-platform Endpoint
  composition and state-root owner before an artifact or user-service wrapper
  can be tested.
- **Measurement:** `go test ./internal/endpoint -count=1` passed on the current
  Windows host in 2.310 seconds on 2026-08-24. This rejects the narrower claim
  that the present Unix-domain test path is technically unavailable on this
  host. It does not select Windows IPC semantics, test state persistence, or
  qualify a Windows H4-1A profile.
- **Measurement:** the first Ubuntu 24.04 fixture run on 2026-08-24 generated
  an ephemeral Ed25519 key and detached signature, but `gpgv` rejected the
  ASCII-armored public-key export as an invalid keyring. The harness terminated
  nonzero before its success/mutation assertions; its cleanup trap removed the
  exact temporary root. This is a harness-format failure, not evidence that an
  artifact verified. The binary keyring export is the named correction and the
  same scenario must be rerun from a clean root.
- **Measurement:** on local Ubuntu 24.04 WSL (UID 1000, `gpg`/`gpgv` 2.4.4,
  systemd 255, `linger=no`), the corrected fixture run accepted the untouched
  detached signature with `gpgv` exit status `0` and rejected the changed bytes
  with status `1`. The respective SHA-256 digests were
  `42318e0daf234f4bad31597ad9c4a2484f32bb9a0a0fda4c47f1610669982005` and
  `0fdc0290d5dfbf4def92e2cb201db198c254e1de061321130d6e4943d8fcac27`.
  The exact temporary root was absent after the run.
- **Inference:** a detached OpenPGP signature plus an explicitly supplied
  binary keyring is a technically viable Ubuntu alpha verifier candidate. Its
  key provenance, signer rotation/revocation, and full release-metadata
  semantics remain separate R-095/H4-6 decisions; the fixture does not prove
  them.
- **Measurement:** on the current Windows host, the Windows experiment copied
  the Microsoft-signed `wsl.exe` fixture into its exact temporary root. PowerShell
  reported `original_status=Valid`; after changing one byte in the copy it
  reported `changed_status=UnknownError`, which is not `Valid`. The SHA-256
  changed from
  `7e9f5cee6d641481e5a942f0e08563bae9c17ee55f0aad888f9aa0be9a5d4757` to
  `e0ad66131036969c8a976c9907e87140d4888560e7af4497a667385023ee35d5`.
  The exact Windows temporary root was absent after the run.
- **Inference:** the Windows Portable instructions can use the already-present
  `Get-AuthenticodeSignature` to reject any status other than `Valid`, rather
  than requiring the Windows SDK's SignTool. The real profile must still compare
  a declared signer identity and/or Ardents release digest, and must state the
  Smart App Control/reputation block outcome.
- **Measurement:** a harmless Go binary built from the R-101 experiment source
  on the current Windows host had `Get-AuthenticodeSignature.Status=NotSigned`
  on 2026-08-24 (SHA-256
  `73f980dcbe1d9b115fc6bb068e3b92d1245315fe60129c8dbb32ecc88ade91e4`).
  The exact temporary fixture was removed after inspection. Therefore current
  local Go build output cannot pass the proposed `Status=Valid` participant
  verification rule; the earlier Microsoft-signed `wsl.exe` result was only a
  verifier capability check, not evidence of an Ardents release pipeline.
- **Inference:** R-098's disclosure catalog can bind artifact identity after
  its inputs are obtained, but it cannot turn an unsigned Windows executable
  into a valid Authenticode artifact or satisfy Smart App Control/reputation.
  A future selected Windows alpha cell remains conditional on its explicitly
  accepted signing/execution profile and end-to-end participant test; it does
  not gate Ubuntu H4-1A.
- **Sourced fact:** the XDG Base Directory Specification gives persistent state
  and session runtime objects different bases. `$XDG_STATE_HOME` defaults to
  `$HOME/.local/state`; `$XDG_RUNTIME_DIR` must be absolute, local, owned by the
  user, mode `0700`, and tied to login lifetime. [XDG Base Directory
  Specification](https://specifications.freedesktop.org/basedir/) (accessed
  2026-08-24).
- **Sourced fact:** `pam_systemd` creates the per-user runtime directory and
  user manager for a login and normally removes the runtime directory after the
  last logout. Enabling linger instead starts the user manager at boot and
  retains it after logout. [pam_systemd](https://www.freedesktop.org/software/systemd/man/251/pam_systemd.html)
  and [loginctl](https://www.freedesktop.org/software/systemd/man/252/loginctl.html)
  (accessed 2026-08-24).
- **Ubuntu lifecycle measurement:** three complete runs as UID 1000 on Ubuntu
  24.04 WSL with systemd `255.4-1ubuntu8.14` passed the predeclared lifecycle
  matrix. Each real user unit was explicitly enabled, started, stopped,
  restarted, replaced while stopped, disabled, and removed. `Linger` remained
  `no`; config/state/cache/runtime roots were `0700` and the unit was `0600`.
  The reproducible v1/v2 fixture SHA-256 values were respectively
  `cd46e8c6ab957d2ae117445922a1a102f804202fe80b32cddc9e1eec94a7948e`
  and `d17c16b11e0cd8998fd881384f4bae1ca331d940bcd113850a0777ebf033d3e5`.
- **State/readiness measurement:** every run used a newly generated synthetic
  state ID and then observed that same ID, floor 7, and start counts 1→2→3
  through v1 start, v1 restart, and v2 start. Readiness came from an exact Unix
  socket request/response after the fixture had acquired its state lock and
  loaded durable state; systemd `active` alone was not accepted as readiness.
- **Replacement/failure measurement:** all three runs refused the checked
  replacement while the unit was active, rejected a corrupted v2 under its
  original signature, retained the installed v1 digest after an injected stop
  following verified staging but before rename, and then activated verified v2
  from the stopped state. A `0755` state root was rejected. Removing the
  stopped/disabled program root left the synthetic state file present, and the
  runner removed every exact fixture/unit/host-temporary path afterward.
  [Lifecycle experiment](https://github.com/dianabuilds/ardents-network/blob/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-095-portable-endpoint-alpha-lifecycle/README.md)
- **Negative harness result:** the first Windows→WSL wrapper run stopped before
  the matrix because backslashes were stripped from paths passed to `wslpath`.
  Slash-form input corrected it; temporary binaries were cleaned in both
  outcomes. This is orchestration evidence, not a platform lifecycle failure.
- **Inference:** the Ubuntu-first flow does not require an installer for this
  lifecycle. It does require a narrow participant-facing lifecycle owner that
  orders signature verification, service-active refusal, same-directory
  staging, atomic commit, and explicit start. `systemd` supplies supervision;
  it does not authenticate artifacts, provide Ardents capability readiness, or
  prevent the owning user from bypassing the supported replacement path.
- **Limit:** WSL is not Ubuntu Desktop qualification. No logout/user-manager
  restart, reboot, disk-full, kill during atomic rename, actual Endpoint/Vault,
  release-key bootstrap, revoked/older release, or independent participant was
  tested. The synthetic key proves only verifier mechanics.
- **Sourced fact:** SSHSIG includes a mandatory non-empty namespace specifically
  to prevent cross-protocol acceptance. OpenSSH verification additionally
  requires an allowed-signers file, signer identity, and caller-supplied
  namespace; allowed-signers entries can constrain namespaces and validity,
  and verification can consume a revocation file. These controls restrict use
  of an already trusted key; they do not establish why that key is trusted.
  [OpenSSH SSHSIG protocol](https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig)
  and [OpenBSD `ssh-keygen(1)`](https://man.openbsd.org/ssh-keygen.1)
  (accessed 2026-08-24).
- **Sourced fact:** TUF requires a client to start with a trusted root obtained
  through an out-of-band process; TUF does not authenticate arbitrary manually
  downloaded client software by itself. Once enrolled, each consecutive root
  must be verified under both predecessor and successor thresholds. [TUF
  specification](https://theupdateframework.github.io/specification/v1.0.35/)
  (accessed 2026-08-24).
- **Tool-profile measurement:** Ubuntu 24.04 WSL had OpenSSH
  `9.6p1 Ubuntu-3ubuntu13.15`, GNU `sha256sum`, and `gpgv`; `gh`, `cosign`,
  `minisign`, and `signify-openbsd` were absent. This supports a no-install
  OpenSSH candidate only for the observed profile.
- **Bootstrap measurement:** three fresh-key runs signed an exact `SHA256SUMS`
  under namespace `ardents-alpha-bootstrap-v1@ardents.network` and bound an
  `ardents-alpha-0001`/`linux-amd64` descriptor, the fixture artifact, and the
  current public TUF test root. Each accepted the exact fingerprint/principal/
  namespace/signature/checksums/descriptor and rejected changed artifact,
  changed root, changed signed bytes, wrong principal, wrong namespace, wrong
  signing key, and substituted public key. No project executable or publisher
  private key was used by the participant verifier. [Bootstrap
  experiment](https://github.com/dianabuilds/ardents-network/blob/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-095-portable-endpoint-alpha-lifecycle/README.md)
- **Replay measurement:** an `ardents-alpha-0000` bundle signed by the same
  legitimate key passed signature and checksum verification but failed the
  independently expected `ardents-alpha-0001` descriptor. First-install
  anti-replay therefore depends on an out-of-band expected release ID, not on
  SSHSIG or GitHub's release page alone.
- **Architecture conflict:** a distinct SSH bootstrap key can authorize
  arbitrary first-run bytes. Binding a TUF root beside those bytes does not
  force a malicious downloaded executable to honor that root. Selecting this
  key as sufficient execution authorization would create a release authority
  outside `internal/release`, conflicting with ADR-0015 unless explicitly
  bounded as the unavoidable first-install trust root or replaced by an
  already-trusted external TUF verifier.
- **Enrollment-digest measurement:** three Ubuntu 24.04 WSL runs first passed
  the predeclared content/substitution matrix. Review then found that content
  checking alone did not reject a same-content symlink or unexpected inventory;
  those falsifiers and an exact owner-only, single-link regular-file rule were
  recorded before three final expanded runs. All six successful runs modelled
  one exact manifest SHA-256 plus cohort/release/platform/environment/network/
  target facts delivered outside the distributor and produced the same
  manifest digest
  `0c799e4a573727197c71e56298a7c13551ce6a6b05d6937f21dde398fbf2ee88`.
  Changed manifest, artifact, root, and descriptor bytes failed. The final
  matrix also rejected an extra entry, missing root, and same-content artifact
  symlink. A completely substituted bundle and an older bundle each had
  internally correct hashes but failed the pinned manifest digest before
  execution. No key or project verifier was used and the artifact never ran.
  The first sandboxed host call was denied WSL access before the guest
  experiment; its host temporary binary was removed and it is not counted as a
  result. [Enrollment experiment](https://github.com/dianabuilds/ardents-network/blob/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-095-portable-endpoint-alpha-lifecycle/README.md)
- **Inference:** a closed cohort already needs one authenticated value outside
  GitHub. Pinning the exact manifest delegates only one bundle. Pinning an
  SSHSIG fingerprint instead delegates signing power and adds private-key
  custody while still needing an exact release/cohort expectation for replay
  resistance. The reusable key has no demonstrated benefit at this scale.
- **Handoff limit:** the first trusted Endpoint must evaluate the same artifact,
  initial root, and complete metadata through `internal/release` before network
  readiness and persist its floors. That honest-code check makes TUF
  authoritative for successors; it cannot retroactively protect against a
  malicious first artifact that the independent enrollment input authorized.

## Options

1. **Ubuntu-first Portable alpha.** One release-gating platform with detached
   authenticated verification and an explicit per-user lifecycle.
2. **Ubuntu plus a no-cost Authenticode Windows companion.** Adds Windows only
   if an external OSS signing service accepts the project and its operating
   requirements remain supportable.
3. **Ubuntu plus an unsigned experimental Windows companion.** Allows early
   testing but carries explicit SmartScreen/Smart App Control and provenance
   limitations and no supported-Windows claim.
4. **Installed/system-integrated profile first.** Rejected unless the portable
   hypotheses fail; it expands authority, update, support, and removal scope.

### First-artifact bootstrap alternatives

| Candidate | Participant prerequisites | Authority/maintenance result |
|---|---|---|
| Independently delivered exact manifest digest | Preinstalled `sha256sum`; one exact manifest digest plus cohort/release/platform facts through an already authenticated Product Owner contact independent of distribution | Narrowest closed-alpha authority: one pin authorizes one bundle, creates no reusable key/custody system, and must be redelivered for every new cohort/release. |
| OpenSSH SSHSIG over descriptor/checksums | Preinstalled `ssh-keygen` and `sha256sum`; out-of-band fingerprint, release ID, and platform | Mechanically works, but the distinct key is real reusable first-install code authority with custody, loss, compromise, and rotation obligations. It adds no trust when the same channel can carry the exact digest. |
| OpenPGP detached signature | Preinstalled `gpgv`; out-of-band keyring/fingerprint and expected release | Mechanically proven, but adds keyring/export/trust instructions without solving bootstrap or replay. |
| TUF metadata alone | An already trusted external TUF client plus initial root | Preserves one release authority, but no such ordinary-user verifier is currently selected or present. A downloaded Ardents verifier cannot authenticate itself. |
| GitHub attestation/Sigstore | Additional `gh`/`cosign`, workflow-identity policy, network/transparency dependencies | Useful optional build provenance; not present on the profile and not a replacement for Ardents release authorization. |
| SHA-256 from the release page | `sha256sum` only | Detects transfer corruption, but a compromised distributor can replace bytes and digest together; rejected as authentication. |

### Current decision gate

| Candidate alpha scope | Evidence state | Truthful result now |
|---|---|---|
| Ubuntu LTS Portable first | A three-run synthetic user-service lifecycle, three-run SSHSIG envelope, and three-run narrower enrollment-digest matrix work; real independent delivery, Endpoint/Release integration, and native-host qualification remain | Selected alpha direction; the Product Owner accepted the exact-manifest enrollment pin on 2026-08-24. |
| Windows 11 + Ubuntu LTS release-gating Portable | Ubuntu verifier works; no Ardents Windows public signing identity or Smart App Control test exists | Not required for the first alpha. |
| Unsigned Windows companion | Current Go fixture is `NotSigned`; SmartScreen/Smart App Control may warn or block | Selected first-stage Windows direction: visibly experimental, digest/attestation-verifiable, and no Windows support claim. |
| No-cost SignPath Windows companion | Service exists but eligibility is discretionary and the project is not yet an eligible released artifact | Optional later improvement after a documented OSS release; no first-stage dependency. |
| Installed/privileged workaround | No evidence; contradicts H4-1A bounds | Rejected as a workaround. |

## Recommendation

### Accepted platform and enrollment decisions

The Product Owner accepted option 1 as the release-gating profile and option 3
only as its non-gating companion on 2026-08-24. The first alpha is Ubuntu LTS
Portable, using a user-session
unit without linger, GitHub Releases as the initial distributor candidate, and
an independently authenticated first-bundle input that is not supplied by
GitHub. An installer is deferred until the Portable journey supplies a specific
usability or lifecycle reason for one.

Continue Windows as a non-gating, explicitly unsigned best-effort companion.
Publish its SHA-256/build-attestation instructions, expected SmartScreen/Smart
App Control outcomes, and absence of a supported-Windows claim. A no-cost OSS
signing service is an optional later improvement after a documented release,
not a first-stage task. Do not install a project CA, ask users to disable
Windows protections, instruct them to bypass an enforced policy, or convert the
portable experiment into an elevated installer to avoid the warning.

The selected Ubuntu implementation and qualification work has two mandatory
subproblems:

1. implement and exercise the selected first-bundle enrollment/provenance path,
   including its independent participant input, complete inventory, and
   Release identity handoff; and
2. select the Endpoint composition/state-root owner, then run the
   harmless lifecycle experiment on a declared Ubuntu LTS host, including
   state-root and interrupted-replacement cases. Windows runs remain separate
   experimental evidence until that platform is selected.

R-101/R-102 supplied the state-layout, filesystem-socket, held-lock, and guarded
stale-recovery candidates. The maintained H4-1 lifecycle now composes their
Ubuntu shape with a real non-lingering user unit across start/restart and both
successful and rollback-protected v1→v2 replacement. It selects the bounded
state-root owner and Release handoff for this closed alpha only; it does not
establish independent release provenance, a real participant contact channel,
or any supported Windows profile.

The selected Ubuntu subproblem has technically viable GPG and SSHSIG verifier
fixtures, but both introduce a key whose initial trust still arrives outside
GitHub. The enrollment-digest comparison demonstrates a narrower closed-alpha
answer with no bootstrap private key. Windows Authenticode remains a verifier
only for a separately signed companion. None of these fixtures proves a real
invitation channel, complete released bundle, or maintained Endpoint handoff.

### Accepted decision: one-release Alpha Enrollment Pin

For the *first closed alpha cohort*, the Product Owner selected the independently delivered exact
manifest digest rather than SSHSIG. This is deliberately a one-release
enrollment, not a permanent release-signature system:

1. the Product Owner sends the invited participant the exact cohort, release,
   platform, and manifest SHA-256 through an already authenticated contact that
   is independent of the GitHub Release page, download, redirect, and account;
2. the participant compares that digest **before** parsing the downloaded
   manifest, then checks the manifest's exact bounded inventory and descriptor
   with preinstalled tools before setting executable permission or running the
   artifact;
3. the inventory binds the Portable artifact, initial TUF root, complete
   metadata set, descriptor, and every unavoidable static companion; unknown or
   missing required entries fail the supported journey;
4. on first execution the Endpoint supplies those same exact bytes and its own
   executable bytes to `internal/release`. It reports no network readiness
   until the Release Decision is accepted and its root/floors are durably
   committed; and
5. the pin authorizes no successor. Existing Endpoints accept later releases
   only through Release Decision and Update. A new or replacement enrollment
   pin is a new out-of-band invitation, never an in-band rotation.

This protects the first bundle's exact bytes against an adversary controlling
GitHub, a mirror, or the download path, provided the adversary does not control
the participant's existing Product Owner contact and cannot defeat SHA-256.
The three-run substitution matrix is its current measurement. It does **not**
protect against Product Owner/contact compromise, a malicious bundle that the
Product Owner pinned, participant error, or endpoint compromise, and it makes
no independent-custody, public-identity, or scalable-release claim.

The Product Owner accepted this recommendation on 2026-08-24. The actual
contact class must still be named before the first invited participant is
enrolled. A digest published only beside the
bundle is explicitly rejected. If reusable public onboarding becomes a real
need, reopen SSHSIG, a platform signature, or another widely verifiable
identity with its own custody and recovery decision. Because the proposed pin
is intentionally cohort-scoped, authorizes no successor, and is replaceable by
a later onboarding profile, it does not yet meet the hard-to-reverse threshold
for a new ADR; ADR-0015 remains the durable authority boundary.

**Decision confidence:** high that `SignTool` must not be a participant
prerequisite, linger is out of scope, a paid Windows identity is
disproportionate for the first alpha, and a one-bundle pin is narrower than a
reusable bootstrap key; medium that the independent-contact plus manifest flow
will remain small enough for a real Ubuntu participant. **Strongest argument
against the enrollment recommendation:** a fresh pin must be delivered for
every cohort/release, so participant volume or weak pre-existing contact paths
could make it operationally worse than one carefully governed signing identity.

## Disposition

Decided and promoted to H4-1, the operating model, and the Release/Update
technical boundary. Ubuntu-first, the one-release Alpha Enrollment Pin, and an
explicitly unsigned non-gating Windows companion are selected. The maintained
Endpoint/Release handoff and H4-1A/H4-1B native Ubuntu user-session
qualification now exist. On 2026-08-28 the Product Owner used the selected
authenticated one-to-one direct-message class, separately from GitHub, and
enacted the exact immutable RC1 through pre-execution Pin verification, Release
Decision, non-lingering start, retained-state restart, stop, and cleanup. This
is the Product Owner's own functional-alpha walkthrough: no independent
external participant, supported Windows platform, or Public Beta release claim
follows. Windows may add an unsigned artifact run without delaying the Ubuntu
path. Retain this record only until the accepted decision and evidence enter
source history.
