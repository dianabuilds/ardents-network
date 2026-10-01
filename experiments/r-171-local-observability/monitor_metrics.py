"""Research-only mapping of one existing safe monitor snapshot to Prometheus.

No sampling, source discovery, raw-log access, network listener or product grant.
"""
import datetime
import json
import math
import sys

MAX_BYTES = 65536
# Input field, exact scope/name, type, scale, explanation.
FIELDS = (
    ("cpu_usage_usec", "cgroup_cpu_usage_seconds_total", "counter", 0.000001,
     "Cumulative selected cgroup CPU time; resets break continuity."),
    ("memory_bytes", "cgroup_memory_bytes", "gauge", 1,
     "Current selected cgroup memory; not process RSS."),
)
# Both current Node samplers populate CPU/memory. Other serialized fields do
# not identify availability: the closed owner-cgroup sampler leaves defaults.


def timestamp(value):
    if not isinstance(value, str):
        raise ValueError("observation timestamp unavailable")
    parsed = datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        raise ValueError("observation timestamp must include timezone")
    return parsed.timestamp()


def number(value):
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise ValueError("numeric observation unavailable")
    if not math.isfinite(value) or value < 0:
        raise ValueError("invalid numeric observation")
    return value


def project(state, now, sample_max_age):
    """Return fixed unlabelled metrics; caller binds the explicitly selected job."""
    if not isinstance(state, dict) or state.get("schema") != "ardents-monitor-v1":
        raise ValueError("selected monitor schema required")
    if not math.isfinite(now) or not math.isfinite(sample_max_age) or sample_max_age <= 0:
        raise ValueError("finite positive freshness budget required")
    updated = timestamp(state.get("updated"))
    started = timestamp(state.get("started"))
    if updated > now or started > updated:
        raise ValueError("invalid monitor observation interval")
    alive = state.get("source_alive")
    if not isinstance(alive, bool):
        raise ValueError("source process status unavailable")
    lines = []

    def emit(name, kind, help_text, value):
        name = "diagnostic_selected_" + name
        lines.extend((f"# HELP {name} {help_text}", f"# TYPE {name} {kind}",
                      f"{name} {value:.17g}"))

    fresh = now - updated <= 3
    emit("monitor_fresh", "gauge", "Supervisor heartbeat within three seconds.", int(fresh))
    # Missing status is not measured zero; stale source status is omitted.
    if fresh:
        emit("source_process_alive", "gauge",
             "Supervisor observed process survival; not product readiness.", int(alive))
    emit("session_started_seconds", "gauge",
         "Supervisor session start; changes identify counter reset boundaries.", started)
    emit("monitor_observation_age_seconds", "gauge", "Supervisor heartbeat age.", now-updated)
    row = state.get("latest_sample")
    available = False
    if fresh and alive and isinstance(row, dict):
        entry = row.get("entry")
        if (isinstance(entry, dict) and entry.get("schema") == "ardents-node-event-v1"
                and entry.get("kind") == "resource-sample"):
            # Producer and collector time remain distinct. Receipt time cannot
            # make a replayed old producer observation current.
            producer = timestamp(entry.get("at"))
            received = timestamp(row.get("at"))
            if producer > now or received > now or producer < started or received < started:
                raise ValueError("invalid sample observation interval")
            emit("sample_age_seconds", "gauge", "Selected producer sample age.", now-producer)
            available = now-producer <= sample_max_age and now-received <= sample_max_age
            if available:
                resource = entry.get("resource")
                if not isinstance(resource, dict):
                    raise ValueError("sample resources unavailable")
                for field, name, kind, scale, help_text in FIELDS:
                    if field in resource:
                        emit(name, kind, help_text, number(resource[field])*scale)
    emit("sample_fresh", "gauge",
         "Fresh selected Node sample and supervisor; absent fields remain absent.",
         int(available))
    return "\n".join(lines) + "\n"


def main():
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sample-max-age", type=float, required=True,
                        help="Explicit positive seconds from selected producer interval")
    args = parser.parse_args()
    data = sys.stdin.buffer.read(MAX_BYTES+1)
    if len(data) > MAX_BYTES:
        raise ValueError("selected safe snapshot exceeds64KiB")
    state = json.loads(data)
    sys.stdout.write(project(state, datetime.datetime.now(datetime.timezone.utc).timestamp(),
                             args.sample_max_age))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, TypeError, KeyError, OverflowError, RecursionError):
        # Never echo malformed input, arbitrary keys, source labels or raw paths.
        print("Selected safe monitor snapshot unavailable or invalid", file=sys.stderr)
        sys.exit(1)