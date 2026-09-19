#!/usr/bin/env python3
import datetime
import json
import re
import signal
import subprocess
import sys
import time

SAFE = re.compile(r"^[a-z0-9_-]{1,63}$")
stopping = False


def stop(_signal, _frame):
    global stopping
    stopping = True


def classes(container):
    output = subprocess.run(
        ["docker", "exec", container, "/usr/sbin/tc", "-s", "-j", "qdisc", "show", "dev", "eth0"],
        check=True, capture_output=True, text=True, timeout=10,
    ).stdout
    values = {}
    for record in json.loads(output):
        if record.get("kind") != "netem" or record.get("parent") not in ("1:10", "1:20"):
            continue
        value = record.get("bytes", record.get("stats", {}).get("bytes"))
        if not isinstance(value, int) or value < 0 or record["parent"] in values:
            raise RuntimeError("relay class counter is invalid")
        values[record["parent"]] = value
    if set(values) != {"1:10", "1:20"}:
        raise RuntimeError("relay class counters are incomplete")
    return values


if len(sys.argv) != 2 or not SAFE.fullmatch(sys.argv[1]):
    raise SystemExit("usage: relay_owner_samples.py SAFE_CONTAINER")
container = sys.argv[1]
signal.signal(signal.SIGTERM, stop)
signal.signal(signal.SIGINT, stop)
previous = None
for _ in range(1325):
    current = classes(container)
    if previous is not None and any(current[key] < previous[key] for key in current):
        raise SystemExit("relay class counter regressed")
    print(json.dumps({
        "At": datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z"),
        "UpstreamBytes": current["1:10"], "ClientBytes": current["1:20"],
    }, separators=(",", ":")), flush=True)
    previous = current
    if stopping:
        break
    time.sleep(1)
else:
    raise SystemExit("relay sampler exceeded its bound")
