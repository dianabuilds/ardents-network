# R-149 voting-core envelope

Status: predeclared analytical experiment, 2026-09-07. This plan is written
before execution. It belongs to the existing R-149 question; no live network,
human panel, implementation slice or cryptographic primitive is introduced.

Question: can committee size, lifetime selection risk, honest participation and
human workload be treated as independent choices for the proposed voting core?

## Hypotheses and falsification

- H1: under a fixed uniform eligible population with 20% hostile labels, a
  larger jury reduces the chance of exceeding the illustrative fault bound,
  but this is not proof of actual attack resistance. Compare exact tails;
  reject an unconditional claim if the population/selection assumptions fail.
- H2: safety-compatible certificate thresholds alone do not supply turnout.
  Compute response capacity with all faulty members withholding and all
  responding honest members supporting the same outcome. Missing that favorable
  envelope rejects any stronger availability claim for the same assumptions.
- H3: repeated admitted tasks and volunteer filtering can defeat apparently
  small per-draw figures. Show the lifetime and filtered-population results.
- H4: two certificates on one immutable instance may have safe intersection
  while two disjoint replacement juries can certify opposite outcomes without
  any double-voting. This rejects safe replacement inferred from per-jury safety.
- H0: these finite checks select no public admission scheme, protocol or KPI.

## Frozen method and inputs

Python standard library, exact integer combinations/Fraction arithmetic for
single-draw and response distributions. The repeated-event illustration uses
independent draws with the same fixed population; a union bound is also reported
without requiring independence. Neither is a measured attack-success rate.

- N in {100, 1000, 10000}; K=N/5; n in {15, 31, 61}.
- f=floor((n-1)/3); A=R=floor((n+f)/2)+1 for this equal-weight binary example.
- Report P(hostile selected > f), not merely hostile majority.
- For N=1000, report 100 and 1000 admitted-draw lifetime exposure.
- Conditional on exactly f faulty members withholding, each remaining member
  responds independently with probability 1/2, 4/5 or 19/20 and all responders
  agree. Report P(at least A responses); this is not a human-turnout estimate.
- With 200 hostile labels always willing and 800 honest labels willing at
  rates 1, 1/2 or 1/5, recompute the actual eligible fraction and n=31 tail.
- For 100 proposals/day and 1000 willing labels, report expected selections/day
  per label and minutes/day at an assumed 10 minutes per task. Also report 10000
  willing labels. This expectation does not establish workload tails or capacity.
- Enumerate all pairs of threshold-size subsets for n=7,f=2,A=R=5 as an
  independent intersection oracle. Exhibit separate 7-member juries making
  opposing certificates with zero overlapping signers and no equivocation.

For every hypergeometric distribution, verify normalization and expected count
using exact arithmetic; for a small N=8,K=3,n=3 example, cross-check with direct
subset enumeration. Do not reinterpret an assertion pass as protocol safety.

## Run and evidence

From the repository root, run:

```text
python -B experiments/r-149-voting-envelope/analyze.py
```

The program writes JSON to stdout, including its SHA-256 and Python version.
Capture generated output outside the repository. No third-party dependencies
or network are used. The source and this plan are retained research evidence;
numeric findings are recorded after execution, with their limitations.

## Results and disposition

Executed on 2026-09-07 with Python 3.12.12. Analyzer SHA-256:
`b858b9144631783ca9b6e8ae8b0e3c4b868301f4501a49b881770b7c0bfb00c6`.
Normalization/expectation assertions passed; the independent oracle checked
56 sampled subsets and 441 same-jury certificate pairs. The disjoint replacement
juries each produced five opposing signatures with zero equivocators.

For N=1000,K=200, outside-envelope probabilities for n=15,31,61 were respectively
16.266930%, 3.051112% and 0.464383%. Conditional on exactly f faulty members
withholding and independent honest response p=0.8, corresponding response
capacity was 32.212255%, 0.922337% and 0.010634%. These are different conditional
questions, not probabilities that the deployed system succeeds or is attacked.
For n=61, 1000 independent admitted draws have 99.048274% exposure to at least
one outside-envelope draw. At n=31, filtering to 160 honest and 200 hostile
volunteers gives a 99.447698% outside-f10 tail.

H1-H4 have supporting evidence within their frozen models. N=100,n=61 has a
zero outside-f20 tail because the assumed entire population contains only 20
hostile labels; it must not be extrapolated to a larger population or adaptive
corruption. H0 remains: no production mechanism or acceptable-risk target is
selected. Retain the analyzer and plan as reproducible research evidence.
The [assessment](../../docs/research/records/r-149-voting-core-design.md#quantitative-envelope)
records interpretation and the conditional work-package map. Raw generated JSON
was captured outside the repository, at the host temporary evidence path;
rerunning the script reproduces the numeric tables without that local file.
