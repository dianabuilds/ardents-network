#!/usr/bin/env python3
"""Selected private Source startup qualification after direct PO acceptance.

Do not invoke this profile before the completed exact public bootstrap receipt
has been directly accepted by the Product Owner. Recording its pin here is
cohort identification, never technical evidence of that human decision or a
runtime permission. Product commands obtain genuine fresh owner admission.

The outside controller owns the private rootfs/manager and original whole-scope
events descriptor, places this caller and children in its supervisor cgroup,
retains the development lock, and joins every descendant before returning it.
Missing prerequisites fail. No host-wide changes, reset, retry/adoption or
Source protection override is admitted. Each selected case needs a fresh stand.
"""
import base64
import errno
import hashlib
import json
import os
from pathlib import Path
import platform
import signal
import stat
import subprocess
import sys
import threading
import time


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def pinned(name):
    prefix = "ARDENTS_SOURCE_STARTUP_" + name
    path = Path(os.environ[prefix])
    wanted = os.environ[prefix + "_SHA256"]
    if not path.is_absolute() or path.is_symlink() or not path.is_file() or len(wanted) != 64 or digest(path) != wanted:
        raise RuntimeError("original pinned profile input unavailable: " + name)
    return path


def main():
    if sys.argv[1:] != ["-timeout=3m"] or not sys.flags.isolated or not sys.flags.no_site or sys.version_info[:2] != (3, 12):
        raise RuntimeError("fixed isolated Python3.12 profile required")
    if sys.platform != "linux" or os.geteuid() != 0:
        raise RuntimeError("private native Linux root controller required")
    release = platform.freedesktop_os_release()
    if (release.get("ID"), release.get("VERSION_ID")) != ("ubuntu", "24.04"):
        raise RuntimeError("selected Ubuntu24.04 profile required")
    if Path("/etc/ardents-source-startup-native-stand").read_text() != "ardents-source-startup-native-v1\n" or Path("/run/systemd/container").read_text().strip() != "ardents-source-startup":
        raise RuntimeError("original private Source startup manager required")
    for name in ("mnt", "pid", "cgroup", "uts"):
        a, b = Path("/proc/1/ns/" + name).stat(), Path("/proc/self/ns/" + name).stat()
        if (a.st_dev, a.st_ino) != (b.st_dev, b.st_ino):
            raise RuntimeError("original caller/manager namespaces differ")
    if Path("/proc/self/cgroup").read_text() != "0::/../supervisor\n":
        raise RuntimeError("caller must belong to the outside original stand supervisor scope")
    probe = os.pidfd_open(os.getpid(), 0)
    try:
        signal.pidfd_send_signal(probe, 0)
    finally:
        os.close(probe)
    case = os.environ["ARDENTS_SOURCE_STARTUP_CASE"]
    if case not in ("positive", "credential-substitution", "clock-loss"):
        raise RuntimeError("unknown finite Source startup case")
    root = Path(__file__).resolve().parents[2]
    manifest = json.loads(pinned("SOURCE_MANIFEST").read_text())["files"]

    def source():
        names = {p.relative_to(root).as_posix() for p in root.rglob("*") if p.is_file()}
        if not manifest or names != set(manifest):
            raise RuntimeError("complete original source snapshot required")
        for name, wanted in manifest.items():
            path = root / name
            if Path(name).is_absolute() or ".." in Path(name).parts or path.is_symlink() or digest(path) != wanted:
                raise RuntimeError("source differs from actual candidate input")

    source()
    paths = {name: pinned(name) for name in ("NODE", "CONTROL", "MANAGER_TEST", "REQUEST", "BUNDLE", "EPOCH", "UNSIGNED_EPOCH", "CLIENT_CERTIFICATE", "CLIENT_KEY", "SERVER_CA", "PUBLIC_RECEIPT")}
    distribution = Path(os.environ["ARDENTS_SOURCE_STARTUP_DISTRIBUTION"])
    if paths["NODE"] != distribution / "ardents-node-linux-amd64":
        raise RuntimeError("execute the actual enrolled initial program")
    request = json.loads(paths["REQUEST"].read_text())
    binding = Path(request["installation_root"]) / "binding.json"
    history = os.environ["ARDENTS_SOURCE_STARTUP_HISTORY"]
    network = os.environ["ARDENTS_SOURCE_STARTUP_NETWORK"]
    output = Path(os.environ["ARDENTS_SOURCE_STARTUP_OUTPUT"])
    if not output.is_absolute() or output.exists() or output.is_symlink() or output.is_relative_to(root):
        raise RuntimeError("fresh private evidence output required")
    os.umask(0o077)
    output.mkdir(mode=0o700)
    interrupted = threading.Event()
    for kind in (signal.SIGTERM, signal.SIGINT):
        signal.signal(kind, lambda *_: interrupted.set())
    started = time.monotonic()
    clock_stop = threading.Event()
    clock_errors = []
    clock_thread = None
    clock_fd = None
    scope_fd = None
    source_pid = None
    source_pidfd = None
    source_started = False
    failures = []
    outcome = {"case": case, "installed_qualification": False, "original_scope_joined": False,
               "public_receipt_sha256": os.environ["ARDENTS_SOURCE_STARTUP_PUBLIC_RECEIPT_SHA256"],
               "source_manifest_sha256": os.environ["ARDENTS_SOURCE_STARTUP_SOURCE_MANIFEST_SHA256"]}

    def observe():
        source()
        for name, path in paths.items():
            if pinned(name) != path:
                raise RuntimeError("original profile declaration changed")
        if interrupted.is_set() or clock_errors or time.monotonic() - started >= 180:
            raise RuntimeError("original caller/clock/profile bound unavailable")

    def run(argv, name, refusal=False, **options):
        observe()
        # Direct children finish through their actual product owner cleanup.
        # The outside original scope still owns interruption/descendant join.
        child_failures = []
        with (output / (name + ".log")).open("xb") as log:
            child = subprocess.Popen([str(a) for a in argv], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, **options)
            drain_errors = []

            def drain():
                total = 0
                try:
                    while True:
                        raw = child.stdout.read(65536)
                        if not raw:
                            break
                        total += len(raw)
                        if total > 32 << 20:
                            if not drain_errors:
                                drain_errors.append(RuntimeError("original product output exceeds finite evidence bound"))
                        elif not drain_errors:
                            try:
                                log.write(raw)
                            except OSError as error:
                                drain_errors.append(error)
                except BaseException as error:
                    drain_errors.append(error)
                finally:
                    try:
                        child.stdout.close()
                    except OSError as error:
                        drain_errors.append(error)

            reader = threading.Thread(target=drain)
            signalled, notice = False, False
            try:
                reader.start()
                while child.poll() is None:
                    if not signalled and (interrupted.is_set() or clock_errors or drain_errors or time.monotonic() - started >= 180):
                        # Signal only the original unreaped direct child. Its
                        # owner joins original work before this actual return.
                        signalled = True
                        child.send_signal(signal.SIGTERM)
                    if not notice and time.monotonic() - started >= 180:
                        print("Original product completion unavailable; retaining child and stand lock", file=sys.stderr)
                        notice = True
                    time.sleep(0.02)
            except BaseException as error:
                child_failures.append(error)
            finally:
                if child.poll() is None:
                    try:
                        child.send_signal(signal.SIGTERM)
                    except OSError as error:
                        child_failures.append(error)
                returncode = child.wait()  # Never abandon the original child.
                if reader.ident is not None:
                    while reader.is_alive():
                        reader.join(0.1)
                        if not notice and time.monotonic() - started >= 180:
                            print("Original output completion unavailable; retaining reader and stand lock", file=sys.stderr)
                            notice = True
                else:
                    child.stdout.close()
                child_failures.extend(drain_errors)
        try:
            observe()
        except BaseException as error:
            child_failures.append(error)
        if returncode != 0 and not refusal:
            child_failures.append(RuntimeError("original product operation failed: " + name))
        if child_failures:
            raise BaseExceptionGroup("original product operation/observation failed", child_failures)
        raw = (output / (name + ".log")).read_bytes()
        return (returncode, raw) if refusal else raw

    def manager_owner():
        return subprocess.check_output(["/usr/bin/busctl", "--timeout=3s", "--system", "call", "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "GetNameOwner", "s", "org.freedesktop.systemd1"])

    original_manager = manager_owner()
    if not original_manager.startswith(b's ":1.') or not subprocess.check_output(["/usr/bin/systemctl", "show", "-p", "Version", "--value"]).startswith(b"255."):
        raise RuntimeError("selected original manager unavailable")

    def original_join():
        # Only this retained kernel object supplies the Source join observation.
        try:
            os.lseek(scope_fd, 0, os.SEEK_SET)
            body = os.read(scope_fd, 1025)
            if len(body) > 1024:
                raise RuntimeError("original scope observation exceeds bound")
            fields = dict(line.split() for line in body.decode("ascii").splitlines())
            if fields.get("populated") not in ("0", "1") or fields.get("frozen") not in ("0", "1"):
                raise RuntimeError("original kernel scope observation is invalid")
            return fields["populated"] == "0"
        except OSError as error:
            if error.errno != errno.ENODEV:
                raise
            return True

    try:
        manager_tests = ("TestSourceControlRetainsStoppedUnitNative", "TestSourceCanceledObservationCloseNative")
        discovered = run([paths["MANAGER_TEST"], "-test.list=^(" + "|".join(manager_tests) + ")$"], "manager-test-discovery")
        if sorted(discovered.splitlines()) != sorted(name.encode("ascii") for name in manager_tests):
            raise RuntimeError("required native manager control is absent")
        clock = Path(request["clock"])
        clock_fd = os.open(clock, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
        clock_info = os.fstat(clock_fd)
        if not stat.S_ISREG(clock_info.st_mode) or clock_info.st_uid != 0 or stat.S_IMODE(clock_info.st_mode) != 0o644:
            raise RuntimeError("original root clock writer/access required")
        os.utime(clock_fd, None)

        def clock_writer():
            while not clock_stop.is_set():
                try:
                    current = clock.stat()
                    if (current.st_dev, current.st_ino) != (clock_info.st_dev, clock_info.st_ino):
                        raise RuntimeError("original clock writer inode changed")
                    os.utime(clock_fd, None)
                except BaseException as error:
                    clock_errors.append(error)
                    return
                clock_stop.wait(0.2)

        clock_thread = threading.Thread(target=clock_writer)
        clock_thread.start()
        # Offline State acceptance already needs a fresh independent clock;
        # keep this same original writer live during installation and preparation.
        run([paths["NODE"], "installation", "install-initial-stopped", distribution, os.environ["ARDENTS_SOURCE_STARTUP_PIN"], history, "alpha", network, paths["REQUEST"]], "initial-stopped")
        selected = json.loads(binding.read_text())
        uid, gid = selected["uid"], selected["gid"]
        if not 100 <= uid <= 999 or uid != gid:
            raise RuntimeError("actual fixed Source principal required")
        plan = Path(request["installation_root"]) / "generations" / selected["generation"] / "source-plan.json"
        # BUNDLE and CONTROL must have a separate root-controlled public
        # delivery traversable by this actual UID; private key directories stay
        # private. There is no fixture-root or root-user State substitution.
        run([paths["CONTROL"], "source", "prepare-state", plan, paths["BUNDLE"]], "state-preparation", user=uid, group=gid, extra_groups=[])

        # An inactive unit may have left the manager's in-memory inventory after
        # initial installation. Root control must acquire its own original
        # reference, not depend on an observer keeping that unit loaded.
        run([paths["MANAGER_TEST"], "-test.run=^TestSourceControlRetainsStoppedUnitNative$", "-test.v", "-test.timeout=30s"], "stopped-unit-reference")
        run([paths["MANAGER_TEST"], "-test.run=^TestSourceCanceledObservationCloseNative$", "-test.v", "-test.timeout=30s"], "canceled-manager-observation")

        # From here even a failed/uncertain start belongs to the outside own
        # stand completion obligation. The product joins its failed attempt.
        source_started = True
        raw = run([paths["NODE"], "installation", "start-source", distribution, history, "alpha", network, binding], "first-start")
        if json.loads(raw) != {"operation": "installation.source", "outcome": "first-start-completed"}:
            raise RuntimeError("foreign first-start completion projection")
        source_pid = int(subprocess.check_output(["/usr/bin/systemctl", "show", "ardents-source.service", "-p", "MainPID", "--value"]))
        if source_pid <= 1:
            raise RuntimeError("actual original non-root Source process missing")
        process_path = Path("/proc") / str(source_pid)
        process_info = process_path.stat()
        source_pidfd = os.pidfd_open(source_pid, 0)
        signal.pidfd_send_signal(source_pidfd, 0)
        current_process = process_path.stat()
        if (current_process.st_dev, current_process.st_ino) != (process_info.st_dev, process_info.st_ino):
            raise RuntimeError("original Source process changed during pidfd acquisition")
        expected = [str(plan.parent / "ardents-node-linux-amd64"), "source", "start-installed", str(binding)]
        if (Path("/proc") / str(source_pid) / "cmdline").read_bytes().split(b"\0") != [s.encode() for s in expected] + [b""]:
            raise RuntimeError("actual original Source argv differs")
        status = dict(line.split(":", 1) for line in (Path("/proc") / str(source_pid) / "status").read_text().splitlines() if ":" in line)
        if [int(value) for value in status["Uid"].split()] != [uid] * 4 or [int(value) for value in status["Gid"].split()] != [gid] * 4:
            raise RuntimeError("actual Source real/effective/saved/filesystem credentials differ")
        if any(int(value) != gid for value in status["Groups"].split()) or any(int(status[key].strip(), 16) for key in ("CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb")):
            raise RuntimeError("actual Source supplementary groups/capabilities differ")
        actual_program, selected_program = (Path("/proc") / str(source_pid) / "exe").stat(), (plan.parent / "ardents-node-linux-amd64").stat()
        if (actual_program.st_dev, actual_program.st_ino) != (selected_program.st_dev, selected_program.st_ino):
            raise RuntimeError("actual original Source executable differs")
        scope_fd = os.open("/sys/fs/cgroup/system.slice/ardents-source.service/cgroup.events", os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
        if os.fstat(scope_fd).st_dev != Path("/sys/fs/cgroup").stat().st_dev or original_join():
            raise RuntimeError("original active kernel Source scope unavailable")
        invocation = subprocess.check_output(["/usr/bin/systemctl", "show", "ardents-source.service", "-p", "InvocationID", "--value"]).decode().strip()
        if len(invocation) != 32 or any(c not in "0123456789abcdef" for c in invocation):
            raise RuntimeError("actual original invocation identity unavailable")
        ready = False
        while not ready:
            observe()
            if manager_owner() != original_manager:
                raise RuntimeError("original manager changed before runtime readiness")
            journal = subprocess.check_output(["/usr/bin/journalctl", "--no-pager", "--output=cat", "--lines=32", "--unit=ardents-source.service", "_SYSTEMD_INVOCATION_ID=" + invocation])
            if len(journal) > 64 << 10:
                raise RuntimeError("original readiness observation exceeds bound")
            for line in journal.splitlines():
                if line == b'{"operation":"source.start-installed","outcome":"listening"}':
                    ready = True
            if not ready:
                if original_join():
                    raise RuntimeError("original Source ended before genuine runtime readiness")
                time.sleep(0.05)
        inspect = [paths["CONTROL"], "source", "inspect-latest", plan, paths["CLIENT_CERTIFICATE"], paths["CLIENT_KEY"], paths["SERVER_CA"],
                   os.environ["ARDENTS_SOURCE_STARTUP_SERVER_NAME"], os.environ["ARDENTS_SOURCE_STARTUP_SERVER_PIN"], os.environ["ARDENTS_SOURCE_STARTUP_SOURCE_ID"],
                   os.environ["ARDENTS_SOURCE_STARTUP_FAMILY"], os.environ["ARDENTS_SOURCE_STARTUP_HANDLE"], "0"]
        response = json.loads(run(inspect, "actual-client"))
        expected_payload = base64.b64encode(paths["BUNDLE"].read_bytes()).decode("ascii")
        # AREP's protocol identity hashes the prepared unsigned envelope. The
        # signed file commitment includes its signer section and is different.
        # Exact bundle equality separately covers the complete signed payload.
        if response.get("operation") != "source.inspect-latest" or response.get("status") != "ok" or response.get("payload") != expected_payload or bytes(response["object_digest"]).hex() != digest(paths["UNSIGNED_EPOCH"]):
            raise RuntimeError("genuine Source response differs from accepted prepared State")
        outcome["response_epoch_sha256"] = digest(paths["EPOCH"])
        if case == "credential-substitution":
            certificate = Path(request["credential_directory"]) / "certificate.pem"
            data, info = certificate.read_bytes(), certificate.stat()
            certificate.rename(certificate.with_name("certificate.pem.original"))
            with certificate.open("xb") as stream:
                stream.write(data)
            os.chmod(certificate, stat.S_IMODE(info.st_mode))
            os.chown(certificate, info.st_uid, info.st_gid)
            # A new physical client request follows the completed inode change;
            # no second family, Plan retry or alternate source is introduced.
            refused, response_raw = run(inspect, "changed-input-client", refusal=True)
            if refused == 0:
                message = json.loads(response_raw)
                if message.get("status") == "ok":
                    raise RuntimeError("changed original credential admitted new response")
            outcome["changed_original_input_denied"] = True
        elif case == "clock-loss":
            clock_stop.set()
            clock_thread.join()
            os.utime(clock_fd, (1, 1))
            if os.fstat(clock_fd).st_mtime_ns != 1_000_000_000:
                raise RuntimeError("original clock loss was not established")
            # The current Source contract distributes retained signed bytes;
            # current membership/time authority is a separate State observation.
            # Neither inspection nor offline preparation establishes that right.
            after = json.loads(run(inspect, "stale-clock-retained-distribution"))
            if after != response:
                raise RuntimeError("retained Source distribution changed after clock loss")
            outcome["stale_clock_retained_distribution_observed"] = True
        observe()
    except BaseException as error:
        failures.append(error)
    finally:
        if source_started:
            try:
                if manager_owner() != original_manager:
                    raise RuntimeError("original manager lost; outside original stand retains completion custody")
                if source_pidfd is not None and scope_fd is not None:
                    # Signal the original process before asking the manager to
                    # deactivate its unit. Otherwise a legitimate active-state
                    # loss can race the ordinary signal and poison that oracle.
                    try:
                        signal.pidfd_send_signal(source_pidfd, signal.SIGTERM)
                    except ProcessLookupError:
                        if not original_join():
                            raise
                else:
                    # An uncertain first-start failure has no published original
                    # process/scope pair. The own fixed unit remains cleanup only.
                    subprocess.run(["/usr/bin/systemctl", "stop", "ardents-source.service"], check=True)
            except BaseException as error:
                failures.append(error)
        if scope_fd is not None:
            notice, join_error = False, None
            while True:
                try:
                    if original_join():
                        break
                except BaseException as error:
                    if join_error is None:
                        join_error = error
                if not notice and time.monotonic() - started >= 180:
                    print("Source original completion unavailable; retaining scope and outside lock", file=sys.stderr)
                    notice = True
                time.sleep(0.1)
            if join_error is not None:
                failures.append(join_error)
            outcome["original_scope_joined"] = True
            try:
                os.close(scope_fd)
            except OSError as error:
                failures.append(error)
        if source_started and source_pidfd is not None and scope_fd is not None:
            try:
                if manager_owner() != original_manager:
                    raise RuntimeError("original manager changed after Source join")
                subprocess.run(["/usr/bin/systemctl", "stop", "ardents-source.service"], check=True)
            except BaseException as error:
                failures.append(error)
        if source_pidfd is not None:
            try:
                os.close(source_pidfd)
            except OSError as error:
                failures.append(error)
        # State cleanup may still observe trusted time. Keep this original
        # writer alive until the actual Source scope joins, even on interruption.
        clock_stop.set()
        if clock_thread is not None:
            clock_thread.join()
        if clock_fd is not None:
            try:
                os.close(clock_fd)
            except OSError as error:
                failures.append(error)
    try:
        observe()
        if not outcome["original_scope_joined"]:
            raise RuntimeError("no original Source scope completion observation")
        # Normal return through the command's joined supervisor is different
        # from a manager timeout/SIGKILL. No clean physical result is inferred
        # from path disappearance or the Stop command's own return.
        code = subprocess.check_output(["/usr/bin/systemctl", "show", "ardents-source.service", "-p", "ExecMainCode", "--value"]).strip()
        status = subprocess.check_output(["/usr/bin/systemctl", "show", "ardents-source.service", "-p", "ExecMainStatus", "--value"]).strip()
        if code != b"1" or status not in (b"0", b"2"):
            raise RuntimeError("Source did not return through its actual command supervisor")
        terminal = None
        while terminal is None:
            observe()
            if manager_owner() != original_manager:
                raise RuntimeError("original manager changed before retirement observation")
            journal = subprocess.check_output(["/usr/bin/journalctl", "--no-pager", "--output=cat", "--lines=32", "--unit=ardents-source.service", "_SYSTEMD_INVOCATION_ID=" + invocation])
            if len(journal) > 64 << 10:
                raise RuntimeError("original retirement observation exceeds bound")
            lines = journal.splitlines()
            if b'{"operation":"source.start-installed","outcome":"completion-unavailable"}' in lines:
                raise RuntimeError("original completion was unavailable before final join")
            reports = [value for value in ("retired", "retirement-failed")
                       if ('{"operation":"source.start-installed","outcome":"' + value + '"}').encode() in lines]
            if len(reports) > 1:
                raise RuntimeError("conflicting original retirement reports")
            if reports:
                terminal = reports[0]
            else:
                time.sleep(0.05)
        outcome["retirement_outcome"] = terminal
        outcome["clean_retirement"] = terminal == "retired"
        if case == "positive" and not outcome["clean_retirement"]:
            raise RuntimeError("positive Source retirement retained an original failure")
        # Negative cases prove causal refusal and original scope join. A failed
        # terminal report remains failed evidence, never clean cleanup or full
        # installed qualification. The positive retirement gate is separate.
    except BaseException as error:
        failures.append(error)
    outcome["case_passed"] = not failures
    try:
        (output / "private-outcome.json").write_text(json.dumps(outcome, sort_keys=True) + "\n")
    except OSError as error:
        failures.append(error)
    if failures:
        raise BaseExceptionGroup("original Source qualification/cleanup failed", failures)
    print(json.dumps({"operation": "source.startup-qualification", **outcome}, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError, BaseExceptionGroup) as error:
        print("Source startup profile failed/invalid:", error, file=sys.stderr)
        sys.exit(2)
