# ADR-0134: Use one Application byte stream

Status: Accepted
Date: 2026-11-27

## Context

The Product Owner requires one byte stream for every file, including text.
The document exchange and the second Application profile introduced duplicate
contracts without a product requirement. There are no deployed clients or
backward compatibility obligations for these profiles.

## Decision

Supersede ADR-0081 and ADR-0132 wherever they select or preserve a separate
document Application protocol. Use one bidirectional ordered byte stream for
all content. File publication sends immutable file bytes and directional EOF.
The receiver closes its unused sending direction and receives bytes until EOF,
then checks authenticated terminal completion. No text magic, request record,
response status, length header or UTF-8 restriction belongs in this exchange.

Use the existing AAI4 local byte-stream framing and the byte-stream authenticated
Service context for every ordinary Application connection. Local snapshot and
relay workers may have different local responsibilities, but content type cannot
select a local Connection grammar or an authenticated Service profile.

## Consequences

File snapshots accept arbitrary bytes within the existing 4 MiB bound.
Terminal text presentation retains its local UTF-8 and control-character checks.
Authority, confinement, cancellation, resource bounds and joined cleanup remain
required. Changed artifacts require new qualification evidence. Historical
receipts describe only their original artifacts.
