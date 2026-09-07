# R-152 — Layered TLS byte cost

Status: disposable component experiment; predeclared 2026-09-07.

## Question and hypothesis

Can the maintained Go TLS implementation carry layered confidential channels
with a small record-byte addition, making it worth selecting for the common
split-circuit design? This does not test the complete privacy scheme.

H1: with four nested TLS 1.3 channels, the record-byte addition for a 512-byte
request and 64 KiB response is at most 15% of the useful bytes in each observed
exchange, and aggregate initial handshake bytes are at most 64 KiB.
These are analyst-selected screening limits that reserve room for other work;
they do not replace NET-14 or NET-32/33.

Reject H1 if any sampled cell exceeds either limit, loses/corrupts bytes,
fails authentication or cannot finish in its bounded local deadline.
A successful result justifies only further composition of this implementation.

## Profile and mechanics

- Windows amd64, locally installed Go 1.26.6, standard library only.
- One through four nested TLS 1.3 client/server pairs over memory net.Pipe.
  Each layer uses a separately generated pinned Ed25519 server identity;
  session tickets are disabled and key-exchange groups use the Go defaults.
- Five fresh stacks per depth; five sequential 512-byte request / 65536-byte
  response exchanges in each stack. Every received byte is checked.
- Counters record successfully written outermost TLS bytes separately for
  client transmission and server transmission. No network socket is opened.
- Cold component cost is all handshakes plus the first exchange. Subsequent
  exchanges reuse open channels; they are **not** the product's warm-access
  case, which requires opening a new Service Connection.
- Certificate generation and process setup are outside the counters.

The four-layer shape represents the possible client access-edge contribution
of an adjacent Carrier, two protected forwarding channels and the Service TLS
channel. Both logical ends are inside one process. It is not an actual chain
of relays or an executable Ardents protocol.

## Reproduction

From the repository with the installed pinned Go toolchain:

~~~powershell
$probeEvidence = Join-Path $env:TEMP ('ardents-tls-cost-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $probeEvidence | Out-Null
go run ./experiments/r-152-layered-tls/tls_cost.go |
    Set-Content -LiteralPath (Join-Path $probeEvidence 'tls-bytes.json') -Encoding utf8
~~~

Use an external Go build cache if one is not already configured. The actual
run record also captures source/output hashes and toolchain identity there.
No dependency installation, public traffic or maintained package is involved.

## Interpretation limits

The counters omit IP/TCP/QUIC headers and acknowledgements, Route/admission/
Name/Descriptor/publication records, control padding, retry, shutdown,
background refresh, relay receive-plus-forward totals, scheduling and quotas.
There is no latency, throughput, loss, CPU, memory, platform isolation,
traffic-analysis or anonymity verdict. Memory pipes cannot supply those facts.
TLS library defaults are experiment inputs, not a selected public cipher suite.

## Captured evidence, result and disposition

Measured on 2026-09-07 with Go 1.26.6, Windows amd64, CGO disabled.
The final source completed 20 fresh stacks and 100 byte-verified exchanges.

| TLS depth | Aggregate initial handshake bytes | Exchange bytes, observed min–max | Maximum record addition to 66,048 useful bytes |
|---|---:|---:|---:|
| 1 | 3,171 | 66,158–66,312 | 0.40% |
| 2 | 6,452 | 66,356–66,576 | 0.80% |
| 3 | 9,843 | 66,576–66,840 | 1.20% |
| 4 | 13,344 | 66,840–67,104 | 1.60% |

All observed cells met H1's component screening thresholds. At depth four
the largest exchange adds 1,056 bytes, separately from its 13,344 handshake
bytes. A first component transaction can therefore cost 80,448 bytes for
66,048 useful bytes; the small exchange percentage must not hide setup.

This supports further TLS composition, not a complete Ardents traffic or
performance forecast. None of the omitted costs or protection claims has
been measured here.

External evidence:
C:/Users/vitek/AppData/Local/Temp/ardents-tls-cost-bd7c5c267c9b4b8588e9675575efd7e0/

- Final source SHA-256:
  1f79ff0bc5933d7bd0814d6f18b8cbb2f4ecfe16aeda5f93a5b487288a446941
- Final output tls-bytes-final.json SHA-256:
  e66c1a00cbc6caf5f076a1d8960745bc73871f00a863d501bc78a2f52aedce69
- receipt-final.json records the toolchain, hashes and source-change reason;
  summary-final.json recomputes count and screening coverage.

The initial component run also completed, but make quick-check rejected the
experiment because it lacked the required build-ignore constraint and appeared
to be a maintained root-module package. Adding the required constraint fixed
that placement error without weakening the gate or changing measurement
behavior. The final source was rerun. Initial source, output and receipt are
retained in the same evidence directory; the earlier rejection is not a pass.
The corrected tree subsequently passed make quick-check, including architecture,
format, vet, deterministic tests, build, module tidiness and artifact representation.

Disposition: retain the build-ignored, standard-library source as reproducible
component evidence. Never copy it into the maintained tree as a Route
implementation. Loss of the temporary directory requires reproducing the run,
not treating this table as an immutable raw capture.
