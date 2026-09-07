"""Finite R-149 semantic counterexamples, not an Ardents/SCP implementation."""
import hashlib
import itertools
import json
from pathlib import Path
import platform

ENUMERATED_SUBSETS = 0


def subsets(items, minimum=1):
    items = sorted(items)
    return [frozenset(c) for k in range(minimum, len(items)+1)
            for c in itertools.combinations(items, k)]


def threshold_slices(nodes, threshold):
    choices = [s for s in subsets(nodes) if len(s) == threshold]
    return {n: [s for s in choices if n in s] for n in nodes}


def is_quorum(group, slices):
    return bool(group) and all(any(s <= group for s in slices[n]) for n in group)


def quorums(slices):
    global ENUMERATED_SUBSETS
    choices = subsets(slices)
    ENUMERATED_SUBSETS += len(choices)
    return [s for s in choices if is_quorum(s, slices)]


def disjoint_pair(groups):
    for left, right in itertools.combinations(groups, 2):
        if left.isdisjoint(right):
            return [sorted(left), sorted(right)]
    return None


def delete_faults(slices, faulty):
    return {n: [s-faulty for s in choices]
            for n, choices in slices.items() if n not in faulty}


def quorum_report(slices):
    nodes = frozenset(slices)
    groups = quorums(slices)
    assert groups
    pair = disjoint_pair(groups)
    fault_failures = []
    full_availability = 0
    partial_availability = 0
    for n in sorted(nodes):
        faulty = frozenset({n})
        reduced = delete_faults(slices, faulty)
        reduced_groups = quorums(reduced)
        counterexample = disjoint_pair(reduced_groups)
        if counterexample:
            left, right = map(frozenset, counterexample)
            assert is_quorum(left, reduced) and is_quorum(right, reduced)
            assert left.isdisjoint(right)
            fault_failures.append({"faulty": n, "reduced_quorums": counterexample})
        remaining = nodes-faulty
        full_availability += is_quorum(remaining, slices)
        partial_availability += any(q <= remaining for q in groups)
    blocking = []
    for removed in subsets(nodes):
        if not any(q.isdisjoint(removed) for q in groups):
            blocking.append(removed)
    min_block = min(map(len, blocking))
    common = set.intersection(*(set(q) for q in groups))
    return {
        "nodes": sorted(nodes),
        "slices": {n: [sorted(s) for s in slices[n]] for n in sorted(nodes)},
        "quorum_count": len(groups), "smallest_quorum": min(map(len, groups)),
        "ordinary_intersection": pair is None, "ordinary_counterexample": pair,
        "single_byzantine_cases": len(nodes),
        "single_byzantine_intersection_failures": fault_failures,
        "all_remaining_form_original_quorum_after_one_crash_count": full_availability,
        "some_original_quorum_survives_one_crash_count": partial_availability,
        "common_to_every_quorum": sorted(common),
        "minimum_crashes_to_remove_every_original_quorum": min_block,
        "example_minimum_crash_set": sorted(next(b for b in blocking if len(b)==min_block)),
    }


def analyzed_configurations():
    four = threshold_slices("abcd", 3)
    five = threshold_slices("abcde", 3)
    # Independent oracle: uniform t-of-n quorums are exactly sets of size >=t.
    for slices, threshold in ((four, 3), (five, 3)):
        actual = set(quorums(slices))
        expected = {s for s in subsets(slices) if len(s) >= threshold}
        assert actual == expected
    split = threshold_slices("abc", 2) | threshold_slices("def", 2)
    anchor = {"r": [frozenset({"r"})]}
    anchor.update({n: [frozenset({n, "r"})] for n in "abcd"})
    newer = threshold_slices("aefg", 3)
    aggregate = {}
    for cfg in (four, newer):
        for n, choices in cfg.items():
            aggregate.setdefault(n, []).extend(choices)
    configurations = {
        "uniform_3_of_4": four,
        "uniform_3_of_5": five,
        "two_disjoint_2_of_3_groups": split,
        "mandatory_anchor": anchor,
        "old_new_aggregate_for_one_slot": aggregate,
    }
    result = {name: quorum_report(cfg) for name, cfg in configurations.items()}
    strong = result["uniform_3_of_4"]
    assert strong["ordinary_intersection"]
    assert not strong["single_byzantine_intersection_failures"]
    assert strong["all_remaining_form_original_quorum_after_one_crash_count"] == 4
    assert strong["common_to_every_quorum"] == []
    weak = result["uniform_3_of_5"]
    assert weak["ordinary_intersection"]
    assert len(weak["single_byzantine_intersection_failures"]) == 5
    assert result["mandatory_anchor"]["common_to_every_quorum"] == ["r"]
    assert not result["two_disjoint_2_of_3_groups"]["ordinary_intersection"]
    assert disjoint_pair(quorums(newer)) is None
    assert not result["old_new_aggregate_for_one_slot"]["ordinary_intersection"]
    return result


def independent_bindings():
    base = {"alpha": (7, "target-old-a"), "beta": (4, "target-old-b")}
    changes = [("alpha", 7, 8, "target-new-a"), ("beta", 4, 5, "target-new-b")]
    results = []
    for order in itertools.permutations(changes):
        state = dict(base)
        for name, prev, revision, target in order:
            assert state[name][0] == prev
            state[name] = (revision, target)
        results.append(state)
    assert results[0] == results[1]
    return {"orders_checked": 2, "equal_result": True,
            "condition": "distinct objects, unchanged authority/lineage, no shared quota",
            "result": results[0]}


def same_predecessor():
    updates = [("target-a", 7, 8), ("target-b", 7, 8)]
    assert all(prev == 7 for _, prev, _ in updates)
    outcomes = []
    for order in itertools.permutations(updates):
        revision, current = 7, "target-old"
        accepted, rejected = [], []
        for target, prev, successor in order:
            if prev == revision:
                revision, current = successor, target
                accepted.append(target)
            else:
                rejected.append(target)
        outcomes.append({"arrival": [x[0] for x in order], "current": current,
                         "accepted": accepted, "rejected": rejected})
    assert outcomes[0]["current"] != outcomes[1]["current"]
    return {"each_admissible_against_same_predecessor": True,
            "local_arrival_is_common_choice": False, "orders": outcomes}


def immutable_facts():
    deliveries = ("owner-a:r1", "owner-b:r1", "owner-c:r1", "owner-a:r1")
    orders = list(itertools.permutations(deliveries))
    results = [frozenset(order) for order in orders]
    assert all(r == results[0] for r in results)
    return {"delivery_permutations": len(orders),
            "distinct_delivery_sequences": len(set(orders)),
            "distinct_accumulated_facts": len(results[0]),
            "converges_as_fact_set": True, "proves_current_view": False}


def claim_counterexample():
    # Symbolic eligibility/rank assumption. No actual ordinal proof is fabricated.
    claims = [(9, "authority-a"), (4, "authority-b")]
    local_first = min(claims[:1])
    after_delivery = min(claims)
    assert local_first != after_delivery
    local_claim_sets = [{claims[0]}, {claims[1]}]
    assert all(len(s) == 1 for s in local_claim_sets)
    assert len(set.union(*local_claim_sets)) == 2
    return {"first_observed": local_first, "after_withheld_earlier_input": after_delivery,
            "union_has_two_claimants": True,
            "unqualified_finalization_reverses_answer": True,
            "safe_missing_evidence_outcome": "pending/unavailable, not finalized"}


def lineage_counterexample():
    child = {"parent_generation": 3, "target": "child-target"}
    def resolve(parent_generation, released):
        return child["target"] if (
            child["parent_generation"] == parent_generation and not released) else None
    before = resolve(3, False)
    after_release = resolve(3, True)
    after_reclaim = resolve(4, False)
    assert before is not None and after_release is None and after_reclaim is None
    # Storage may retain the child; current derivation still must recheck lineage.
    return {"before": before, "after_parent_release": after_release,
            "after_parent_reclaim": after_reclaim,
            "stored_child_may_remain": True,
            "unconditional_cached_child_would_violate_lineage": True}


def local_budget():
    cap, spent = 8, set()
    accepted = []
    for i in range(100):
        request = f"distinct-key-{i}"
        if len(spent) < cap and request not in spent:
            spent.add(request)
            accepted.append(request)
    before = len(spent)
    spent.add(accepted[0])  # exact replay cannot add an allocation
    assert len(spent) == before == cap
    return {"parent_slots": cap, "distinct_labels_offered": 100,
            "allocations": len(spent), "replay_adds_allocation": False,
            "global_identity_fairness": "not established"}


def evidence_counterexamples():
    transcript = ("state-revision-7", "source-time-100", "owner-action-chain")
    worlds = [{"real_time": 100, "visible": transcript},
              {"real_time": 100+6*3600, "visible": transcript}]
    assert worlds[0]["visible"] == worlds[1]["visible"]
    prefix = ("event-0",)
    branch_a, branch_b = prefix+("successor-a",), prefix+("successor-b",)
    assert branch_a[:1] == branch_b[:1] == prefix
    assert branch_a != branch_b
    rank = max([("honest", 100), ("hostile", 10**12)], key=lambda x:x[1])
    assert rank[0] == "hostile"
    return {
        "fresh_and_stale_worlds_share_observation": worlds,
        "owner_claimed_timestamp_naively_selects": rank,
        "split_logs": {"common_prefix": prefix, "branch_a": branch_a,
                       "branch_b": branch_b, "both_extend_known_prefix": True,
                       "chooses_canonical_branch": False},
        "renewal_and_reclaim": {
            "old_renewal": {"generation": 3, "timely_admission": "not evidenced"},
            "new_claim": {"generation": 4, "committed": "not established by a signature"},
            "union_can_authorize_both": False,
            "required_evidence": "admission, predecessor, time and agreed lifecycle",
        },
        "limitations": [
            "All visible records are symbolic; no proof is verified here.",
            "A usable honest communication path and endpoint execution are assumptions.",
            "Neither eventual replication nor quorum structure bounds delivery latency.",
        ],
    }


if __name__ == "__main__":
    configs = analyzed_configurations()
    semantic = {
        "immutable_facts": immutable_facts(), "independent_bindings": independent_bindings(),
        "exclusive_successors": same_predecessor(), "root_claims": claim_counterexample(),
        "lineage": lineage_counterexample(), "local_budget": local_budget(),
        "evidence": evidence_counterexamples(),
    }
    output = {
        "kind": "finite_symbolic_counterexamples_not_protocol_qualification",
        "environment": {"python": platform.python_version(), "platform": platform.platform()},
        "source_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
        "enumerated_nonempty_subsets_including_fault_reductions": ENUMERATED_SUBSETS,
        "quorum_configurations": configs, "semantic_cases": semantic,
        "comparison_outcome": "partial_obligations_fit; no_complete_public_protocol_selected",
    }
    print(json.dumps(output, indent=2, sort_keys=True, allow_nan=False))
