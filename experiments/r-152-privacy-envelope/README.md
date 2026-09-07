# Privacy cost envelope — R-152

Question: can a symmetric, continuously traffic-independent client schedule
fit the selected idle cap, response delay and existing sustained-stream floor?

This is a disposable deterministic arithmetic model. It implements neither
Ardents nor a cryptographic protocol and produces no anonymity qualification.

**Disposition after the Product Owner's correction, 2026-09-07:** retained
exploratory evidence only. Full-capacity filler is excluded from the proposed
design direction. This model does not estimate ordinary useful demand,
necessary added protection traffic or a hosting bill. Current design compares
the same useful workload under realistic low-overhead mechanisms in
[R-152](../../docs/research/records/r-152-common-privacy-design.md#cost-model-for-the-ordinary-workload).

## Hypotheses and falsification fixed before execution

- A constant rate that also carries useful traffic cannot offer a useful
  one-direction rate greater than its total charged rate in that direction.
  For equal send/receive rates, the ideal daily cost is twice that rate times
  86,400 seconds. Reject joint idle/goodput feasibility when these bounds cross.
- For an unchanged Poisson emission schedule with mean rate lambda and an
  independently arriving request, the first available slot has p95 waiting
  time log(20)/lambda. If this optimistic pre-network wait already exceeds
  the admitted response bound, reject that schedule/packet cell.
- An Erlang delay calculation is a comparison for independent exponential
  holds. Passing its delay arithmetic proves neither mixing anonymity nor
  a working network. Do not sum individual p95 values as a total p95.

## Inputs and scope

Product Owner selected on 2026-09-07: p95 first useful response <=3 seconds
cold and <=1 second warm from an already working client; <=1 GB (decimal
1,000,000,000 bytes) aggregate send/receive in 24 hours without useful exchange.
Existing normal sustained goodput is min(10 Mbit/s, 50% of the paired direct
baseline), separately by direction. The model includes 10 Mbit/s as the
full-floor reference case, not a claim that every reference pair reaches it.

Fixed grid: packet bytes 512, 1024, 1280, 2048, 4096, 8192, 16384, 32768;
reserved other idle traffic 0% and 20%; equal send/receive accounting. Packet
sizes are algebraic inputs, not an accepted wire format. Zero framing,
cryptographic and network overhead makes these optimistic bounds. The separate
9-hold Erlang comparison uses mean holds 0.01, 0.02, 0.05, 0.1 and 0.2 seconds.
No change to a queue, rate or delay is made after results to produce a pass.

The full-rate padding load is an analyst-chosen hypothesis: dummy traffic fills
unused capacity continuously. The 10 Mbit/s useful-capacity reference is not
normal Application demand or an accepted idle-rate requirement. The Product
Owner's 1 GB idle allowance is a ceiling, not a target; useful exchange can
make an active day's total traffic greater than 1 GB without violating that
idle-only requirement. Active overhead/resource obligations still apply.

A client can send faster during useful work only by leaving the unchanged-rate
model. This result does not rule out bounded background exchange, finite padded
transfer windows or demand-driven transmission with different claims. The
first-slot results similarly apply only to requests forced to await unchanged
Poisson slots, not every packet protocol of those sizes.
A Poisson process has an unbounded tail: mean daily traffic below a hard daily
cap does not prove that every day meets that cap.

## Run

Use Python 3 with the standard library:

```powershell
python experiments/r-152-privacy-envelope/envelope.py --output "$env:TEMP/ardents-privacy-envelope"
```

The output directory must resolve outside the repository. The command writes
a JSON receipt and CSV grid there and prints a compact result. It records
script identity, exact inputs, Python version and calculations. No fetched code,
credentials, traffic captures, dependency cache or generated data enters Git.

## Captured evidence and result

Executed on 2026-09-07 with Python 3.12.12. Receipt and CSV are retained outside
Git at:

C:/Users/vitek/AppData/Local/Temp/ardents-privacy-design-f4ad4e86eb4e4323ae437b6bd8f20b59/envelope/

Script SHA-256:
dc48b2cafaa345863b8116e5565f32ffd35029e2e4c569d28a5528a8bbddeef1.
JSON receipt SHA-256:
74ce2f38cafbee9c6ee306b034d0e776fa6295eefbba8aa723891762d80be139.

The internal arithmetic consistency checks passed. The symmetric unchanged
full-floor rate costs 216 GB/day before overhead, whereas the 1 GB cap permits
only 46.3 kbit/s per direction. An unchanged symmetric 2048-byte Poisson
schedule already waits 1.060 seconds at p95 before network or processing.
These particular model cells fail their combined bounds. Smaller cells and
periodic schedules are not qualified by passing this arithmetic.

[R-152](../../docs/research/records/r-152-common-privacy-design.md#arithmetic-findings)
records the conclusions, competing designs and honest scope. Rerunning this
small model regenerates the evidence if the external temporary directory is
lost; the retained source and input grid are sufficient for reproduction.

## Disposition

Retain the small model as reproducible design evidence. It is excluded from
maintained Go, product execution and qualification. Do not turn model rows into
implementation tasks or public anonymity claims.
