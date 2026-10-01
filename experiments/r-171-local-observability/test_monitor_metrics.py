"""Behavior checks for scope, unavailable fields, replay and sensitive exclusion."""
import json
import subprocess
import sys
import unittest
import monitor_metrics


def snapshot():
    return {"schema": "ardents-monitor-v1", "started": "2026-10-01T00:00:00Z",
            "updated": "2026-10-01T00:00:10Z", "source_alive": True,
            "source_name": "PRIVATE_SENTINEL", "source_pid": 987654,
            "latest_sample": {"at": "2026-10-01T00:00:10Z",
                "entry": {"schema": "ardents-node-event-v1", "kind": "resource-sample",
                    "at": "2026-10-01T00:00:10Z",
                    "resource": {"cpu_usage_usec": 2000000, "memory_bytes": 4096,
                                 "rss_bytes": 0, "admission_active": 0,
                                 "queue_items": 0, "fds": 0}}},
            "tail": [{"payload": "PRIVATE_SENTINEL"}]}


NOW = monitor_metrics.timestamp("2026-10-01T00:00:11Z")


class ProjectionTests(unittest.TestCase):
    def test_scope_units_and_unpopulated_exclusion(self):
        result = monitor_metrics.project(snapshot(), NOW, 5)
        self.assertIn("diagnostic_selected_cgroup_cpu_usage_seconds_total 2\n", result)
        self.assertIn("diagnostic_selected_cgroup_memory_bytes 4096\n", result)
        self.assertIn("diagnostic_selected_process_fds 0\n", result)
        for absent in ("rss", "admission", "queue_items", "PRIVATE_SENTINEL", "987654"):
            self.assertNotIn(absent, result)

    def test_stale_supervisor_does_not_repeat_resources_or_alive(self):
        result = monitor_metrics.project(snapshot(), NOW+10, 30)
        self.assertIn("diagnostic_selected_monitor_fresh 0\n", result)
        self.assertNotIn("diagnostic_selected_source_process_alive", result)
        self.assertNotIn("diagnostic_selected_cgroup_memory_bytes", result)

    def test_fresh_receipt_does_not_make_old_sample_current(self):
        state = snapshot()
        state["latest_sample"]["entry"]["at"] = "2026-10-01T00:00:01Z"
        result = monitor_metrics.project(state, NOW, 5)
        self.assertIn("diagnostic_selected_sample_fresh 0\n", result)
        self.assertNotIn("diagnostic_selected_cgroup_memory_bytes", result)

    def test_stopped_process_drops_resource_values(self):
        state = snapshot()
        state["source_alive"] = False
        result = monitor_metrics.project(state, NOW, 5)
        self.assertIn("diagnostic_selected_source_process_alive 0\n", result)
        self.assertNotIn("diagnostic_selected_cgroup_memory_bytes", result)

    def test_missing_field_is_absent_not_zero(self):
        state = snapshot()
        del state["latest_sample"]["entry"]["resource"]["fds"]
        self.assertNotIn("diagnostic_selected_process_fds",
                         monitor_metrics.project(state, NOW, 5))

    def test_invalid_values_and_future_time_refuse(self):
        for value in (True, -1, float("nan"), float("inf"), "0"):
            state = snapshot()
            state["latest_sample"]["entry"]["resource"]["fds"] = value
            with self.assertRaises(ValueError):
                monitor_metrics.project(state, NOW, 5)
        state = snapshot()
        state["updated"] = "2026-10-01T00:00:20Z"
        with self.assertRaises(ValueError):
            monitor_metrics.project(state, NOW, 5)

    def run_cli(self, data):
        return subprocess.run(
            [sys.executable, "-B", "monitor_metrics.py", "--sample-max-age", "5"],
            input=data, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            timeout=5, check=False)

    def test_cli_rejects_oversize_without_echoing_input(self):
        result = self.run_cli(b"PRIVATE_SENTINEL" + b" " * 65536)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, b"")
        self.assertEqual(result.stderr,
                         b"Selected safe monitor snapshot unavailable or invalid\n")

    def test_cli_rejects_malformed_and_excessively_nested_input(self):
        for data in (b'{"secret":"PRIVATE_SENTINEL",', b"\xff",
                     b"["*2000+b"0"+b"]"*2000):
            result = self.run_cli(data)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(result.stdout, b"")
            self.assertEqual(result.stderr,
                             b"Selected safe monitor snapshot unavailable or invalid\n")

    def test_cli_stale_snapshot_does_not_echo_sensitive_fields(self):
        result = self.run_cli(json.dumps(snapshot()).encode())
        self.assertEqual(result.returncode, 0)
        self.assertEqual(result.stderr, b"")
        self.assertIn(b"diagnostic_selected_sample_fresh 0\n", result.stdout)
        self.assertNotIn(b"PRIVATE_SENTINEL", result.stdout)
        self.assertNotIn(b"cgroup_memory_bytes", result.stdout)

    def test_session_start_is_preserved_for_reset_detection(self):
        result = monitor_metrics.project(snapshot(), NOW, 5)
        self.assertIn("diagnostic_selected_session_started_seconds 1790812800\n", result)


if __name__ == "__main__":
    unittest.main()