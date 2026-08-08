# Experiments

This directory contains disposable code written to answer named research
questions. It is not the product source tree.

The only current multi-component experiment authorized by
[product scope](../docs/product/scope.md) is Carrier Lab. Its directory does not
exist until Gate B freezes the experiment record. Naming, public bootstrap,
Bridges, updater/governance, Windows, SDK/browser work, and complete public
qualification must not be added to Carrier Lab.

Create one directory per question:

```text
experiments/
  r-004-interactive-routing/
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

Experiments must not introduce shared production APIs, deployment promises, or
an implicit choice of language. Generated dependencies, packet captures,
databases, credentials, and build caches must not be committed.
