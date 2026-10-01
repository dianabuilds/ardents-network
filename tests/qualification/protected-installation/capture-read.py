#!/usr/bin/env python3
"""Capture one public read; this does not qualify the installed journey."""
import hashlib
import json
import os
import pathlib
import re
import signal
import subprocess
import sys


def refuse(message):
    raise SystemExit(message)


if len(sys.argv) != 7:
    refuse("usage: capture-read.py text-binary binary-sha256 application-socket target-link-file expected-presentation-sha256 new-evidence-directory")
binary, digest, socket, link_path, expected, evidence = sys.argv[1:]
for path in (binary, socket, link_path, evidence):
    if not os.path.isabs(path) or os.path.normpath(path) != path:
        refuse("invalid environment: canonical absolute paths required")
for value in (digest, expected):
    if not re.fullmatch(r"[0-9a-f]{64}", value):
        refuse("invalid environment: independent SHA-256 required")
if os.path.islink(binary) or os.path.islink(link_path):
    refuse("invalid environment: direct program and Target Link file required")
with open(binary, "rb") as source:
    observed = hashlib.file_digest(source, "sha256").hexdigest()
if observed != digest:
    refuse("invalid environment: independently declared program digest differs")
with open(link_path, "rb") as source:
    link = source.read(515)
if not link or len(link) > 514 or b"\n" in link.rstrip(b"\n"):
    refuse("invalid environment: one bounded Target Link line required")
# Complete Target admission belongs to the public command and Endpoint.
checkout = str(pathlib.Path(__file__).resolve().parents[3])
if os.path.commonpath((checkout, os.path.realpath(evidence))) == checkout:
    refuse("invalid environment: evidence must be outside repository")
revision = os.environ.get("ARDENTS_INSTALLATION_SOURCE_REVISION", "unknown")
if revision != "unknown" and not re.fullmatch(r"[0-9a-f]{40}", revision):
    refuse("invalid environment: source revision must be a full commit or unknown")
os.umask(0o077)
os.mkdir(evidence, 0o700)
with open(os.path.join(evidence, "run-identity.json"), "x") as record:
    json.dump({"program_sha256": digest, "declared_source_revision": revision,
               "expected_presentation_sha256": expected,
               "scope": "one public read capture; not installed qualification"}, record)
    record.write("\n")
# The UI refuses regular-file stdout. Pipes preserve its deadline-capable IO;
# communicate retains this command's exit, without a shell pipeline or retry.
try:
    process = subprocess.Popen([binary, "read", socket], stdin=subprocess.PIPE,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE)
except OSError as error:
    # No child exit exists when exec itself refuses. Preserve that distinction.
    with open(os.path.join(evidence, "read-launch-failure.json"), "x") as record:
        json.dump({"errno": error.errno, "error": str(error)}, record)
        record.write("\n")
    with open(os.path.join(evidence, "read.exit"), "x") as record:
        record.write("not-started\n")
    refuse("captured original process launch refusal; no retry or reset")
timed_out = False
interruption = None
cleanup_errors = []
output, diagnostic = b"", b""


def interrupt_capture(signum, frame):
    raise KeyboardInterrupt("capture interrupted by signal " + str(signum))


signal.signal(signal.SIGTERM, interrupt_capture)
try:
    output, diagnostic = process.communicate(link, timeout=40)
except BaseException as error:
    timed_out = isinstance(error, subprocess.TimeoutExpired)
    interruption = type(error).__name__
    # A second interrupt must not abandon the owned child during cleanup.
    signal.signal(signal.SIGINT, signal.SIG_IGN)
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    try:
        with open(os.path.join(evidence, "read-interruption.json"), "x") as record:
            json.dump({"kind": interruption, "error": str(error)}, record)
            record.write("\n")
    except OSError as capture_error:
        cleanup_errors.append(str(capture_error))
    if process.poll() is None:
        try:
            process.terminate()
        except OSError as cleanup_error:
            cleanup_errors.append(str(cleanup_error))
    try:
        output, diagnostic = process.communicate(timeout=5)
    except BaseException as cleanup_error:
        cleanup_errors.append(str(cleanup_error))
        output = getattr(cleanup_error, "output", None) or b""
        diagnostic = getattr(cleanup_error, "stderr", None) or b""
        try:
            if process.poll() is None:
                process.kill()
        except OSError as kill_error:
            cleanup_errors.append(str(kill_error))
        try:
            output, diagnostic = process.communicate(timeout=5)
        except BaseException as join_error:
            cleanup_errors.append(str(join_error))
            output = getattr(join_error, "output", None) or output
            diagnostic = getattr(join_error, "stderr", None) or diagnostic
for name, body in (("read.stdout", output), ("read.stderr", diagnostic)):
    with open(os.path.join(evidence, name), "xb") as capture:
        capture.write(body)
with open(os.path.join(evidence, "read.exit"), "x") as record:
    record.write((str(process.returncode) if process.returncode is not None else "unjoined") + "\n")
with open(os.path.join(evidence, "read-result.json"), "x") as record:
    json.dump({"timed_out": timed_out, "interruption": interruption,
               "joined": process.poll() is not None,
               "cleanup_errors": cleanup_errors,
               "presentation_sha256": hashlib.sha256(output).hexdigest()}, record)
    record.write("\n")
if interruption is not None or process.returncode != 0:
    refuse("captured original read failure; no retry or reset")
if hashlib.sha256(output).hexdigest() != expected:
    refuse("captured presentation differs from independently expected bytes")
print("Public read captured; installed journey, Carrier and confinement remain separate evidence.")
