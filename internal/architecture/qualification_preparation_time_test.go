package architecture

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestQualificationCanonicalInstantHelper(t *testing.T) {
	root := repositoryRoot(t)
	helper := filepath.Join(root, "tests", "qualification", "stream-network-two-host", "canonical-json-instant.ps1")
	pwsh := "pwsh"
	if runtime.GOOS == "windows" {
		pwsh = "pwsh.exe"
	}
	command := exec.Command(pwsh, "-NoProfile", "-Command",
		". $env:ARDENTS_CANONICAL_INSTANT_SCRIPT; $value = Read-CanonicalJSONInstant -JSON '{\"At\":\"2026-09-16T07:30:00Z\"}' -Property 'At'; [Console]::Write($value.ToString('yyyy-MM-ddTHH:mm:ssZ'))")
	command.Env = append(os.Environ(), "ARDENTS_CANONICAL_INSTANT_SCRIPT="+helper)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("canonical instant helper failed: %v\n%s", err, output)
	}
	if string(output) != "2026-09-16T07:30:00Z" {
		t.Fatalf("canonical instant = %q", output)
	}
}

func TestQualificationAdmissionWindowHelper(t *testing.T) {
	root := repositoryRoot(t)
	helper := filepath.Join(root, "tests", "qualification", "stream-network-two-host", "admission-window.ps1")
	pwsh := "pwsh"
	if runtime.GOOS == "windows" {
		pwsh = "pwsh.exe"
	}
	command := exec.Command(pwsh, "-NoProfile", "-Command",
		". $env:ARDENTS_ADMISSION_WINDOW_SCRIPT; $open = Get-QualificationAdmissionWindowDelay -Now ([DateTimeOffset]::Parse('2026-09-17T14:30:00Z')) -MinimumRemaining ([TimeSpan]::FromMinutes(15)); $closing = Get-QualificationAdmissionWindowDelay -Now ([DateTimeOffset]::Parse('2026-09-17T14:55:00Z')) -MinimumRemaining ([TimeSpan]::FromMinutes(15)); [Console]::Write(('{0},{1}' -f [int]$open.TotalSeconds, [int]$closing.TotalSeconds))")
	command.Env = append(os.Environ(), "ARDENTS_ADMISSION_WINDOW_SCRIPT="+helper)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("admission-window helper failed: %v\n%s", err, output)
	}
	if string(output) != "0,302" {
		t.Fatalf("admission-window delays = %q, want 0,302", output)
	}
}
func TestQualificationClockObserverAdvancesObservationContent(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	if _, err := exec.LookPath(python); err != nil {
		t.Fatalf("qualification clock observer requires %s: %v", python, err)
	}
	observation := filepath.Join(t.TempDir(), "clock", "observation")
	command := exec.Command(python,
		filepath.Join(root, "tests", "qualification", "stream-network-two-host", "clock_observer.py"), observation)
	if output, err := command.StdoutPipe(); err != nil || output == nil {
		t.Fatalf("clock observer stdout: %v", err)
	}
	if output, err := command.StderrPipe(); err != nil || output == nil {
		t.Fatalf("clock observer stderr: %v", err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})

	var first string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		body, readErr := os.ReadFile(observation)
		if readErr == nil {
			current := strings.TrimSpace(string(body))
			if _, parseErr := time.Parse(time.RFC3339, current); parseErr != nil {
				t.Fatalf("clock observation %q: %v", current, parseErr)
			}
			if first == "" {
				first = current
			} else if current != first {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("clock observation content remained %q", first)
}

func TestQualificationClockObserverStaysCurrentWhenDurableSyncIsSlow(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	if _, err := exec.LookPath(python); err != nil {
		t.Fatalf("qualification clock observer requires %s: %v", python, err)
	}

	injection := t.TempDir()
	if err := os.WriteFile(filepath.Join(injection, "sitecustomize.py"), []byte(
		"import os\nimport time\n_original_fsync = os.fsync\ndef slow_fsync(fd):\n    time.sleep(3)\n    return _original_fsync(fd)\nos.fsync = slow_fsync\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	observation := filepath.Join(t.TempDir(), "clock", "observation")
	command := exec.Command(python,
		filepath.Join(root, "tests", "qualification", "stream-network-two-host", "clock_observer.py"), observation)
	command.Env = append(os.Environ(), "PYTHONPATH="+injection)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})

	var first time.Time
	firstDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(firstDeadline) {
		if info, statErr := os.Stat(observation); statErr == nil {
			first = info.ModTime()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if first.IsZero() {
		t.Fatal("clock observer published no initial observation")
	}
	advanceDeadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(advanceDeadline) {
		if info, statErr := os.Stat(observation); statErr == nil && info.ModTime().After(first) && time.Since(info.ModTime()) < 2*time.Second {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("clock observation did not remain inside the two-second confidence bound while durable sync was slow")
}
