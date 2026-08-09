# Experiments

This directory is reserved for disposable spikes written to answer named
research questions. It is not the maintained project source tree.

The current maintained slice is Carrier Lab. Its implementation lives in the
single root Go project; this directory may hold only separately authorized,
disposable comparison spikes or fixtures. Naming, public bootstrap, Bridges,
updater/governance, Windows, SDK/browser work, and complete public qualification
must not be added by implication.

[R-013](../docs/research/records/r-013-carrier-lab-technology-candidates.md)
freezes Gate B. ADR-0009 separately establishes the project foundation without
promoting any networking claim or later delivery horizon.

Create one directory per question:

```text
experiments/
  r-xxx-named-question/
    README.md
    ...disposable code and fixtures...
```

Each experiment README must include:

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
