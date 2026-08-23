# Experiments

This directory contains only the active disposable spike for a named research
question. It is not the maintained project source tree.

All closed Stage 8 spikes were C0 retired on 2026-08-23 after their
decision-relevant results were recorded. The research records, target-module
tests, and independent historical evidence retain their facts; a disposable
implementation is not a second maintained version. The remaining
`r-092-native-node-profile` baseline is active only until its reference-host
campaign selects or rejects a native Node profile.

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

Each active experiment README must include:

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
