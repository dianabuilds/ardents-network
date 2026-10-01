"""Actual accepted Node resource probe; local shared-container engineering only."""
import argparse
import datetime as dt
import hashlib
import json
import os
import pathlib
import re
import shutil
import signal
import subprocess
import tarfile
import tempfile
import time
import urllib.request

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--carrier", choices=("tcp-tls", "quic"), default="tcp-tls")
parser.add_argument("--preview-seconds", type=int, default=0,
                    help="Explicit finite operator preview, 300..7200 seconds; zero runs the probe")
parser.add_argument("--debug-profiles", action="store_true",
                    help="Explicit private runtime/CPU/heap/allocs/goroutine/block/mutex/trace capture from the owned Node")
parser.add_argument("--persistent-journal", action="store_true",
                    help="Use normal retained-journal budgets in the explicit persistent preview")
args = parser.parse_args()
if args.persistent_journal and not args.preview_seconds:
    parser.error("persistent journal requires an explicit preview")
if args.debug_profiles and args.preview_seconds:
    parser.error("private profiling cannot be combined with the monitoring preview")
if args.preview_seconds and not 300 <= args.preview_seconds <= 7200:
    parser.error("preview duration must be 300..7200 seconds")
preview_stop = False
def stop_preview(signum, frame):
    global preview_stop
    preview_stop = True
if args.preview_seconds:
    signal.signal(signal.SIGTERM, stop_preview)
    signal.signal(signal.SIGINT, stop_preview)
os.umask(0o077)
reports = pathlib.Path("/reports")
binaries = pathlib.Path("/binaries")
receipt = {"complete": False, "qualification": False, "placement": "one local shared container",
           "source_scope": "one accepted Introduction Node and two actual Sources", "carrier": args.carrier}
processes = []
work = None
phase = "verify-build"

def invoke(label, *args, timeout=20):
    global phase
    phase = label
    result = subprocess.run([str(a) for a in args], capture_output=True, timeout=timeout)
    if work:
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
    if result.returncode != 0:
        raise RuntimeError("selected command failed at " + label)
    if len(result.stdout) > 1 << 20 or len(result.stderr) > 1 << 20:
        raise RuntimeError("command output exceeded bound at " + label)
    return result.stdout

def write(path, value):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    path.write_text(json.dumps(value))
    path.chmod(0o600)
    return path

def spawn(label, *args):
    output = (work / (label + ".stdout")).open("wb")
    error = (work / (label + ".stderr")).open("wb")
    try:
        process = subprocess.Popen([str(a) for a in args], stdout=output, stderr=error)
    finally:
        output.close()
        error.close()
    processes.append((label, process))
    return process

def wait_ready(label, process):
    deadline = time.monotonic() + 15
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError("source exited at " + label)
        path = work / (label + ".stdout")
        if path.stat().st_size > 64 << 10:
            raise RuntimeError("source output outside bound")
        for line in path.read_bytes().splitlines():
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            if event.get("kind") == "source-ready":
                return
        time.sleep(0.1)
    raise RuntimeError("source readiness observation missing at " + label)

try:
    manifest = json.loads((binaries / "build-receipt.json").read_text(encoding="utf-8-sig"))
    required = {"ardents-diagnostics", "ardents-node", "ardents", "ardents-control", "qualification-network"}
    if not manifest.get("complete") or {a["name"] for a in manifest["artifacts"]} != required:
        raise RuntimeError("explicit complete Node fixture build required")
    for artifact in manifest["artifacts"]:
        path = binaries / artifact["name"]
        if not 0 < path.stat().st_size <= 64 << 20:
            raise RuntimeError("artifact outside bound")
        with path.open("rb") as source:
            if hashlib.file_digest(source, "sha256").hexdigest() != artifact["sha256"]:
                raise RuntimeError("selected artifact changed")
    receipt["artifacts"] = manifest["artifacts"]
    receipt["build"] = {key: manifest.get(key) for key in ("head", "dirty", "goos", "goarch", "helper_image")}
    work = pathlib.Path(tempfile.mkdtemp(prefix="accepted-node-", dir="/tmp"))
    generated = work / "generated"
    now = dt.datetime.now(dt.timezone.utc).replace(minute=0, second=0, microsecond=0)
    stamp = now.isoformat().replace("+00:00", "Z")
    seed = os.urandom(32).hex()
    invoke("generate", binaries / "qualification-network", "--output", generated,
           "--reader-host", "127.0.0.2", "--publisher-host", "127.0.0.3",
           "--carrier", args.carrier, "--cell", "net14ad", "--profile", "client-to-publisher",
           "--seed", seed, "--at", stamp)
    bundle = json.loads((generated / "fixture.json").read_text())
    runtime = work / "runtime"
    runtime.mkdir(mode=0o700)
    shutil.copytree(generated, runtime / "bundle")
    old_root, old_hosting = bundle["RemoteRoot"], bundle["HostingRoot"]
    def remap(value):
        if isinstance(value, str):
            if value == old_hosting or value.startswith(old_hosting + "/"):
                return str(runtime / "hosting") + value[len(old_hosting):]
            if value == old_root or value.startswith(old_root + "/"):
                return str(runtime) + value[len(old_root):]
            return value
        if isinstance(value, list):
            return [remap(v) for v in value]
        if isinstance(value, dict):
            return {k: remap(v) for k, v in value.items()}
        return value
    for path in (runtime / "bundle" / "plans").glob("*.json"):
        write(path, remap(json.loads(path.read_text())))
    provision = remap(json.loads((generated / "provisioning-inventory.json").read_text()))
    selected_plan = None
    for relative in bundle["NodePlans"]:
        plan = json.loads((runtime / "bundle" / relative).read_text())
        if "closed_introduction" in plan:
            selected_plan = (runtime / "bundle" / relative, plan)
            break
    if selected_plan is None:
        raise RuntimeError("current fixture has no Introduction duty")
    node_path, node_plan = selected_plan
    selected_owner = pathlib.Path(node_plan["state_root"]).name
    owners = [o for o in provision["State"] if o["Owner"] in {selected_owner, "source-reader", "source-publisher"}]
    if len(owners) != 3:
        raise RuntimeError("explicit source/Node owner selection incomplete")
    for index, owner in enumerate(owners):
        for name in ("Root", "LocalRoleStateRoot", "DutyParent"):
            if owner.get(name):
                pathlib.Path(owner[name]).mkdir(mode=0o700, parents=True, exist_ok=True)
        for duty_root in owner["DutyRoots"]:
            pathlib.Path(duty_root).mkdir(mode=0o700, parents=True, exist_ok=True)
        invoke("accept-state-" + str(index), binaries / "ardents", "accept-offline",
               "--state-root", owner["Root"], "--network-id", provision["NetworkID"],
               "--authorities", provision["AuthorityPublic"], "--threshold", "1", "--at", stamp,
               "--epoch", provision["Epoch"], "--inputs", provision["Inputs"],
               "--materialization", owner["Materialization"], "--profile", "ardents-route-v3",
               "--closed-profile-authority", provision["AuthorityPublic"])
    issuer_plan = runtime / "bundle" / bundle["IssuerInitialization"]
    issuer_values = json.loads(issuer_plan.read_text())
    pathlib.Path(issuer_values["root"]).parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    issuer_public = runtime / "issuer-public.json"
    issuer_public.write_bytes(invoke("initialize-issuer", binaries / "ardents-node", "issuer",
                                    "initialize", "--config", issuer_plan, timeout=90))
    issuer = provision["IssuerNode"]
    inspection = json.loads(invoke("inspect-issuer", binaries / "ardents-control",
                         "inspect-closed-issuer-profile", "--profile", issuer_public,
                         "--network", provision["NetworkID"], "--node", issuer["ID"],
                         "--node-key", issuer["PublicKey"]))
    # Dedicated fixture admission authority; no Permission/Custody journey claim.
    admission_key = work / "admission.pem"
    invoke("admission-key", "openssl", "genpkey", "-algorithm", "ED25519", "-out", admission_key)
    public_der = invoke("admission-public", "openssl", "pkey", "-in", admission_key,
                        "-pubout", "-outform", "DER")
    if len(public_der) != 44 or public_der[:12] != bytes.fromhex("302a300506032b6570032100"):
        raise RuntimeError("unexpected OpenSSL Ed25519 public encoding")
    profile = json.loads((runtime / "bundle" / bundle["ClosedProfilePlan"]).read_text())
    profile["IssuanceAuthorityKey"] = public_der[12:].hex()
    profile["TokenKeys"] = inspection["TokenKeys"]
    profile_path = write(runtime / "closed-profile.json", profile)
    signed_profile = runtime / "closed.profile"
    invoke("sign-profile", binaries / "ardents-control", "sign-closed-profile", "--plan", profile_path,
           "--authority-key", provision["AuthorityKey"], "--output", signed_profile)
    invoke("inspect-profile", binaries / "ardents-control", "inspect-closed-profile", "--plan", profile_path,
           "--profile", signed_profile, "--authority", provision["AuthorityPublic"], "--at", stamp)
    for index, owner in enumerate(owners):
        invoke("accept-profile-" + str(index), binaries / "ardents", "accept-closed-profile",
               "--state-root", owner["Root"], "--network-id", provision["NetworkID"],
               "--authorities", provision["AuthorityPublic"], "--threshold", "1", "--at", stamp,
               "--profile", "ardents-route-v3", "--closed-profile-authority", provision["AuthorityPublic"],
               "--closed-profile", signed_profile)
    policy = {"provider": "local monitoring fixture", "start": stamp,
              "end": (now + dt.timedelta(hours=6)).isoformat().replace("+00:00", "Z"),
              "unit": "GiB", "quantity": 1, "directions": "tx+rx", "interfaces": ["lo"],
              "low_watermark_bytes": 1 << 20}
    hosting_plan = write(runtime / "hosting-init.json", {"schema": "ardents-hosting-initialization-v1",
                              "root": str(runtime / "hosting"), "policy": policy})
    invoke("initialize-hosting", binaries / "ardents-node", "hosting", "initialize", "--config", hosting_plan)
    clock = spawn("clock", "python3", "/qualification/clock_observer.py", node_plan["clock_observation_file"])
    for index, relative in enumerate(bundle["SourcePlans"]):
        source = spawn("source-" + str(index), binaries / "ardents-node", "source", "--config",
                       runtime / "bundle" / relative)
        wait_ready("source-" + str(index), source)
    # Private producer evidence for this generated local fixture only. Never
    # send raw lifecycle identity/reason fields to monitoring backends.
    if not args.preview_seconds:
        diagnostic_directory = runtime / "node-diagnostic"
        diagnostic_directory.mkdir(mode=0o700)
        node_plan["diagnostic_directory"] = str(diagnostic_directory)
        write(node_path, node_plan)
    phase = "observe-node"
    monitor_directory = pathlib.Path("/events/monitor") if args.preview_seconds else runtime / "monitor"
    monitor_args = [binaries / "ardents-diagnostics", "monitor", "-out", monitor_directory,
                    "-console=false", "-listen", "127.0.0.1:8094", "-sample-max-age", "2s",
                    "-segment-bytes", "16777216" if args.persistent_journal else "16384",
                    "-retain-bytes", "1073741824" if args.persistent_journal else "2097152",
                    "-retain-files", "65" if args.persistent_journal else "128",
                    "-rotate-after", "15m" if args.persistent_journal else ("30s" if args.preview_seconds else "1s"),
                    "-retain-for", "72h" if args.persistent_journal else "30m"]
    if args.preview_seconds:
        pin = pathlib.Path("/metrics-certs/client-pin.txt").read_text().strip()
        monitor_args += ["-metrics-listen", "0.0.0.0:9101", "-metrics-certs", "/metrics-certs",
                         "-metrics-client-pin", pin, "-container"]
    else:
        monitor_args += ["-raw"]
    node_command = [binaries / "ardents-node", "node", "--config", node_path]
    if args.debug_profiles:
        debug_directory = runtime / "debug"
        debug_directory.mkdir(mode=0o700)
        debug_socket = debug_directory / "node.sock"
        node_command = ["env", "ARDENTS_DEBUG_SOCKET=" + str(debug_socket), *node_command]
    monitor_args += ["--", *node_command]
    monitor = spawn("monitor", *monitor_args)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    deadline = time.monotonic() + 15
    observed = False
    while time.monotonic() < deadline:
        if monitor.poll() is not None:
            raise RuntimeError("monitor/Node exited before resource observation")
        try:
            with opener.open("http://127.0.0.1:8094/metrics", timeout=1) as response:
                body = response.read((16 << 10) + 1)
            values = {line.split()[0]: float(line.split()[1]) for line in body.decode().splitlines()
                      if line and not line.startswith("#")}
            if values.get("diagnostic_selected_source_process_alive") == 0:
                raise RuntimeError("selected Node exited before resource observation")
            if values.get("diagnostic_selected_sample_fresh") == 1:
                assert values["diagnostic_selected_source_process_alive"] == 1
                assert values["diagnostic_selected_cgroup_memory_bytes"] > 0
                assert values["diagnostic_selected_cgroup_cpu_usage_seconds_total"] > 0
                assert "diagnostic_selected_process_go_memory_bytes" not in values
                assert "diagnostic_selected_process_goroutines" not in values
                assert "diagnostic_selected_process_fds" not in values
                assert "diagnostic_selected_cgroup_cpu_pressure_avg10_percent" not in values
                (reports / "actual-node-metrics.txt").write_bytes(body)
                observed = True
                break
        except (OSError, TimeoutError):
            pass
        time.sleep(0.1)
    if not observed:
        raise RuntimeError("accepted Node resource observation unavailable")
    if args.debug_profiles:
        phase = "private-node-profiles"
        pprof = shutil.which("pprof")
        trace = shutil.which("trace")
        if not pprof or not trace:
            raise RuntimeError("explicitly installed offline Go parsers required")
        profile_directory = runtime / "profiles"
        profile_directory.mkdir(mode=0o700)
        captures = []
        for kind in ("runtime", "cpu", "heap", "allocs", "goroutine", "block", "mutex", "trace"):
            target = profile_directory / (kind + (".json" if kind == "runtime" else ".out"))
            capture_args = ["snapshot", "-socket", debug_socket, "-kind", kind,
                            "-seconds", "2", "-out", target]
            if kind != "runtime":
                capture_args.append("-sensitive")
            capture_started = dt.datetime.now(dt.timezone.utc).isoformat()
            capture_clock = time.monotonic()
            invoke("capture-" + kind, binaries / "ardents-diagnostics", *capture_args, timeout=10)
            capture_completed = dt.datetime.now(dt.timezone.utc).isoformat()
            capture_elapsed = time.monotonic() - capture_clock
            if not 0 < target.stat().st_size <= 64 << 20:
                raise RuntimeError("private capture outside bound")
            if kind == "runtime":
                actual_runtime = json.loads(target.read_text())
                receipt["sampling"] = {"block_rate_ns": actual_runtime.get("block_sampling_rate_ns"),
                                       "mutex_fraction": actual_runtime.get("mutex_sampling_fraction"),
                                       "memory_rate_bytes": actual_runtime.get("memory_sampling_rate_bytes"),
                                       "scope": "process-wide while the explicit debug socket is open; block/mutex/allocs are cumulative, not two-second intervals"}
                if actual_runtime["goroutines"] <= 0 or actual_runtime["heap_alloc_bytes"] <= 0:
                    raise RuntimeError("actual Node runtime observation absent")
            elif kind == "trace":
                invoke("parse-trace", trace, "-pprof=sched", target, timeout=2)
            else:
                parsed = invoke("parse-" + kind, pprof, "-top", target, timeout=2)
                if kind == "cpu":
                    # Parser acceptance alone does not establish useful CPU samples.
                    total = re.search(rb"Total samples = ([0-9]+(?:\.[0-9]+)?)(?:[a-z]+)?(?:[ ,\r\n]|$)", parsed)
                    receipt["cpu_sampling"] = (
                        "empty" if total and float(total[1]) == 0 else
                        "observed" if total else "unavailable")
            with target.open("rb") as captured:
                digest = hashlib.file_digest(captured, "sha256").hexdigest()
            captures.append({"kind": kind, "bytes": target.stat().st_size, "sha256": digest,
                             "private_archive": "private-fixture.tar.gz",
                             "private_artifact": "owned-local-fixture/runtime/profiles/" + target.name,
                             "validation": "passed", "request_started_at": capture_started,
                             "request_completed_at": capture_completed, "request_elapsed_seconds": capture_elapsed,
                             "requested_duration_seconds": 2 if kind in ("cpu", "trace") else None})
        receipt["debug_profiles"] = captures
        receipt["profile_scope"] = "explicitly enabled owned Introduction Node process; no operation causality claim"
    if args.preview_seconds:
        write(reports / "live-source.json", {"actual_resource_observed": True, "carrier": args.carrier,
              "preview_seconds": args.preview_seconds, "scope": "one local shared container",
              "raw_capture": False, "backend_admission": False})
        phase = "operator-preview"
        deadline = time.monotonic() + args.preview_seconds
        while not preview_stop and time.monotonic() < deadline:
            if monitor.poll() is not None:
                raise RuntimeError("selected monitor exited during preview")
            time.sleep(1)
    else:
        time.sleep(3)
    state = json.loads((monitor_directory / "monitor.json").read_text())
    assert state["total_samples"] >= 2
    assert all(row["entry"]["kind"] != "resource-sample" for row in state["tail"])
    assert 2 < state["logs"]["files"] <= 128
    assert state["logs"]["retained_bytes"] <= 2097152
    receipt.update({"resource_samples": state["total_samples"], "safe_event_rows": state["total_rows"],
                    "source_alive": state["source_alive"], "actual_resource_observed": True,
                    "accepted_owners": len(owners)})
    receipt["complete"] = True
finally:
    outcomes = []
    for label, process in reversed(processes):
        if process.poll() is None:
            process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)
            receipt["complete"] = False
        outcomes.append({"role": label, "exit": process.returncode})
    receipt["process_outcomes"] = outcomes
    cleanup_failures = []
    for outcome in outcomes:
        expected = 1 if outcome["role"] == "monitor" else 0
        if outcome["exit"] != expected:
            cleanup_failures.append("unexpected " + outcome["role"] + " exit")
    if work and "monitor_directory" in globals() and (monitor_directory / "monitor.json").exists():
        final = json.loads((monitor_directory / "monitor.json").read_text())
        for field in ("cleanup_failed", "cleanup_in_progress", "file_failed", "snapshot_failed",
                      "retention_failed", "queue_dropped_bytes", "console_dropped_bytes"):
            if final[field]:
                cleanup_failures.append(field)
        if not final["sinks_joined"] or final["source_alive"] or final.get("source_exit") != 0:
            cleanup_failures.append("owned source/sinks did not finish normally")
        if final["logs"]["lost_bytes"]:
            cleanup_failures.append("retained log loss")
        receipt["shutdown_verified"] = not cleanup_failures
        receipt["final_safe_event_rows"] = final["total_rows"]
        receipt["retained_log_files"] = final["logs"]["files"]
        receipt["retained_log_bytes"] = final["logs"]["retained_bytes"]
    receipt["cleanup_failures"] = cleanup_failures
    if cleanup_failures:
        receipt["complete"] = False
    receipt["last_phase"] = phase
    if work:
        total = sum(p.stat().st_size for p in work.rglob("*") if p.is_file())
        receipt["private_work_bytes"] = total
        if total <= 64 << 20:
            with tarfile.open(reports / "private-fixture.tar.gz", "w:gz") as archive:
                archive.add(work, arcname="owned-local-fixture")
            if args.debug_profiles:
                try:
                    with tarfile.open(reports / "private-fixture.tar.gz", "r:gz") as archive:
                        for captured in receipt.get("debug_profiles", []):
                            member = archive.getmember(captured["private_artifact"])
                            if not member.isfile() or member.size != captured["bytes"]:
                                raise RuntimeError("saved private profile reference mismatch")
                            with archive.extractfile(member) as source:
                                if hashlib.file_digest(source, "sha256").hexdigest() != captured["sha256"]:
                                    raise RuntimeError("saved private profile bytes changed")
                    receipt["profile_archive_verified"] = True
                except (OSError, KeyError, RuntimeError, tarfile.TarError):
                    receipt["complete"] = False
                    receipt["profile_archive_verified"] = False
                    receipt["cleanup_failures"].append("private profile archive verification failed")
        else:
            receipt["complete"] = False
    (reports / "runtime-receipt.json").write_text(json.dumps(receipt, indent=2))

if not receipt["complete"]:
    raise RuntimeError("accepted Node probe did not complete; retain receipt")
