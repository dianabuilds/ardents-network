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
		"$atText = $at.ToString('yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)",
		"foreach ($class in 1..3)",
		"the exact six-window, three-class key inventory",
	} {
		if !strings.Contains(string(preparer), required) {
			t.Fatalf("qualification preparer lacks %q", required)
		}
	}
	if strings.Contains(string(preparer), "[string]$provision.At") {
		t.Fatal("qualification preparer reuses PowerShell's culture-sensitive JSON date conversion")
	}
	rootPreparation := strings.Index(string(preparer), "foreach ($role in @('reader','publisher')) {")
	serviceInitialization := strings.Index(string(preparer), "foreach ($service in @($provision.Services)) {")
	if rootPreparation < 0 || serviceInitialization < 0 || rootPreparation > serviceInitialization {
		t.Fatal("qualification preparer initializes Service Instances before their roots exist")
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

func TestQualificationCustodyDoesNotBlockOnOperatorInput(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{
		filepath.Join(root, "tests", "qualification", "stream-network-two-host", "prepare-windows.ps1"),
		filepath.Join(root, "tests", "qualification", "stream-network-two-host", "run-windows.ps1"),
		filepath.Join(root, "tests", "qualification", "net32-idle-one-host", "run-windows.ps1"),
	}
	for _, path := range paths {
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(body)
		for _, forbidden := range []string{"Read-Host", "-Interactive", "Enter this independently observed"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s retains operator-blocking custody input %q", path, forbidden)
			}
		}
		if !strings.Contains(text, "stty -echo") {
			t.Fatalf("%s does not suppress terminal echo around custody input", path)
		}
	}
	preparer, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(preparer), "ProtectedData]::Protect") ||
		!strings.Contains(string(preparer), "admission-secret.dpapi") {
		t.Fatal("qualification preparer does not retain the admission secret as account-bound ciphertext")
	}
	for _, path := range paths[1:] {
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(body), "ProtectedData]::Unprotect") {
			t.Fatalf("%s does not consume the account-bound custody secret", path)
		}
	}
}
