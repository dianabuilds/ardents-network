"""Exact finite committee arithmetic, not a consensus or cryptography implementation."""

from fractions import Fraction
from hashlib import sha256
from itertools import combinations
from math import comb, expm1, log1p
from pathlib import Path
import json
import platform


def distribution(population, hostile, size):
    denominator = comb(population, size)
    probabilities = {
        count: Fraction(
            comb(hostile, count) * comb(population - hostile, size - count),
            denominator,
        )
        for count in range(max(0, size - (population - hostile)), min(hostile, size) + 1)
    }
    assert sum(probabilities.values()) == 1
    assert sum(count * p for count, p in probabilities.items()) == Fraction(
        size * hostile, population
    )
    return probabilities


def tail(population, hostile, size, threshold):
    return sum(
        (p for count, p in distribution(population, hostile, size).items()
         if count >= threshold), Fraction(0)
    )


def report(probability):
    return {"exact": str(probability), "percent": float(probability) * 100}


def sampling_rows():
    rows = []
    for hostile in (10, 20, 30):
        for size in (9, 15, 21):
            fault_bound = (size - 1) // 3
            rows.append({
                "population": 100,
                "hostile": hostile,
                "committee_size": size,
                "assumed_fault_bound": fault_bound,
                "strict_majority": report(tail(100, hostile, size, size // 2 + 1)),
                "outside_fault_bound": report(tail(100, hostile, size, fault_bound + 1)),
            })
    return rows


def tally(yes, no, minimum_turnout=0):
    turnout = yes + no
    return turnout >= minimum_turnout and 2 * yes > turnout


def vote_counterexamples():
    assert tally(2, 1)
    assert tally(6, 4, 10)
    yes = set(range(7))
    no = set(range(7, 15))
    reader_a = yes | {7, 8, 9}
    reader_b = {0, 1} | no
    assert tally(len(reader_a & yes), len(reader_a & no), 10)
    assert tally(len(reader_b & no), len(reader_b & yes), 10)
    assert not yes & no
    return {
        "missing_dropped": {"selected": 15, "yes": 2, "no": 1, "missing": 12,
                            "responder_rule_passes": True},
        "turnout_only": {"selected": 15, "hostile_yes": 6, "honest_no": 4,
                         "missing_honest": 5, "minimum_turnout": 10,
                         "responder_rule_passes": True},
        "two_local_closes": {"reader_a": {"yes": 7, "no": 3},
                             "reader_b": {"yes": 2, "no": 8},
                             "double_voters": 0,
                             "contradictory_responder_majorities": True},
    }


def fixed_threshold():
    size, fault_bound, threshold = 15, 4, 10
    minimum_intersection = 2 * threshold - size
    assert minimum_intersection > fault_bound
    assert threshold <= size - fault_bound
    equivocators = set(range(5))
    certificate_a = equivocators | set(range(5, 10))
    certificate_b = equivocators | set(range(10, 15))
    assert len(certificate_a) == len(certificate_b) == threshold
    assert certificate_a & certificate_b == equivocators
    assert size - fault_bound - 2 < threshold
    return {
        "selected": size, "assumed_fault_bound": fault_bound,
        "required_matching_signatures": threshold,
        "minimum_intersection": minimum_intersection,
        "two_certificates_with_five_equivocators": True,
        "all_four_faulty_withhold_plus_two_honest_unavailable": "insufficient",
        "guaranteed_consensus_or_availability": False,
    }


def repeated_and_disjoint():
    probability = tail(100, 20, 15, 8)
    both_disjoint = sum(
        (p * tail(85, 20 - count, 15, 8)
         for count, p in distribution(100, 20, 15).items() if count >= 8),
        Fraction(0),
    )
    independent_square = probability ** 2
    assert both_disjoint != independent_square
    assert both_disjoint < probability
    return {
        "single_majority": report(probability),
        "independent_draws_at_least_one_majority_percent": {
            str(draws): -expm1(draws * log1p(-float(probability))) * 100
            for draws in (1, 100, 500, 1000)
        },
        "two_disjoint_both_majorities": report(both_disjoint),
        "two_independent_both_majorities": report(independent_square),
    }


def independent_oracle():
    samples = list(combinations(range(8), 3))
    observed = {count: Fraction(0) for count in range(4)}
    for sample in samples:
        observed[sum(member < 3 for member in sample)] += Fraction(1, len(samples))
    assert observed == distribution(8, 3, 3)
    # Joint disjoint-draw oracle: 8 labels, 3 hostile, two committees of size 2.
    pairs = [(a, b) for a in combinations(range(8), 2)
             for b in combinations(set(range(8)) - set(a), 2)]
    observed_joint = Fraction(sum(
        any(member < 3 for member in a) and any(member < 3 for member in b)
        for a, b in pairs
    ), len(pairs))
    formula_joint = sum((p * tail(6, 3 - count, 2, 1)
                         for count, p in distribution(8, 3, 2).items() if count >= 1),
                        Fraction(0))
    assert observed_joint == formula_joint
    return {"single_subsets": len(samples), "ordered_disjoint_pairs": len(pairs)}


def opposite_certificates():
    rows = []
    for n, accept, reject, faults in ((15, 10, 5, 4), (15, 10, 6, 4),
                                      (15, 10, 10, 4), (21, 14, 7, 6),
                                      (21, 14, 14, 6)):
        intersection = max(0, accept + reject - n)
        rows.append({"selected": n, "accept": accept, "reject": reject,
                     "assumed_fault_bound": faults,
                     "minimum_opposite_intersection": intersection,
                     "correct_intersection_required_and_present": intersection > faults})
    yes = set(range(10))
    no = set(range(10, 15))
    assert len(yes) == 10 and len(no) == 5 and not yes & no
    one_equivocating_no = {0} | no
    assert len(one_equivocating_no) == 6 and yes & one_equivocating_no == {0}
    pair_count = 0
    for a in range(1, 7):
        for r in range(1, 7):
            intersections = []
            for left in combinations(range(6), a):
                for right in combinations(range(6), r):
                    intersections.append(len(set(left) & set(right)))
                    pair_count += 1
            assert min(intersections) == max(0, a + r - 6)
    return {"rows": rows, "all_honest_10_yes_5_no_certifies_both": True,
            "one_equivocator_10_yes_6_no_certifies_both": True,
            "small_oracle_pairs": pair_count}


def binomial_tail(count, response_probability, threshold):
    probabilities = {k: comb(count, k) * response_probability ** k
                      * (1 - response_probability) ** (count - k)
                     for k in range(count + 1)}
    assert sum(probabilities.values()) == 1
    assert sum(k * p for k, p in probabilities.items()) == count * response_probability
    return sum((p for k, p in probabilities.items() if k >= threshold), Fraction(0))


def willingness():
    rows = []
    for p in (Fraction(1, 5), Fraction(1, 2), Fraction(4, 5), Fraction(19, 20)):
        rows.append({"response_probability": str(p),
                     "all_15_honest_capacity_at_least_10": report(binomial_tail(15, p, 10)),
                     "four_withhold_11_honest_capacity_at_least_10": report(binomial_tail(11, p, 10))})
    assert binomial_tail(4, Fraction(1, 2), 3) == Fraction(
        sum(mask.bit_count() >= 3 for mask in range(16)), 16)
    return {"response_rows": rows,
            "volunteer_pool": [{"hostile_volunteers": 20, "honest_volunteers": honest,
                                "hostile_fraction": report(Fraction(20, 20 + honest))}
                               for honest in (80, 40, 16)],
            "empirical_turnout_measurement": False,
            "response_oracle_patterns": 16}


def availability_filter():
    hostile = {0, 1, 2} | set(range(15, 20))
    original = set(range(15))
    suppressed = set(range(3, 8))
    final = (original - suppressed) | set(range(15, 20))
    assert len(final) == 15 and len(final & hostile) == 8
    assert len(original & hostile) == 3
    return {"candidates": {"total": 40, "hostile": 8},
            "confirmed": {"total": 16, "hostile": 8},
            "final_15_hostile_majority": report(tail(16, 8, 15, 8)),
            "final_15_outside_f4": report(tail(16, 8, 15, 5)),
            "fixed_alternates": {"original_hostile": 3, "blocked_honest": 5,
                                 "replacement_hostile": 5, "final_hostile": 8,
                                 "final_size": 15, "rerolls": 0}}


def main():
    output = {
        "model": "fixed uniform labels; no implemented protocol",
        "python": platform.python_version(),
        "platform": platform.platform(),
        "source_sha256": sha256(Path(__file__).read_bytes()).hexdigest(),
        "oracle": independent_oracle(),
        "sampling": sampling_rows(),
        "tallies": vote_counterexamples(),
        "fixed_threshold": fixed_threshold(),
        "opposite_certificates": opposite_certificates(),
        "willingness": willingness(),
        "availability_filter": availability_filter(),
        "redraws_and_second_committee": repeated_and_disjoint(),
    }
    print(json.dumps(output, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
