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
		"$serviceAuthority = New-Authority 'service'",
		"RemoteRoot is outside the fixed issue-60 fixture namespace.",
		"chown root:ardents-endpoint '$bundleRoot' '$privateRoot'; chmod 710 '$bundleRoot' '$privateRoot'",
		"chown -R ardents-endpoint:ardents-endpoint '$nodePrivate' '$sourcePrivate'",
		"runuser -u ardents-endpoint -- find '$nodePrivate' '$sourcePrivate' -type f ! -readable -print -quit",
		"find '$privateRoot' -maxdepth 1 -type f -exec chmod 600 '{}' +",
		"! runuser -u ardents-endpoint -- test -r '$privateRoot/state-authority.pem'",
		"find '$nodePrivate' '$sourcePrivate' -type d -exec chmod 700 '{}' +",
		"find '$nodePrivate' '$sourcePrivate' -type f -exec chmod 600 '{}' +",
		"runuser -u ardents-endpoint -- test -r '$sourcePrivate/0-cert.pem'",
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
	serviceAuthority := strings.Index(string(preparer), "$serviceAuthority = New-Authority 'service'")
	if rootPreparation < 0 || serviceInitialization < 0 || rootPreparation > serviceInitialization || serviceAuthority < serviceInitialization {
		t.Fatal("qualification preparer orders Service roots and per-owner authorities incorrectly")
	}
	accountInstallation := strings.Index(string(preparer), "install qualification command identities")
	privateBinding := strings.Index(string(preparer), "bind qualification runtime credentials")
	stateAcceptance := strings.Index(string(preparer), "foreach ($state in @($provision.State))")
	if accountInstallation < 0 || privateBinding < accountInstallation || stateAcceptance < privateBinding {
		t.Fatal("qualification preparer does not bind runtime credentials after creating the Endpoint account and before State acceptance")
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

func TestQualificationPreparationAssignsEveryStateRootToRuntimeOwner(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "tests", "qualification", "stream-network-two-host", "prepare-windows.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	stateRoots := strings.Index(text, "$stateRoots = @($provision.State | Where-Object { [string]$_.Host -ceq $role } | ForEach-Object { [string]$_.Root })")
	localRoleRoots := strings.Index(text, "$localRoleRoots = @($provision.State | Where-Object { [string]$_.Host -ceq $role } | ForEach-Object { [string]$_.LocalRoleStateRoot })")
	dutyRoots := strings.Index(text, "$dutyRoots = @($provision.State | Where-Object { [string]$_.Host -ceq $role -and -not [string]::IsNullOrWhiteSpace([string]$_.DutyRoot) } | ForEach-Object { [string]$_.DutyRoot })")
	ownerPaths := strings.Index(text, "$paths = @($stateRoots + $localRoleRoots + $dutyRoots)")
	assignment := -1
	writableCheck := -1
	if ownerPaths >= 0 {
		if offset := strings.Index(text[ownerPaths:], "; chown -R ardents-endpoint:ardents-endpoint "); offset >= 0 {
			assignment = ownerPaths + offset
		}
		if offset := strings.Index(text[ownerPaths:], `$verificationCommand = 'for path in ' + $pathList + '; do runuser -u ardents-endpoint -- test -d "$path"; runuser -u ardents-endpoint -- test -w "$path"; done'`); offset >= 0 {
			writableCheck = ownerPaths + offset
		}
	}
	if stateRoots < 0 || localRoleRoots < stateRoots || dutyRoots < localRoleRoots || ownerPaths < dutyRoots || assignment < ownerPaths || writableCheck < assignment || strings.Contains(text, "$writableChecks =") {
		t.Fatal("qualification preparer does not assign every host State and local role root to the shared runtime owner")
	}

	provisioningBody, err := os.ReadFile(filepath.Join(root, "tests", "qualification", "stream-network-two-host", "fixturecommand", "qualification-network", "provisioning.go"))
	if err != nil {
		t.Fatal(err)
	}
	provisioning := string(provisioningBody)
	for _, required := range []string{
		"LocalRoleStateRoot",
		"DutyRoot",
		`path.Join(result.RemoteRoot, "roles", fmt.Sprintf("node-%02d", index))`,
		`path.Join(result.RemoteRoot, "duty", item.ID)`,
		`path.Join(result.RemoteRoot, "state-roles", item.Name)`,
		`path.Join(result.RemoteRoot, "roles", item.Name)`,
	} {
		if !strings.Contains(provisioning, required) {
			t.Fatalf("qualification provisioning omits runtime-owned local role root %q", required)
		}
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

func TestQualificationActivatesOwnerSliceBeforeInspectingLimits(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "tests", "qualification", "stream-network-two-host", "run-windows.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	setProperty := strings.Index(text, "systemctl set-property --runtime '$unit'")
	start := strings.Index(text, "systemctl start '$unit'")
	inspect := strings.Index(text, "group=`$(systemctl show '$unit' -p ControlGroup --value)")
	if setProperty < 0 || start < setProperty || inspect < start {
		t.Fatal("qualification runner inspects owner slice limits before activating its cgroup")
	}
}

func TestQualificationResetsOnlyFailedEndpointUnit(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{
		filepath.Join(root, "tests", "qualification", "stream-network-two-host", "run-windows.ps1"),
		filepath.Join(root, "tests", "qualification", "net32-idle-one-host", "run-windows.ps1"),
	}
	for _, path := range paths {
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(body)
		failFast := strings.Index(text, "set -eu; chmod 700")
		if failFast < 0 {
			t.Fatalf("%s does not fail fast during Endpoint installation", path)
		}
		command := text[failFast:]
		state := strings.Index(command, "systemctl show ardents-endpoint.service -p ActiveState --value")
		conditional := strings.Index(command, "if test")
		reset := strings.Index(command, "systemctl reset-failed ardents-endpoint.service")
		inactive := strings.Index(command, "= inactive; fi")
		if state < 0 || conditional < state || reset < conditional || inactive < reset {
			t.Fatalf("%s does not conditionally reset only a failed Endpoint unit", path)
		}
	}
}
