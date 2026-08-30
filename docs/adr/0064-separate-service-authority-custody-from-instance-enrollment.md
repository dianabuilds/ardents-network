---
status: accepted
date: 2026-08-30
extends: 0003-bounded-service-instance-credentials.md; 0021-use-password-derived-authority-custody.md
---

# ADR-0064 — Separate Service Authority custody from host Instance enrollment

## Context

The supported headless Publisher can publish only a prepared public Service
Instance Credential and matching private Instance Key. Generating those inputs
in a fixture, accepting raw key files, or placing Service Authority in
enrollment leaves the normal participant journey without an honest owner and
collapses Authority Custody into release distribution or runtime composition.

## Decision

A separate local Authority Custody operation alone creates or imports an
encrypted Service Authority and issues a monotonically higher, bounded public
Credential for one canonical host-generated Instance enrollment request. The
request contains only the Instance signing public key, separate Introduction
recipient public key, Network ID, requested validity bound, and a fresh request
commitment. Custody derives the Service Target from its authenticated Authority,
advances the Authority-owned generation watermark durably before returning the
Credential, and never releases root material or accepts a secret through argv,
environment, configuration, or shared stdin.

The Publisher host owns one owner-only Instance enrollment root. It generates
and retains the Instance and Introduction private keys, emits only the canonical
public request, accepts only the exact matching Credential response at most
once, and exposes only an opened non-exporting binding to Service publication.
The maintained headless runtime receives no Credential file, Instance-key file,
raw Target, or Service Authority.

Withdrawal prevents new publication acquisition, drains retained work, erases
the live binding, and terminally closes that Instance generation. Restart never
revives a generation already committed to the publication floor: routine
recovery creates a fresh host key and obtains a higher Credential. Ambiguous or
malformed durable state fails unavailable rather than resetting a generation.

Authority Recovery Bundles remain supported only under the existing custody
contract. A restored Service Authority stays authority-locked and cannot issue
until a future accepted authenticated currentness witness exists. Loss of the
active Authority record therefore loses the Target in this usable-alpha
profile.

The headless artifact lane authenticates the real custody and Publisher-host
commands but carries no Authority, Target, Credential, generation choice,
Instance secret, or reusable issuance power. Browser, Node, Network State,
Route, release metadata, and ordinary Application Interfaces never receive
Service Authority.

## Consequences

- The owner performs a deliberate request/response custody ceremony for each
  Publisher generation; routine restart cannot silently reuse an old key.
- “Non-exporting” means no supported Interface returns the private Instance
  keys. Filesystem or administrator compromise remains Endpoint compromise;
  this decision adds no hardware-keystore, snapshot-resistance, or
  post-compromise-healing claim.
- Service Credential v2, Service Target derivation, Route, Transit Grant v1,
  and public network wire semantics remain unchanged.
- The new host Instance Module and existing custody Module are separate deep
  owners; no generic signer Interface, online Authority service, enrollment
  registrar, or second publication implementation is introduced.

## Compliance

[R-131](../research/records/r-131-headless-publisher-authority-acquisition.md)
records the accepted option and rejected alternatives. ADR-0003 continues to
own the bounded Service Instance Credential hierarchy, and ADR-0021 continues
to own password-derived encrypted Authority custody.
