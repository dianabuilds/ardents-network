# R-156 — Carrier pool progress probe

Question: can opening one unavailable peer prevent access to an already
acquired Carrier for a different peer?

Predeclared hypothesis: the healthy lease remains usable while the unrelated
dial is pending. Falsification: its Carrier() call cannot return until the
controlled dial is released. The 500 ms watchdog is a scheduling allowance,
not a product latency benchmark. Source inspection must corroborate a failure.

Run on a clean temporary extraction of commit
ba5fb8e02e7b9fc45e500872ba20329140d5016b. Copy
pool_progress_test.go.txt to internal/route/r156_pool_progress_test.go there,
then run:
```text
go test ./internal/route -run '^TestR156PoolIndependentProgress$' -count=1 -timeout=30s
```

The probe uses the real exported pool/lease implementation and the existing
deterministic Carrier fake. It creates no sockets, credentials or runtime
state. The extraction, build outputs and raw log belong outside the repository.
The active implementation and its checks are not changed. No new maintained
test profile is created; this is a disposable experiment, not qualification.

Result: the independent-progress hypothesis was falsified; see captured result below. Retain the probe as reproducible research evidence;
any production correction needs a maintained behavioral regression test and
the selected checks. See the R-156 research record for final disposition.


## Captured result (2026-09-14)

On the selected commit with Go 1.26.8 windows/amd64, the probe failed with
exit 1: the existing healthy Carrier became accessible only after releasing
the unrelated blocked dial. Existing TestClosedCarrierPool* checks passed
(exit 0). The watchdog is not a network latency measurement. The shared lock
across open() corroborates the cause. No installed Linux or race result is
claimed. See [R-156](../../docs/research/records/r-156-runtime-architecture-use-review.md)
for the log identity and proposed correction.
