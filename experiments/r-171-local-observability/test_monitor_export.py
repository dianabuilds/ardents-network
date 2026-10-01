"""Linux file-boundary and actual HTTP lifecycle checks."""
import datetime
import http.client
import json
import os
import pathlib
import selectors
import subprocess
import sys
import tempfile
import time
import unittest

import monitor_export


class ExporterTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.directory.name)
        self.descriptor = monitor_export.selected_directory(str(self.root))

    def tearDown(self):
        os.close(self.descriptor)
        self.directory.cleanup()

    def write(self, body):
        path = self.root / "monitor.json"
        path.write_bytes(body)
        path.chmod(0o600)

    def test_file_failures_and_symlink_refuse(self):
        with self.assertRaises(OSError):
            monitor_export.snapshot(self.descriptor)
        self.write(b"x"*65537)
        with self.assertRaises(ValueError):
            monitor_export.snapshot(self.descriptor)
        (self.root/"monitor.json").unlink()
        (self.root/"private").write_text("PRIVATE_SENTINEL")
        (self.root/"monitor.json").symlink_to("private")
        with self.assertRaises(OSError):
            monitor_export.snapshot(self.descriptor)

    def test_fifo_refuses_without_blocking(self):
        os.mkfifo(self.root/"monitor.json", 0o600)
        before = time.monotonic()
        with self.assertRaises(ValueError):
            monitor_export.snapshot(self.descriptor)
        self.assertLess(time.monotonic()-before, 1)

    def test_non_private_file_and_directory_refuse(self):
        self.write(b"{}")
        (self.root/"monitor.json").chmod(0o644)
        with self.assertRaises(ValueError):
            monitor_export.snapshot(self.descriptor)
        self.root.chmod(0o755)
        with self.assertRaises(ValueError):
            monitor_export.selected_directory(str(self.root))
        self.root.chmod(0o700)

    def test_live_existing_monitor_supervisor_and_failed_source(self):
        root = self.root/"live-monitor"
        monitor = subprocess.Popen(
            ["ardents-diagnostics", "monitor", "-name", "selected-process",
             "-out", str(root), "-timeout", "6s", "-console=false",
             "--", "/bin/sh", "-c", "sleep 2; exit 7"],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        exporter = None
        try:
            deadline = time.monotonic()+4
            while not (root/"monitor.json").exists():
                self.assertLess(time.monotonic(), deadline,
                                "existing monitor produced no snapshot")
                time.sleep(0.05)
            exporter = subprocess.Popen(
                [sys.executable, "-B", "monitor_export.py", "--snapshot-dir", str(root),
                 "--sample-max-age", "5", "--port", "0", "--duration", "4"],
                stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            ready = selectors.DefaultSelector()
            ready.register(exporter.stdout, selectors.EVENT_READ)
            self.assertTrue(ready.select(3), "live exporter did not become ready")
            port = json.loads(exporter.stdout.readline())["port"]
            ready.close()

            def metrics():
                connection = http.client.HTTPConnection("127.0.0.1", port, timeout=2)
                try:
                    connection.request("GET", "/metrics")
                    response = connection.getresponse()
                    self.assertEqual(response.status, 200)
                    return response.read()
                finally:
                    connection.close()

            body = metrics()
            self.assertIn(b"diagnostic_selected_source_process_alive 1\n", body)
            self.assertIn(b"diagnostic_selected_sample_fresh 0\n", body)
            self.assertNotIn(b"cgroup_cpu_usage_seconds_total", body)
            self.assertNotEqual(monitor.wait(timeout=4), 0)
            state = json.loads((root/"monitor.json").read_bytes())
            self.assertEqual(state["source_exit"], 7)
            self.assertTrue(state["sinks_joined"])
            self.assertFalse(state["source_alive"])
            self.assertIn(b"diagnostic_selected_source_process_alive 0\n", metrics())
            self.assertEqual(exporter.wait(timeout=5), 0)
            self.assertEqual(exporter.stderr.read(), b"")
        finally:
            for process in (monitor, exporter):
                if process is not None:
                    if process.poll() is None:
                        process.kill()
                        process.wait(timeout=2)
                    process.stdout.close()
                    process.stderr.close()

    def test_actual_http_missing_stale_stopped_and_shutdown(self):
        process = subprocess.Popen(
            [sys.executable, "-B", "monitor_export.py", "--snapshot-dir", str(self.root),
             "--sample-max-age", "5", "--port", "0", "--duration", "3"],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            ready = selectors.DefaultSelector()
            ready.register(process.stdout, selectors.EVENT_READ)
            self.assertTrue(ready.select(5), "exporter failed to announce readiness")
            port = json.loads(process.stdout.readline())["port"]
            ready.close()

            def get(path):
                connection = http.client.HTTPConnection("127.0.0.1", port, timeout=2)
                try:
                    connection.request("GET", path)
                    response = connection.getresponse()
                    return response.status, response.read()
                finally:
                    connection.close()

            status, body = get("/metrics")
            self.assertEqual(status, 503)
            self.assertNotIn(b"PRIVATE_SENTINEL", body)
            self.assertEqual(get("/monitor.json")[0], 404)
            now = datetime.datetime.now(datetime.timezone.utc)
            state = {"schema": "ardents-monitor-v1",
                     "started": (now-datetime.timedelta(seconds=30)).isoformat(),
                     "updated": (now-datetime.timedelta(seconds=10)).isoformat(),
                     "source_alive": True, "source_name": "PRIVATE_SENTINEL"}
            self.write(json.dumps(state).encode())
            status, body = get("/metrics")
            self.assertEqual(status, 200)
            self.assertIn(b"diagnostic_selected_monitor_fresh 0\n", body)
            self.assertNotIn(b"source_process_alive", body)
            state["updated"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
            state["source_alive"] = False
            self.write(json.dumps(state).encode())
            status, body = get("/metrics")
            self.assertEqual(status, 200)
            self.assertIn(b"diagnostic_selected_source_process_alive 0\n", body)
            self.assertNotIn(b"PRIVATE_SENTINEL", body)
            # A partially sent request cannot keep the finite capture alive.
            connection = http.client.HTTPConnection("127.0.0.1", port, timeout=2)
            connection.connect()
            connection.sock.sendall(b"GET /metrics HTTP/1.1\r\n")
            self.assertEqual(process.wait(timeout=5), 0)
            connection.close()
            self.assertEqual(process.stderr.read(), b"")
        finally:
            if process.poll() is None:
                process.kill()
                process.wait(timeout=2)
            process.stdout.close()
            process.stderr.close()


if __name__ == "__main__":
    unittest.main()