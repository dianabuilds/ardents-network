#!/usr/bin/env python3
"""Apply one verified NET-14 recovery manifest to live relay containers."""

import argparse
import json
import re
import subprocess
import sys
import threading
import time
from pathlib import Path

SAFE = re.compile(r"^[a-z0-9_-]{1,63}$")


def command(arguments):
    return subprocess.run(arguments, check=True, capture_output=True, text=True, timeout=10)


def netem(segment):
    delay = int(segment["DelayMicros"])
    jitter = int(segment["JitterP95Micros"])
    loss = int(segment["LossPartsPerMillion"])
    if delay <= 0 or delay > 150_000 or jitter < 0 or jitter > 100_000 or loss < 0 or loss > 50_000:
        raise RuntimeError("recovery segment impairment is invalid")
    result = ["netem", "delay", f"{delay}us"]
    if jitter:
        result += [f"{jitter}us", "25%"]
    if loss:
        result += ["loss", f"{loss / 10_000:.4f}%"]
    return result


def emit(value, lock):
    with lock:
        print(json.dumps(value, sort_keys=True, separators=(",", ":")), flush=True)


def apply_failure(failure, relay, segment, parent, started_millis, output, failures):
    try:
        due = started_millis + int(failure["AtMillis"])
        remaining = (due - time.time_ns() // 1_000_000) / 1000
        if remaining <= 0:
            raise RuntimeError("recovery fault scheduler missed its declared start")
        time.sleep(remaining)
        handle = "10:" if parent == "1:10" else "20:"
        base = ["docker", "exec", relay["Container"], "/usr/sbin/tc", "qdisc", "replace", "dev", "eth0", "parent", parent, "handle", handle]
        command(base + ["netem", "loss", "100%"])
        actual_start = time.time_ns() // 1_000_000
        emit({"kind": "recovery-fault-start", "episode": failure["Episode"], "segment": failure["SegmentID"], "scheduled_millis": due, "actual_millis": actual_start}, output)
        time.sleep(int(failure["DurationMillis"]) / 1000)
        command(base + netem(segment))
        actual_stop = time.time_ns() // 1_000_000
        emit({"kind": "recovery-fault-stop", "episode": failure["Episode"], "segment": failure["SegmentID"], "scheduled_millis": due + int(failure["DurationMillis"]), "actual_millis": actual_stop}, output)
    except Exception as error:
        failures.append(str(error))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    parser.add_argument("host_role", choices=("reader", "publisher"))
    parser.add_argument("started_millis", type=int)
    arguments = parser.parse_args()
    manifest = json.loads(arguments.manifest.read_text(encoding="utf-8"))
    if manifest.get("Cell") != "net14-recovery" or arguments.started_millis <= 0:
        raise RuntimeError("recovery scheduler input is invalid")
    segments = {item["ID"]: item for path in manifest["Paths"] for item in path["Segments"]}
    relays = {}
    for relay in manifest["Relays"]:
        if not SAFE.fullmatch(relay["Container"]):
            raise RuntimeError("recovery relay container is invalid")
        relays[relay["UpstreamSegment"]] = (relay, "1:10")
        relays[relay["ClientSegment"]] = (relay, "1:20")
    output = threading.Lock()
    failures = []
    workers = []
    for failure in manifest["Failures"]:
        relay, parent = relays[failure["SegmentID"]]
        if relay["Host"] != arguments.host_role:
            continue
        worker = threading.Thread(target=apply_failure, args=(failure, relay, segments[failure["SegmentID"]], parent, arguments.started_millis, output, failures))
        worker.start()
        workers.append(worker)
    for worker in workers:
        worker.join()
    if failures:
        raise RuntimeError("; ".join(failures))
    emit({"kind": "recovery-faults-complete", "host": arguments.host_role, "episodes": len(workers)}, output)


if __name__ == "__main__":
    try:
        main()
    except (OSError, KeyError, TypeError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
