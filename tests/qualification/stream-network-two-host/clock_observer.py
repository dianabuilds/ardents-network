#!/usr/bin/env python3
import datetime
import os
import signal
import stat
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
temporary = destination + ".pending"

while running:
    stamp = datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    flags = os.O_WRONLY | os.O_CREAT | os.O_TRUNC
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    descriptor = os.open(temporary, flags, 0o644)
    try:
        if not stat.S_ISREG(os.fstat(descriptor).st_mode):
            raise SystemExit("clock observation must be one regular file")
        os.write(descriptor, (stamp + chr(10)).encode("ascii"))
        # This is a live confidence signal, not durable State. Waiting for
        # storage sync can make a healthy observer stale under unrelated host
        # I/O; atomic replacement still keeps every reader on a complete value.
    finally:
        os.close(descriptor)
    os.chmod(temporary, 0o644)
    os.replace(temporary, destination)
    time.sleep(0.5)
