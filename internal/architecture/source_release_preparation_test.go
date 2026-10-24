package architecture

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSourceReleasePreparationTimestampPreservesSafetyHorizon(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	// Exercise the actual public preparer's time rule without manufacturing
	// build records or treating this isolated validation as signed authority.
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import runpy, sys
from datetime import datetime, timedelta, timezone
check = runpy.run_path(sys.argv[1])["check_time_bounds"]
now = datetime(2026, 10, 24, tzinfo=timezone.utc)
short, renewed = now + timedelta(minutes=10), now + timedelta(minutes=30)
no_new, terminate = now + timedelta(hours=2), now + timedelta(hours=3)
for expires in (short, renewed):
    check(now, expires, no_new, terminate, timestamp_only=True)
    for purpose in ("initial", "forward"):
        try:
            check(now, expires, no_new, terminate, timestamp_only=False)
        except RuntimeError:
            pass
        else:
            raise AssertionError(purpose + " admitted metadata shorter than artifact safety")
for timestamp_only in (False, True):
    check(now, terminate, no_new, terminate, timestamp_only=timestamp_only)
    check(now, now + timedelta(days=7), no_new, terminate, timestamp_only=timestamp_only)
    for expires, stop_new, stop_all in (
        (now, no_new, terminate),
        (now - timedelta(seconds=1), no_new, terminate),
        (now + timedelta(days=7, seconds=1), no_new, terminate),
        (short, now, terminate),
        (short, no_new, no_new),
        (short, terminate, no_new),
    ):
        try:
            check(now, expires, stop_new, stop_all, timestamp_only=timestamp_only)
        except RuntimeError:
            pass
        else:
            raise AssertionError("invalid finite/current bounds admitted")
`, filepath.Join(repositoryRoot(t), "scripts", "installation", "prepare-source-release-plan.py"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("public Source preparation bounds: %v\n%s", err, output)
	}
}
