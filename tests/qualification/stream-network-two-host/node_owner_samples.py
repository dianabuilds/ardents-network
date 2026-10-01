#!/usr/bin/env python3
import datetime
import json
import re
import subprocess
import sys
import time

if len(sys.argv) != 2 or not re.fullmatch(r"ardents-qualification-(?:node-[0-9a-f]{32}-(?:[0-9]|1[0-5])|source-[0-9a-f]{32}-[01])\.service", sys.argv[1]):
    raise SystemExit("usage: node_owner_samples.py SAFE_UNIT")
unit = sys.argv[1]
previous = None
for _ in range(1325):
    shown = subprocess.run(
        ["systemctl", "show", unit, "-p", "ActiveState", "-p", "MemoryCurrent", "-p", "CPUUsageNSec", "-p", "IPIngressBytes", "-p", "IPEgressBytes"],
        check=True, capture_output=True, text=True,
    ).stdout.splitlines()
    values = dict(line.split("=", 1) for line in shown if "=" in line)
    if values.get("ActiveState") not in ("active", "activating", "deactivating"):
        break
    names = ("MemoryCurrent", "CPUUsageNSec", "IPIngressBytes", "IPEgressBytes")
    if any(not values.get(name, "").isdecimal() for name in names):
        raise SystemExit("Node owner counters are unavailable")
    current = tuple(int(values[name]) for name in names)
    if previous is not None and any(current[index] < previous[index] for index in (1, 2, 3)):
        raise SystemExit("Node owner counter regressed")
    print(json.dumps({
        "At": datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z"),
        "MemoryCurrent": current[0], "CPUUsageNSec": current[1],
        "IPIngressBytes": current[2], "IPEgressBytes": current[3],
    }, separators=(",", ":")), flush=True)
    previous = current
    time.sleep(1)
else:
    raise SystemExit("Node owner sampler exceeded its bound")
