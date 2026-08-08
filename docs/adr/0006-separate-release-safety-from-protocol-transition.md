---
status: accepted
date: 2026-08-08
---

# Separate release safety from protocol transition

This decision applies when a closed test network begins producing installable
release candidates. Carrier Lab and early Reference Application experiments have
no automatic updater, public threshold, or protocol-migration subsystem.

Ardents uses two independent lifecycle machines. Protocol generations move from
announced through overlap-supported, preferred, required, and retired, with at
least 90 days of current/previous overlap except an expiring threshold emergency
for a credible exploitable flaw, compromised primitive/key, or demonstrated
safety incompatibility.
Software builds move from current to superseded and may separately become
vulnerable or revoked at any time. A safe superseded build is not the same as an
incompatible protocol, and a compatible revoked build opens no new network work.
Signed transition policy carries finite no-new-work and terminal/no-recovery
deadlines. Until its deadline a vulnerable build keeps the same exact qualified
contract or fails closed; it never receives a weaker profile. Recovery after the
terminal deadline is new security work, and owner drain cannot extend it.

Every live Route, Service Connection, publication, and Contributor duty has a
finite Work Safety Lease no later than its applicable epoch, release-safety,
protocol/build, credential, and role-specific terminal bounds. Authenticated
refresh may extend it before expiry. New leg attachment or recovery requires
current safety state; an old live stream cannot preserve revoked or stale trust
indefinitely.

Capability readiness includes an expiring cached Release Safety State distributed
independently of its authority. Every executable digest requires the public
Targets threshold plus two matching build attestations from builders independent
of each other and of that threshold; snapshot/timestamp delegates cannot add
code. Protocol may
become required only after every Role Domain has qualified current-generation
capacity and drain reserve. Emergency disablement may bypass that capacity gate
only with an honest availability loss and must be ratified into ordinary
metadata before it expires.

Ongoing updates have explicit private-only, direct-allowed, and offline-import
modes with no silent privacy fallback. Once Release Safety has expired or the
build is revoked, V1 does not run an Ardents Service Route to repair itself:
recovery requires a previously configured external privacy proxy, an explicit
direct-disclosure choice, or an offline import. This avoids making an unverified
or revoked networking runtime its own trust-recovery path.
