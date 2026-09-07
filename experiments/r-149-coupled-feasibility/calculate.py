"""Reproducible R-149 arithmetic; no consensus, crypto, or runtime simulation."""
from fractions import Fraction
import hashlib
import json
import math
from pathlib import Path
import platform

DAY = 86400
MIB = 1024**2
GIB = 1024**3
EPOCH = 300
FINALITY = 10800
INCLUSION = 600
CLOCK_MARGIN = 240


def catchup(q, z):
    """Exact fixed-rate concurrent race; a tie counts as attacker success."""
    if q >= Fraction(1, 2):
        return Fraction(1)
    a, b = q.numerator, q.denominator
    n = 2 * z - 1
    numerator = sum(math.comb(n, k) * a**k * (b-a)**(n-k)
                    for k in range(z, n+1))
    return Fraction(2 * numerator, b**n)


def catchup_negative_binomial(q, z):
    """Independent finite sum from the attacker's negative-binomial count."""
    if q >= Fraction(1, 2):
        return Fraction(1)
    p = 1-q
    return 1-sum(math.comb(z+k-1, k) * (p**z*q**k-q**z*p**k)
                 for k in range(z))


def erlang_survival(z, seconds, rate):
    """P(fewer than z arrivals); finite Poisson sum at the chosen finite inputs."""
    mean = rate * seconds
    term = math.exp(-mean)
    terms = [term]
    for k in range(1, z):
        term *= mean/k
        terms.append(term)
    return min(1.0, math.fsum(terms))


def erlang_quantile(z, rate, quantile):
    lo, hi = 0.0, z/rate*4
    if erlang_survival(z, hi, rate) > 1-quantile:
        raise ValueError("predeclared quantile search bracket insufficient")
    for _ in range(100):
        mid = (lo+hi)/2
        if erlang_survival(z, mid, rate) > 1-quantile:
            lo = mid
        else:
            hi = mid
    return hi


def traffic(nodes, names, renewal_hours=6):
    node_ops = nodes * (24/renewal_hours + .05)
    name_body = 100*4096 + (names/30 + names*.01)*4096
    opaque_claims = 100*64
    infra_updates = node_ops*1024
    headers = DAY/60*256
    common_bytes = infra_updates + headers + opaque_claims
    # Names are on the separate Namespace path. It is NOT mandatory client data.
    aggregate_history = common_bytes + name_body
    full_view_daily = nodes*512*(DAY/EPOCH)
    delta_wire_daily = common_bytes*1.5 + 8*MIB
    proof_8k = delta_wire_daily + DAY/EPOCH*8192*1.5
    proof_64k = delta_wire_daily + DAY/EPOCH*65536*1.5
    proof_room = (25*MIB-delta_wire_daily)/((DAY/EPOCH)*1.5)
    return {
        "nodes": nodes, "names": names, "renewal_hours": renewal_hours,
        "node_updates_per_day": node_ops,
        "common_payload_mib_day": common_bytes/MIB,
        "separate_namespace_payload_mib_day": name_body/MIB,
        "common_replay_gib_year": common_bytes*365/GIB,
        "common_replay_transfer_seconds_at_100mbit": common_bytes*365*8/100e6,
        "aggregate_history_gib_year": aggregate_history*365/GIB,
        "current_view_mib": nodes*512/MIB,
        "current_name_records_mib": names*1920/MIB,
        "full_view_payload_mib_day": full_view_daily/MIB,
        "delta_assumed_wire_mib_day": delta_wire_daily/MIB,
        "with_hypothetical_8k_proof_mib_day": proof_8k/MIB,
        "with_hypothetical_64k_proof_mib_day": proof_64k/MIB,
        "max_proof_bytes_per_epoch_with_assumed_reserve": proof_room,
        "aggregate_average_payload_bytes_second": aggregate_history/DAY,
    }


def renewal(censorship_days, parent_end_days=37):
    start = 23*DAY
    # Envelope requires valid inclusion, complete data and timely proof delivery.
    committed = (start + censorship_days*DAY + INCLUSION + FINALITY
                 + EPOCH + CLOCK_MARGIN)
    end = min(37, parent_end_days)*DAY
    return {
        "censorship_days": censorship_days,
        "effective_deadline_day": end/DAY,
        "conservative_completion_day": committed/DAY,
        "margin_hours": (end-committed)/3600,
        "exclusive_renewal_before_deadline": committed < end,
    }


def calculations():
    comparisons = 0
    for q in (Fraction(0), Fraction(1, 10), Fraction(3, 10),
              Fraction(2, 5), Fraction(49, 100), Fraction(51, 100)):
        for z in (1, 6, 80):
            assert catchup(q, z) == catchup_negative_binomial(q, z)
            comparisons += 1
    assert catchup(Fraction(1, 10), 1) == Fraction(1, 5)
    assert abs(float(catchup(Fraction(3, 10), 50)) - .0000311) < 0.00000005
    assert renewal(7)["exclusive_renewal_before_deadline"]
    assert not renewal(14)["exclusive_renewal_before_deadline"]
    assert not renewal(0, parent_end_days=20)["exclusive_renewal_before_deadline"]
    assert erlang_survival(1, 60, 1/60) == math.exp(-1)
    q = Fraction(3, 10)
    races = []
    for share in (Fraction(1, 10), q, Fraction(2, 5),
                  Fraction(49, 100), Fraction(51, 100)):
        races.append({
            "hostile_work_fraction": float(share),
            "six_confirmations": float(catchup(share, 6)),
            "eighty_confirmations": float(catchup(share, 80)),
        })
    rate = .7/60  # attacker withholds; total reference supply unchanged
    finality = []
    for z in (6, 80):
        finality.append({
            "confirmations": z, "mean_minutes": z/rate/60,
            "p99_minutes": erlang_quantile(z, rate, .99)/60,
            "p999_minutes": erlang_quantile(z, rate, .999)/60,
            "exceeds_180_minutes_probability": erlang_survival(z, FINALITY, rate),
        })
    supply = []
    for loss in (0, .5, .9, 1):
        honest, attacker = 70*(1-loss), 30
        share = attacker/(honest+attacker)
        supply.append({
            "honest_loss": loss, "remaining_honest_work_units": honest,
            "hostile_share": share,
            "eighty_honest_blocks_mean_minutes": None if honest == 0 else 8000/honest,
            "eventual_race_catchup": float(catchup(Fraction(str(share)), 80)),
        })
    times = []
    for days in (0, 7, 30, 365):
        error = 30 + days*DAY*50/1e6
        times.append({"elapsed_days": days, "uncertainty_seconds": error,
                      "within_120_seconds": error <= 120})
    finality_age = FINALITY + EPOCH + INCLUSION + CLOCK_MARGIN
    common = traffic(1000, 10000)
    risk = float(catchup(q, 80))
    return {
        "consistency_checks": {"exact_formula_comparisons": comparisons,
                               "known_probability_time_and_lease_boundaries": "passed"},
        "races": races,
        "risk_min_confirmations_q30_1e6": next(
            z for z in range(1, 201) if catchup(q, z) <= Fraction(1, 10**6)),
        "risk_union_upper_for_1000_races_at_q30_z80": min(1, 1000*risk),
        "risk_union_upper_for_1000000_races_at_q30_z80": min(1, 1000000*risk),
        "finality": finality,
        "no_honest_block_in_600s_probability": math.exp(-rate*INCLUSION),
        "age_budget": {
            "cutoff_plus_confirmation_acquisition_clock_seconds": finality_age,
            "fits_30_minutes": finality_age <= 1800,
            "fits_4_hours": finality_age <= 14400,
            "remaining_4hour_headroom_minutes": (14400-finality_age)/60,
            "finalized_commit_reveal_window_seconds": EPOCH,
            "fits_finality_inside_next_epoch": FINALITY+INCLUSION+CLOCK_MARGIN <= EPOCH,
        },
        "resource_supply": supply,
        "minimum_honest_to_attacker_ratio_for_q30": 7/3,
        "traffic": [common, traffic(10000, 100000), traffic(1000, 10000, 1)],
        "saturated_log": {
            "aggregate_payload_mib_day": 65536*(DAY/60)/MIB,
            "aggregate_payload_gib_year": 65536*(DAY/60)*365/GIB,
            "assumed_common_half_payload_mib_day": 32768*(DAY/60)/MIB,
            "node_class_updates_second": 32768/1024/60,
            "name_class_4k_updates_second": 32768/4096/60,
            "name_class_4k_updates_day_at_nominal_rate": 32768/4096*DAY/60,
            "name_class_4k_updates_day_if_only_q30_honest_publish": 32768/4096*DAY/60*.7,
            "name_class_queue_slots": 4096,
            "name_class_queue_mib_at_4k": 4096*4096/MIB,
            "queue_fill_seconds_at_10_valid_updates_second":
                4096/(10-32768/4096/60),
            "individual_inclusion_bound_from_class_cap": None,
        },
        "time": times,
        "elapsed_days_until_120s_uncertainty": (120-30)/(50/1e6)/DAY,
        "renewal": [renewal(d) for d in (0, 7, 13, 14, 21)],
        "parent_deadline_case": renewal(0, parent_end_days=20),
        "record_expires_at_active_end_case": {
            "renewal_can_still_commit_inside_grace":
                renewal(7)["exclusive_renewal_before_deadline"],
            "old_record_resolvable_after_day30": False,
            "resolution_gap_until_renewed_record_minutes":
                (renewal(7)["conservative_completion_day"]-30)*1440,
            "note": "Record notAfter limits resolution, not the Authority's Grace renewal right",
        },
        "name_registration": {
            "two_epoch_pipeline_plus_envelope_minutes": (2*EPOCH+INCLUSION+FINALITY+CLOCK_MARGIN)/60,
            "interpretation": "optimistic only; safely finalizing before reveal does not fit E+1",
        },
        "input_envelope_only": {
            "name_leaf_plus_binary_path_and_256_framing_bytes":
                1920 + math.ceil(math.log2(10000))*32 + 256,
            "remaining_4096_bytes": 4096-(1920 + math.ceil(math.log2(10000))*32 + 256),
            "validity_lineage_finality_proof_fits": "UNKNOWN, simple inclusion is insufficient",
        },
        "not_measured": [
            "cryptographic validity/completeness/currentness or proof production",
            "CPU/RSS, retained indexes, memory amplification, verification latency",
            "real operator independence, honest funding, current rental cost",
            "difficulty adjustment, dissemination delay, censorship scheduling",
            "anonymous Name fairness, per-owner inclusion, public privacy",
            "maintained component fit, integration effort, public qualification",
        ],
        "profile_verdict": "NO_COMPLETE_QUALIFIED_PROFILE; some conditional arithmetic fits",
    }


if __name__ == "__main__":
    payload = {
        "kind": "analytical_results_not_network_measurements",
        "environment": {"python": platform.python_version(), "platform": platform.platform()},
        "calculator_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
        "results": calculations(),
    }
    print(json.dumps(payload, indent=2, sort_keys=True, allow_nan=False))
