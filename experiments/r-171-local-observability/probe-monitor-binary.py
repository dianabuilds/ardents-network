"""Bounded actual binary/TLS refusal probe; no healthy Node qualification claim."""
import hashlib
import json
import pathlib
import signal
import ssl
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

reports = pathlib.Path("/reports")
receipt = {"complete": False, "source_scope": "actual ardents-node with explicitly missing plan",
           "qualification": False}
monitor = None

def command(*args, input=None):
    return subprocess.run(args, input=input, stdout=subprocess.PIPE,
                          stderr=subprocess.PIPE, timeout=5, check=True).stdout

def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()

try:
    binaries = pathlib.Path("/binaries")
    manifest_path = binaries / "build-receipt.json"
    if manifest_path.stat().st_size > 1 << 20:
        raise RuntimeError("build receipt outside bound")
    manifest = json.loads(manifest_path.read_text(encoding="utf-8-sig"))
    if not manifest.get("complete") or manifest.get("exit_code") != 0:
        raise RuntimeError("explicit complete build required")
    artifacts = manifest.get("artifacts", [])
    if {a["name"] for a in artifacts} != {"ardents-diagnostics", "ardents-node"}:
        raise RuntimeError("unexpected selected build artifacts")
    for artifact in artifacts:
        path = binaries / artifact["name"]
        if not 0 < path.stat().st_size <= 64 << 20 or digest(path) != artifact["sha256"]:
            raise RuntimeError("selected binary changed")
    receipt["artifacts"] = artifacts
    help_result = subprocess.run([str(binaries / "ardents-diagnostics"), "monitor", "-h"],
                                 capture_output=True, timeout=5)
    if help_result.returncode != 1 or any(
            flag not in help_result.stderr for flag in
            (b"-metrics-listen", b"-metrics-certs", b"-metrics-client-pin")):
        raise RuntimeError("rebuilt metrics CLI unavailable")

    with tempfile.TemporaryDirectory(prefix="selected-monitor-", dir="/tmp") as directory:
        root = pathlib.Path(directory)
        certs = root / "certs"
        certs.mkdir(mode=0o700)
        ca_key, ca_cert = root / "ca.key", root / "ca.crt"
        command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
                "-subj", "/CN=Private diagnostic probe CA", "-keyout", str(ca_key),
                "-out", str(ca_cert))
        for serial, role in enumerate(("server", "client"), 2):
            key, csr, cert = root / (role+".key"), root / (role+".csr"), root / (role+".crt")
            ext = root / (role+".ext")
            ext.write_text("subjectAltName=IP:127.0.0.1\nextendedKeyUsage=" +
                           ("serverAuth" if role == "server" else "clientAuth") + "\n")
            command("openssl", "req", "-new", "-newkey", "rsa:2048", "-nodes",
                    "-subj", "/CN=Selected diagnostic " + role, "-keyout", str(key),
                    "-out", str(csr))
            command("openssl", "x509", "-req", "-days", "1", "-in", str(csr),
                    "-CA", str(ca_cert), "-CAkey", str(ca_key), "-set_serial", str(serial),
                    "-extfile", str(ext), "-out", str(cert))
        for name, source in (("server.key", root / "server.key"),
                             ("server.crt", root / "server.crt"), ("client-ca.crt", ca_cert)):
            (certs / name).write_bytes(source.read_bytes())
            (certs / name).chmod(0o600)
        public = command("openssl", "x509", "-in", str(root / "client.crt"), "-pubkey", "-noout")
        spki = command("openssl", "pkey", "-pubin", "-outform", "DER", input=public)
        pin = hashlib.sha256(spki).hexdigest()
        monitor = subprocess.Popen([
            str(binaries / "ardents-diagnostics"), "monitor", "-out", str(root / "monitor"),
            "-console=false", "-metrics-listen", "127.0.0.1:9443", "-metrics-certs", str(certs),
            "-metrics-client-pin", pin, "-sample-max-age", "1s", "--",
            str(binaries / "ardents-node"), "node", "--config", str(root / "missing-plan.json")],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        context = ssl.create_default_context(cafile=str(ca_cert))
        context.minimum_version = ssl.TLSVersion.TLSv1_3
        context.load_cert_chain(str(root / "client.crt"), str(root / "client.key"))
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}),
                                             urllib.request.HTTPSHandler(context=context))
        deadline = time.monotonic() + 10
        observed = False
        while time.monotonic() < deadline:
            if monitor.poll() is not None:
                raise RuntimeError("monitor failed before terminal metrics")
            try:
                with opener.open("https://127.0.0.1:9443/metrics", timeout=1) as response:
                    body = response.read((16 << 10) + 1)
                if len(body) > 16 << 10:
                    raise RuntimeError("metric response outside bound")
                values = {}
                for line in body.decode().splitlines():
                    if line and not line.startswith("#"):
                        name, value = line.split()
                        values[name] = float(value)
                if values.get("diagnostic_selected_source_process_alive") == 0:
                    assert values["diagnostic_selected_monitor_fresh"] == 1
                    assert values["diagnostic_selected_sample_fresh"] == 0
                    assert not any(name.startswith("diagnostic_selected_process_go") for name in values)
                    assert not any(name.startswith("diagnostic_selected_cgroup") for name in values)
                    (reports / "terminal-metrics.txt").write_bytes(body)
                    observed = True
                    break
            except (urllib.error.URLError, TimeoutError):
                pass
            time.sleep(0.05)
        if not observed:
            raise RuntimeError("actual Node refusal missing from live TLS metrics")
        monitor.send_signal(signal.SIGTERM)
        stdout, stderr = monitor.communicate(timeout=10)
        if monitor.returncode != 1:
            raise RuntimeError("monitor erased selected Node failure")
        state = json.loads((root / "monitor" / "monitor.json").read_text())
        assert state["source_exit"] == 2 and state["source_alive"] is False
        assert state["sinks_joined"] and not state["cleanup_failed"]
        assert not state.get("latest_sample") and state["total_samples"] == 0
        receipt.update({"source_exit": state["source_exit"], "monitor_exit": monitor.returncode,
                        "sinks_joined": state["sinks_joined"], "resource_samples": state["total_samples"],
                        "tls_terminal_observed": True, "complete": True})
finally:
    if monitor is not None and monitor.poll() is None:
        monitor.kill()
        monitor.communicate(timeout=5)
    (reports / "runtime-receipt.json").write_text(json.dumps(receipt, indent=2))
