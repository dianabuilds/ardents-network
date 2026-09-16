package architecture

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestQualificationPreparationReadsCanonicalInstantsBeforePowerShellConversion(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(root, "tests", "qualification", "stream-network-two-host", "canonical-json-instant.ps1")
	preparerPath := filepath.Join(root, "tests", "qualification", "stream-network-two-host", "prepare-windows.ps1")
	preparer, err := os.ReadFile(preparerPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		". (Join-Path $PSScriptRoot 'canonical-json-instant.ps1')",
		"Read-CanonicalJSONInstant -JSON $inventoryJSON -Property 'At'",
		"Read-CanonicalJSONInstant -JSON $inventoryJSON -Property 'NotAfter'",
	} {
		if !strings.Contains(string(preparer), required) {
			t.Fatalf("qualification preparer lacks %q", required)
		}
	}

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
