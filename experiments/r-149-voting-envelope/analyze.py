"""Finite voting-core arithmetic; no consensus, cryptography or human simulation."""

from collections import Counter
from fractions import Fraction
from hashlib import sha256
from itertools import combinations
import json
from math import comb
from pathlib import Path
import platform


def probability(value):
    return {"fraction": str(value), "percent": float(value * 100)}


def distribution(population, hostile, jury):
    denominator = comb(population, jury)
    values = {
        k: Fraction(comb(hostile, k) * comb(population - hostile, jury - k),
                    denominator)
        for k in range(max(0, jury - population + hostile),
                       min(hostile, jury) + 1)
    }
    assert sum(values.values()) == 1
    assert sum(k * p for k, p in values.items()) == Fraction(jury * hostile, population)
    return values


def outside(population, hostile, jury, faults):
    return sum((p for k, p in distribution(population, hostile, jury).items()
                if k > faults), Fraction())


def envelope(jury):
    faults = (jury - 1) // 3
    threshold = (jury + faults) // 2 + 1
    assert 2 * threshold > jury + faults
    assert threshold <= jury - faults
    return faults, threshold


def response_capacity(jury, faults, threshold, response):
    honest = jury - faults
    return sum((Fraction(comb(honest, k)) * response ** k *
                (1 - response) ** (honest - k)
                for k in range(threshold, honest + 1)), Fraction())


def independent_oracles():
    counts = Counter(sum(i < 3 for i in sample)
                     for sample in combinations(range(8), 3))
    assert {k: Fraction(v, comb(8, 3)) for k, v in counts.items()} == distribution(8, 3, 3)
    sets = [set(group) for group in combinations(range(7), 5)]
    intersections = [len(a & b) for a in sets for b in sets]
    assert min(intersections) == 3
    assert all(value > 2 for value in intersections)
    first_jury, second_jury = set(range(7)), set(range(7, 14))
    yes, no = set(range(5)), set(range(7, 12))
    assert yes <= first_jury and no <= second_jury
    assert len(yes) == len(no) == 5 and not yes & no
    return {"enumerated_samples": comb(8, 3),
            "same_jury_certificate_pairs": len(intersections),
            "same_jury_minimum_intersection": min(intersections),
            "replacement_counterexample": {"yes": sorted(yes), "no": sorted(no),
                                           "equivocators": 0,
                                           "safe_cross_attempt_close_assumed": False}}


def main():
    sampling, lifetime, participation, workload, volunteers = [], [], [], [], []
    for jury in (15, 31, 61):
        faults, threshold = envelope(jury)
        for population in (100, 1000, 10000):
            tail = outside(population, population // 5, jury, faults)
            sampling.append({"N": population, "K": population // 5, "n": jury,
                             "f": faults, "A": threshold, "R": threshold,
                             "outside_fault_envelope": probability(tail)})
            if population == 1000:
                for attempts in (100, 1000):
                    lifetime.append({"n": jury, "attempts": attempts,
                                     "independent_draw_percent":
                                     100 * (1 - (1 - float(tail)) ** attempts),
                                     "union_bound_percent":
                                     100 * min(1, attempts * float(tail))})
        for rate in (Fraction(1, 2), Fraction(4, 5), Fraction(19, 20)):
            participation.append({"n": jury, "f_withholding": faults,
                                  "A": threshold, "response_rate": str(rate),
                                  "enough_agreeing_honest_replies": probability(
                                      response_capacity(jury, faults, threshold, rate))})
        for willing in (1000, 10000):
            load = Fraction(100 * jury, willing)
            workload.append({"n": jury, "willing": willing, "proposals_per_day": 100,
                             "mean_tasks_per_day": str(load),
                             "assumed_minutes_per_task": 10,
                             "mean_minutes_per_day": str(load * 10)})
    for honest_willing in (800, 400, 160):
        population = 200 + honest_willing
        volunteers.append({"hostile_willing": 200, "honest_willing": honest_willing,
                           "hostile_fraction": probability(Fraction(200, population)),
                           "n31_outside_f10": probability(outside(population, 200, 31, 10))})
    print(json.dumps({"question": "R-149", "model": "conditional analytical envelope",
                      "python": platform.python_version(),
                      "source_sha256": sha256(Path(__file__).read_bytes()).hexdigest(),
                      "oracle": independent_oracles(), "sampling": sampling,
                      "lifetime": lifetime, "participation": participation,
                      "volunteers": volunteers, "workload": workload,
                      "implemented_protocol": False, "empirical_humans": False},
                     indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
