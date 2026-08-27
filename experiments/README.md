# Experiments

This directory contains disposable spikes for named research questions. It is
not the maintained project source tree. Open implementation-linked spikes are
retained until their real candidate exists; decided H4 spikes in the current
uncommitted change are retained only until their unique evidence enters source
history, after which their maintained owner/tests must supersede them.

All closed Stage 8 spikes were C0 retired on 2026-08-23 after their
decision-relevant results were recorded. The research records, target-module
tests, and independent historical evidence retain their facts; a disposable
implementation is not a second maintained version. R-092/R-098 remain open at
explicit implementation triggers. R-094/R-095/R-096/R-101/R-102 are decided
pre-development evidence, not active implementation or a second specification.

A new spike must begin with a decision-relevant question, an accepted scope
decision where the horizon requires one, and a purpose-named directory. Remove
it in the owning change when its result is absorbed, rejected, or superseded;
retain source only while it has a live measurement or falsification duty.

Create one directory per question:

```text
experiments/
  r-xxx-named-question/
    README.md
    ...disposable code and fixtures...
```

Each retained experiment README must include:

- research question and linked record;
- hypotheses and falsification criteria;
- environment and exact run instructions;
- synthetic or public inputs and sensitive-data handling;
- expected measurements and retained evidence;
- actual result, including negative results;
- limitations and threats to validity;
- disposition: delete, retain as evidence, repeat, or redesign for production.

Experiments must not introduce project APIs, a nested Go module, deployment
promises, or an implicit production subsystem. Generated dependencies, packet
captures, databases, credentials, and build caches must not be committed.
