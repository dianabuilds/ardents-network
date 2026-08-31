# Service command end to end

These tests build the maintained Service and Application commands and exercise
their public process and Unix-socket boundaries. The ordinary checked profile
covers:

- readiness timeout and cleanup for the Service command;
- authenticated Publisher acquisition of one local Application attachment;
- at-most-once headless Service Instance acquisition across processes;
- a Publisher Application-to-Service stream through the maintained product
  commands and their selected State assignments;
- the interactive local Application journey; and
- bounded terminal outcomes when a current Route fails or is withdrawn.

The tests may replace an external Route socket with a purpose-specific test
adapter, but the Service, publication, Application IPC, continuity, and workload
processes under assertion are real commands. Fixtures never receive naming,
State, Target, Route, or release authority merely because they supply bounded
Application bytes.

The former tagged Reference C-2 topology, recovery fixture, exact-candidate
Carrier seam, stage-specific Browser scenarios, and their qualification runners
were retired from the C0 candidate. Their last source snapshot is immutable at
[`fbb42034757513ac009114a00b933aefa76d8ddf`](https://github.com/dianabuilds/ardents-network/commit/fbb42034757513ac009114a00b933aefa76d8ddf).
That historical source is provenance only and is not part of the current test
denominator.
