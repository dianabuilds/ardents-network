#!/usr/bin/env python3
import datetime
import os
import signal
import sys
import time

if len(sys.argv) != 2 or not os.path.isabs(sys.argv[1]):
    raise SystemExit("usage: clock_observer.py ABSOLUTE_PATH")

destination = os.path.normpath(sys.argv[1])
running = True

def stop(_signum, _frame):
    global running
    running = False

signal.signal(signal.SIGTERM, stop)
signal.signal(signal.SIGINT, stop)
os.makedirs(os.path.dirname(destination), mode=0o755, exist_ok=True)
while running:
    stamp = datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    temporary = destination + ".pending"
    with open(temporary, "w", encoding="ascii", newline=chr(10)) as stream:
        stream.write(stamp + chr(10))
        stream.flush()
        os.fsync(stream.fileno())
    os.chmod(temporary, 0o644)
    os.replace(temporary, destination)
    time.sleep(0.5)