# Closed-alpha Alpha Name Corpus intake

Status: **closed-alpha participant procedure template; no published alpha corpus
source is currently promoted.**

This procedure lets an already enrolled participant accept one finite Alpha
Name Corpus for the `ardents-alpha://` overlay. It is not public DNS, a name
registration workflow, an automatic updater, or a browser configuration
procedure.

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
`ardents-control-<platform>` companion. Its verified `SHA256SUMS` inventory
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

Put the downloaded pair outside the enrolled bundle. The following Ubuntu
example uses owner-only state roots and does not start an Endpoint, modify DNS,
install a certificate, or launch a browser.

```sh
set -eu
umask 077

bundle=/absolute/path/to/unpacked-bundle
artifact="$bundle/ardents-linux-amd64"
control="$bundle/ardents-control-linux-amd64"
enrollment="$HOME/.local/state/ardents-alpha/declared-release/alpha-enrollment.json"
incoming=/absolute/path/to/received-alpha-corpus
state_root="${XDG_STATE_HOME:-$HOME/.local/state}/ardents-alpha/name-corpus"
control_root="$state_root/control"
corpus_root="$state_root/corpus-floor"

test -f "$artifact"
test -f "$control"
test -f "$enrollment"
test -f "$incoming/catalog.ac2"
test -f "$incoming/corpus.anc"
mkdir -p "$state_root"
chmod 700 "$state_root"

"$control" accept-alpha-corpus \
  --enrollment "$enrollment" --artifact "$artifact" \
  --control-state-root "$control_root" --corpus-state-root "$corpus_root" \
  --catalog "$incoming/catalog.ac2" --corpus "$incoming/corpus.anc" \
  --at "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
```

Each floor root is owner-only and must belong to the account running the
command. The command refuses a group-accessible, symlinked, or foreign-owned
floor root rather than repairing or claiming it.

The command is the `ardents-control` binary already verified as a regular file
in the same exact bundle. It checks the enrolled Endpoint artifact named by
`--artifact`, but it is not itself an Endpoint and does not start one. This
template cannot be promoted from a bundle that lacks that manifested command.
A future Endpoint package may expose the same operation only through a
documented, equivalently verified command; do not substitute an arbitrary
binary merely because it has the same command name.

Success prints `ardents-alpha-corpus-acceptance-v1` with `corpus: accepted`,
the Network and retained serial. A non-zero result means the corpus floor was
not advanced. The independent ACA1 inspection root can retain its own accepted
control evidence; it is not a usable name floor.

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
