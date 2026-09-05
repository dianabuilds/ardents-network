# Closed-alpha Alpha Name Corpus intake

Status: **deferred closed-alpha design template; it is not a maintained C0
operator procedure and no published alpha corpus source is promoted.**

This retained template describes a separately selected future Alpha Name Corpus
intake route for an already enrolled participant. It is not part of the
maintained Target Link-to-Connection journey and must not be used as a
first-enrollment instruction while it remains deferred. Re-activating it
requires a separately selected issue to reconcile its inputs with the current
portable enrollment contract. It is not public DNS, a name registration
workflow, an automatic updater, or a browser configuration procedure.

## Trust boundary

The delivery location is a byte carrier only. It may be a GitHub Release asset,
an HTTPS download, removable media, or a message attachment; none of those
locations is a name authority. The participant must already have:

- a verified enrollment-v3-or-later bundle and its independently delivered Alpha
  Enrollment Pin, as described in [Closed-alpha Ubuntu Portable
  enrollment](closed-alpha-enrollment.md);
- the exact enrolled Endpoint artifact **and** `ardents-control` command from
  that same bundle; and
- two explicitly supplied files for one corpus serial: `catalog.ac2` and
  `corpus.anc`.

The v3-or-later bundle pins the corpus public key and the platform-specific
`ardents-control-<platform>` companion (`.exe` is part of its canonical name on
Windows). Its verified `SHA256SUMS` inventory
must list both executable files; the accepting command verifies that it is
that exact manifested companion, so a separately downloaded control command
is not an acceptable substitute. `catalog.ac2` binds the exact corpus bytes and
serial under the separately pinned disclosure key. The signed corpus itself
binds the finite alpha name set, Network, validity period, and either its
active bindings or an explicit total withdrawal. A delivery URL, release page,
file name, or message cannot retarget a name on its own.

During closed alpha, the Product Owner must provide the location through the
same authenticated contact class that supplied the Enrollment Pin. Use the
immutable [cohort notice template](closed-alpha-name-corpus-notice-template.md)
for each published serial. This is a notice of where to obtain bytes, not
additional authority. An unavailable,
changed, expired, lower-serial, or conflicting pair is a failed update; the
participant does not look for a Target Link, another resolver, or an alternate
name source.

## First intake

There is deliberately no executable first-intake procedure in this deferred
template. In particular, do not create or retain an `alpha-enrollment.json`
file and do not infer that `ardents-control accept-alpha-corpus` accepts a
current Portable enrollment argument. The historic command sequence was
removed because its JSON input no longer matches the current manifest-pinned
Portable contract.

Before this template can become a live procedure, its selected issue must
define one concrete command invocation that receives the bundle root and the
independently delivered manifest pin without reintroducing an ambient
enrollment document. That change must also preserve the owner-only corpus and
control floor roots, exact manifested Endpoint/control binaries, corpus
serial/floor behavior, and explicit failure outcome described here.

## Replacement and withdrawal

For a later announced serial, repeat the command with the new pair and the
same `control_root` and `corpus_root`.

- Repeating identical accepted bytes is harmless.
- A valid higher serial atomically replaces the retained corpus.
- A lower serial, or different bytes at the retained serial, fails and leaves
  the retained corpus unchanged.
- A valid higher withdrawn corpus is accepted as control evidence, then every
  alpha resolution returns the explicit `withdrawn` state. It does not restore
  a previous corpus or fall back to a Target Link.

The participant may retain received files as personal evidence, but they are
not part of the owned floor and must never be copied into its root. The floor
rejects unknown files and protects its own atomic writes.

## Promotion gates

Before this becomes a live alpha instruction, publish one concrete source
notice from the template containing the cohort, serial, expiry, file locations,
and authenticated contact class; ship the verified control command as a manifest entry in that
same alpha bundle; attach the exact two files to that source; and run this
procedure from a fresh enrolled Ubuntu Endpoint. The resulting evidence must
cover initial acceptance, higher-serial replacement, rollback refusal, total
withdrawal, unavailable source, and a C-2 name journey using the retained
floor. A source notice does not turn its host into a registry, Namespace
authority, automatic updater, public DNS service, or browser integration.
