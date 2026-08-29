# Command reference

Status: **current maintained command routes.** These commands exercise the
implemented closed-test-network Modules. They are not a public operator API,
installer, supported Node hosting profile, or compatibility promise for old
plans and results.

Every command fails closed on malformed, unavailable, or unqualified input.
Its bounded JSON input belongs to the owning Module; a plan is not an ambient
configuration format or an authority source.

## `ardents`

`ardents` is the Endpoint and Network State command adapter.

| Route | Purpose |
|---|---|
| `accept-offline --state-root PATH --network-id HEX --authorities HEX,... --threshold N --at RFC3339 --epoch PATH --inputs PATH --materialization PATH` | Accept one complete authenticated offline Network State generation. It emits one `ardents-state-event-v1` `generation-accepted` JSON event. |
| `refresh-sources --state-root PATH --source-plan PATH [--once] [--resume]` | Run one selected Direct-Origin Source wave, or wait for the plan-owned `ardents-source-plan-v1` input. It emits an `ardents-source-event-v1` `source-wave-accepted` event after acceptance. |
| `endpoint run <endpoint-plan.json>` | Run one bounded local Endpoint process. The plan remains Endpoint-owned and temporary; it is not a supported service-management format. |
| `endpoint alpha-browser <alpha-browser-runtime.json>` | Retain one named-alpha Browser runtime from local State, Entry, and corpus owners. Its closed input contains local roots, pinned control keys, local Endpoint broker values, and optionally the existing bounded State Source plan needed to refresh that same root; it rejects a Target, Descriptor, Gateway, Node endpoint, Grant, certificate, or browser URL. The runtime, not a competing `refresh-sources` process, owns the State root while it is live. It publishes only the fixed Browser Entry state expected by the separately installed native host and withdraws it on stop. |
| `endpoint enrollment-check <alpha-enrollment.json>` | Diagnose one already-running artifact against an independently pinned closed-alpha inventory. It does not authenticate first execution. |
| `endpoint enroll <alpha-enrollment.json>` | Run the explicit Ubuntu Portable enrollment/start path after pin and Release Decision verification. |
| `endpoint enroll-installed <package-enrollment.json>` | Run the explicit Ubuntu Installed enrollment/start path for one root-owned package artifact and versioned static enrollment root. |
| `endpoint user-unit <alpha-enrollment.json>` / `endpoint installed-user-unit <package-enrollment.json>` | Render, but never write, enable, or start, the matching `systemd --user` unit. |
| `endpoint replace <replacement-bundle>` | Perform one explicit Ubuntu local Release-authorized replacement against the fixed user unit; it neither downloads nor schedules updates. |
| `endpoint replacement-recovery` | Report durable H4-1B recovery classification only; it never starts, replaces, or rolls back a program. |
| `<journal-bound-recovery-program> endpoint rollback <replacement-bundle>` | Perform the only permitted explicit rollback: the retained predecessor, with a fresh Release authorization for its exact bytes. |
| `entry import <entry-import-plan.json>` | Import one signed State-referenced Entry Invite into Entry-owned durable replay and replacement state. |
| `name encode <name>` | Print one canonical Service Name wire encoding as lowercase hexadecimal. |
| `name resolve <input-file> <name> <context-hex>` | Perform one private resolution exchange through the selected Namespace and State views. |
| `name control <input-file> <operation-file> <context-hex>` | Perform one admitted private Namespace control exchange. |

The current State and source event schemas are coordinated C0 command outputs:
there is no H3 reader or compatibility window. Resource observations are
Module diagnostics, not a capacity or hosting claim.

## `ardents-node`

`ardents-node source --config PATH` runs one selected Direct-Origin Source
server from an `ardents-source-server-v1` input and emits
`ardents-source-event-v1` after its State view is ready.

`ardents-node node --config PATH` runs one separately keyed Node process from
an `ardents-node-plan-v1` input. It
owns one admitted native duty, pressure reaction, drain, withdrawal, and joined
cleanup; lifecycle JSON uses `ardents-node-event-v1`. On Linux, `SIGTERM` and
the foreground interrupt request that local withdrawal before process exit.
An arbitrary Node config is not a supported Node operating profile. A config
file is a bounded Node-owned input, not a general Node configuration contract. Every
native-duty stanza must set its finite `admission_timeout_ms`: it bounds TLS
and binding admission only, is capped by the current State expiry, and has no
implicit default or retry/fallback behavior.

The Rendezvous stanza may additionally set `listen_loopback_override` only to
a literal loopback IP with the same numeric port as the authenticated
State-advertised Rendezvous candidate. This is an operational bind adapter for
a host-owned byte-transparent Carrier relay: State remains the sole owner of
the advertised endpoint, identity, digest, Epoch, and Carrier profile. A
hostname, unspecified or non-loopback address, zero/out-of-range port, or port
mismatch is rejected. Omitting the field retains the State endpoint as the
listener and does not change existing plan behavior.

On Linux, `ardents-node contributor` exposes the complete candidate
`h4-5-rendezvous-alpha-v1` dedicated-host lifecycle: `apply`, `diagnose`,
`restart`, `drain`, `withdraw`, and confirmed `remove`. It accepts no other
duty or system-service operation. The exact prerequisites, authenticated
bundle, commands, limits, diagnostics, update recovery, and residue contract
are in the [Rendezvous Contributor candidate runbook](rendezvous-contributor.md).
The surface remains candidate-only until its fresh-host qualification closes
R-092 and H4-5; it is not a public Contributor offer.

## `ardents-custody`

`ardents-custody` is separate from Endpoint, Release, and Update. It accepts no
Authority material or password through flags, environment, configuration, or an
Application data stream.

| Route | Required flags | Result |
|---|---|---|
| `inspect-envelope` | `--vault-root PATH --envelope PATH` | Validates and prints only canonical public envelope facts as `ardents-custody-inspection-v1`. |
| `verify-record` | `--vault-root PATH --record ID --environment-commitment HEX --network-commitment HEX --root-commitment HEX --kind service|name --id-commitment HEX` | Reads one password from an interactive no-echo terminal, verifies one active encrypted record against exact public commitments, and prints bounded non-secret facts as `ardents-custody-verification-v1`. |
| `export-recovery-bundle` | Exact vault record, public Authority commitments, and output Bundle path | Reads the record secret only from the terminal, writes a separately passworded Bundle, and test-restores it before success. |
| `restore-recovery-bundle` | Empty destination Vault, public commitments, Bundle path | Reads the Bundle password from the terminal and writes only an `authority-locked` quarantine record. |
| `purge-record` | Exact vault record, public commitments, and terminal confirmation | Deletes only the exact verified encrypted record after explicit confirmation while retaining the Authority floor. |

The custody command deliberately exposes no recovery activation, reconciliation
without a Namespace witness, or Namespace signing route. A restored Name
Authority remains locked until a separate fresh opaque Namespace witness is
implemented and verified.

## `ardents-release-custody`

`ardents-release-custody` is the Product Owner's separate local alpha
release-seed custody adapter. It accepts no password through flags,
environment, configuration, or shared stdin. It exposes one fixed-profile
metadata assembly route but no generic signer, upload, artifact publication,
Endpoint start, or VPS configuration route.

| Route | Required flags | Result |
|---|---|---|
| `initialize` | `--root ABSOLUTE_OWNER_ONLY_DIRECTORY` | Reads a new local passphrase and confirmation, then creates the one encrypted fixed-role seed record and prints its public receipt. |
| `inspect` | `--root ABSOLUTE_OWNER_ONLY_DIRECTORY` | Reads the existing local passphrase, authenticates the fixed encrypted record without altering it, and prints only its public receipt. |
| `assemble` | `--root ABSOLUTE_OWNER_ONLY_DIRECTORY --request ABSOLUTE_FILE --endpoint ABSOLUTE_FILE --control ABSOLUTE_FILE --output ABSENT_ABSOLUTE_DIRECTORY` | Authenticates the selected H4-alpha-1 envelope, accepts only the recorded profile/source/artifact identities, constructs and preflights the fixed TUF/H4-6A static input set, then prints a public receipt. It does not assemble or publish the release bundle. |

## `ardents-state-custody`

`ardents-state-custody` is the separate Product Owner Adapter for the one
ADR-0053 functional-alpha Network State genesis. It accepts only
`initialize-alpha-genesis --root ABSOLUTE_OWNER_ONLY_DIRECTORY`. The Module
generates the Network identifier, 1-of-1 Epoch key, assignment seed, fixed
30-day validity, and empty candidate view internally. It asks for a new local
passphrase and confirmation, atomically creates `functional-alpha-state` below
the supplied root, and prints a non-secret receipt.

The child contains the encrypted `state-seeds.json` and public
`alpha-network-state.json` request fragment. The latter declares
`empty-no-persistent-node`: it is valid H4-6A control input, but never evidence
of route readiness, operator capacity, availability, independent control, or
Public Beta governance. The command exposes no generic signer, successor,
Node-key, upload, or publication route.

## `ardents-control`

`ardents-control` is a separate alpha-control program. It never starts an
Endpoint, downloads bytes, changes Release/Network State roots, or turns an
alpha name into canonical Namespace state.

| Route | Required flags | Result |
|---|---|---|
| `inspect` | `--directory PATH --state-root PATH --disclosure-key HEX --release-key HEX --network-key HEX --compatibility-key HEX --at RFC3339` | Low-level statement-integrity diagnostic for an explicit ACA1 directory and caller-pinned component roots. It advances the catalog child of the named reader root and deliberately does not perform enrollment, artifact, Release Decision, or Network State verification. It is not the participant H4-6A route. |
| `inspect-bundle` | `--enrollment PATH --artifact PATH --state-root PATH --at RFC3339` | Participant H4-6A route. It first verifies the exact enrolled bundle and artifact, then runs the maintained Release Decision, Network State, ACA1 catalog, and Release/Network/Compatibility component verifiers. The named standalone inspection root owns separate `catalog`, `release`, and `network` floor children and is physically distinct from Endpoint state. Repeating the same accepted input against the same inspection root reports the cached/no-update outcome; a second absent inspection root supplies an independent fresh observation. |
| `inspect-alpha-corpus` | `--catalog PATH --corpus PATH --state-root PATH --disclosure-key HEX --corpus-key HEX --network HEX --at RFC3339` | Verifies one explicit ACA2 catalog and separately signed Alpha Name Corpus under independent keys, then writes only the corpus serial/digest/bytes to its named persistent floor. It reports `ardents-alpha-corpus-report-v1`; stale, conflicting, changed, expired, or wrong-network input fails. |
| `accept-alpha-corpus` | `--enrollment PATH --artifact PATH --control-state-root PATH --corpus-state-root PATH --catalog PATH --corpus PATH --at RFC3339` | First verifies the exact enrollment-v3-or-later bundle, its enrolled Endpoint executable, and that the running platform-specific `ardents-control` file is the exact separately manifested companion. It then accepts the fixed ACA1 Release/Network/Compatibility evidence and verifies the independently pinned ACA2/corpus component before advancing only the named Endpoint-local corpus floor. An exact repeat is harmless; a higher serial replaces the retained corpus, while a lower or same-serial-different input fails. It reports `ardents-alpha-corpus-acceptance-v1`. |

None of these routes launches a browser, opens a Service, chooses a Relay/Gateway, or
makes an `ardents-alpha://` link a public DNS/HTTPS address. The acceptance
route retains only supplied bytes after its checks; it does not fetch or
install an Endpoint.

## `ardents-browser-entry`

`ardents-browser-entry native-host --state ABSOLUTE_PATH` is the fixed Firefox
native-messaging adapter for the selected alpha Browser Entry. Firefox invokes
it through an exact-ID native manifest, not an operator shell. It consumes one
bounded `loopback-proxy-port` or `loopback-proxy-authentication` native frame
and returns its port (and, only for the latter, a one-process proxy password)
only after the
current Endpoint-owned AlphaProxy answers a random local liveness probe. It
never receives a Service Name, Target, Service credential, route, or browser
URL; its bounded authentication answer is only the separate current local proxy
password. It cannot install the extension, open a browser, resolve a name, or
act as a proxy.

The separately invoked participant route
`ardents-browser-entry install --enrollment PATH --endpoint-artifact PATH --at RFC3339`
accepts only an enrollment-v4 bundle. It first verifies the exact enrolled
Endpoint artifact, the running native-host executable, and the fixed-ID XPI
companion against one independently delivered manifest pin. It then creates or
replaces only the current user's exact native-messaging registration. On
Windows this is the one `HKCU\Software\Mozilla\NativeMessagingHosts` value;
on Ubuntu it is the one per-user Firefox native-manifest file. The command
prints the XPI path and `manual-required`: the participant still installs the
Mozilla-signed unlisted XPI explicitly in Firefox. `ardents-browser-entry
remove` withdraws only that native-manifest registration; it cannot remove the
XPI, Endpoint state, Authority, corpus, or Release floors. The installer also
rejects an XPI without the current Mozilla COSE signature metadata, but leaves
cryptographic signature acceptance to Firefox when the participant explicitly
installs that exact XPI.

The matching add-on source and manifest template are in
`packaging/firefox-alpha-browser-entry`. A Mozilla-signed XPI and a real
enrollment-v4 release bundle remain H4-4 promotion artifacts; repository
source or a GitHub download alone is not an installed Browser Entry profile.

## Process and support limits

All maintained commands return a non-zero process status when their bounded input or
owned lifecycle fails. The repository's current process tests cover retained
source and Node lifecycles; they do not qualify public deployment, platform
durability, privacy, or Node capacity. See the current technical contracts for
[Network, Route, and Node](../technical/network-route-node.md),
[Endpoint and Service](../technical/endpoint-service-runtime.md), and
[Release, Update, and Custody](../technical/release-update-custody.md).
