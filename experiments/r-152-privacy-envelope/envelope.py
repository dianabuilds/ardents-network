"""Deterministic optimistic cost/delay bounds for R-152; no anonymity claim."""
import argparse
import csv
import hashlib
import json
import math
from pathlib import Path
import platform
import sys

DAY_SECONDS = 86400
IDLE_BYTES = 1_000_000_000
COLD_SECONDS = 3.0
WARM_SECONDS = 1.0
PACKET_BYTES = (512, 1024, 1280, 2048, 4096, 8192, 16384, 32768)
RESERVED_FRACTIONS = (0.0, 0.2)
HOLD_MEANS = (0.01, 0.02, 0.05, 0.1, 0.2)


def erlang_cdf(x, shape):
    """CDF of a unit-mean-stage Erlang distribution, evaluated without SciPy."""
    if x <= 0:
        return 0.0
    term = 1.0
    total = term
    for n in range(1, shape):
        term *= x / n
        total += term
    return max(0.0, min(1.0, 1.0 - math.exp(-x) * total))


def erlang_quantile(probability, shape):
    low, high = 0.0, float(shape)
    while erlang_cdf(high, shape) < probability:
        high *= 2
    for _ in range(100):
        mid = (low + high) / 2
        if erlang_cdf(mid, shape) < probability:
            low = mid
        else:
            high = mid
    return (low + high) / 2


def calculation_checks():
    # Cross-check the solver against the distinct closed-form exponential case.
    for probability in (0.5, 0.95, 0.99):
        observed = erlang_quantile(probability, 1)
        expected = -math.log1p(-probability)
        if not math.isclose(observed, expected, rel_tol=1e-12):
            raise RuntimeError("Erlang solver disagrees with exponential formula")
    q95 = erlang_quantile(0.95, 9)
    if not erlang_cdf(q95 - 1e-6, 9) < 0.95 < erlang_cdf(q95 + 1e-6, 9):
        raise RuntimeError("Erlang quantile bracket is invalid")
    if 2 * 10_000_000 / 8 * DAY_SECONDS != 216_000_000_000:
        raise RuntimeError("Directional byte accounting is invalid")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    script_path = Path(__file__).resolve()
    repository = script_path.parents[2]
    output = args.output.resolve()
    if output == repository or repository in output.parents:
        raise SystemExit("Generated evidence must stay outside the repository")
    output.mkdir(parents=True, exist_ok=True)
    calculation_checks()

    rows = []
    for fraction in RESERVED_FRACTIONS:
        available = IDLE_BYTES * (1 - fraction)
        for packet in PACKET_BYTES:
            rate = available / (2 * packet * DAY_SECONDS)
            wait95 = math.log(20) / rate
            rows.append({
                "packet_bytes_each_direction": packet,
                "other_idle_reserved_fraction": fraction,
                "max_mean_echoes_per_second": rate,
                "poisson_first_slot_wait_p95_seconds": wait95,
                "uniform_slot_wait_p95_seconds": 0.95 / rate,
                "poisson_wait_alone_exceeds_warm": wait95 > WARM_SECONDS,
                "poisson_wait_alone_exceeds_cold": wait95 > COLD_SECONDS,
            })

    constant_rate = [
        {
            "each_direction_bits_per_second": bps,
            "ideal_aggregate_bytes_per_day": 2 * bps / 8 * DAY_SECONDS,
            "ratio_to_idle_cap": 2 * bps / 8 * DAY_SECONDS / IDLE_BYTES,
        }
        for bps in (100_000, 1_000_000, 10_000_000)
    ]
    hold_q95 = erlang_quantile(0.95, 9)
    receipt = {
        "question": "R-152",
        "classification": "optimistic arithmetic model; no network or anonymity test",
        "python": platform.python_version(),
        "script_sha256": hashlib.sha256(script_path.read_bytes()).hexdigest(),
        "inputs": {
            "idle_aggregate_bytes_per_day": IDLE_BYTES,
            "day_seconds": DAY_SECONDS,
            "cold_first_response_p95_seconds": COLD_SECONDS,
            "warm_first_response_p95_seconds": WARM_SECONDS,
            "packet_grid_bytes": PACKET_BYTES,
            "other_idle_reserved_fractions": RESERVED_FRACTIONS,
            "hop_delay_count": 9,
            "hop_delay_mean_grid_seconds": HOLD_MEANS,
        },
        "accounting": {
            "aggregate_bits_per_second_at_cap": IDLE_BYTES * 8 / DAY_SECONDS,
            "symmetric_bits_per_second_each_direction": IDLE_BYTES * 8 / DAY_SECONDS / 2,
            "symmetric_bytes_per_second_each_direction": IDLE_BYTES / DAY_SECONDS / 2,
            "largest_symmetric_echo_packet_for_poisson_wait_p95_le_1_second":
                IDLE_BYTES * WARM_SECONDS / (2 * DAY_SECONDS * math.log(20)),
        },
        "unchanged_rate_cells": constant_rate,
        "schedule_cells": rows,
        "nine_exponential_holds_only": [
            {"mean_hold_seconds": mean, "round_trip_delay_p95_seconds": mean * hold_q95}
            for mean in HOLD_MEANS
        ],
        "limitations": [
            "Full equality of send and receive packet sizes is a model assumption.",
            "All framing, lookup, key, update and transport cost is omitted unless reserved.",
            "Useful data faster than the schedule changes the model and requires a different claim.",
            "Mean Poisson traffic does not guarantee a hard 24-hour cap.",
            "Mixing, topology, receiver retrieval, authentication and application processing add delay.",
            "An arithmetic cell below a limit does not establish feasible or anonymous operation.",
        ],
    }
    receipt_path = output / "envelope.json"
    receipt_path.write_text(json.dumps(receipt, indent=2) + "\n", encoding="utf-8")
    with (output / "schedule.csv").open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=list(rows[0]))
        writer.writeheader()
        writer.writerows(rows)
    print(json.dumps({
        "receipt": str(receipt_path),
        "accounting": receipt["accounting"],
        "constant_rate": constant_rate,
        "selected_schedule_cells": [r for r in rows if r["other_idle_reserved_fraction"] == 0
                                    and r["packet_bytes_each_direction"] in (1024, 1280, 2048, 16384)],
        "erlang": receipt["nine_exponential_holds_only"],
        "calculation_checks": "passed",
    }, indent=2))


if __name__ == "__main__":
    main()
