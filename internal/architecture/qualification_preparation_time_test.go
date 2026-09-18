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

func TestQualificationPrivateFixtureTransfersHaveBoundedRetries(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"generate-windows.ps1", "prepare-windows.ps1"} {
		path := filepath.Join(root, "tests", "qualification", "stream-network-two-host", name)
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, required := range []string{"function Invoke-SCP", "foreach ($attempt in 1..3)", "Start-Sleep -Seconds $attempt", "failed after three bounded attempts"} {
			if !strings.Contains(string(body), required) {
				t.Fatalf("%s lacks bounded idempotent transfer retry %q", name, required)
			}
		}
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
	dutyRoots := strings.Index(text, "$dutyRoots = @($provision.State | Where-Object { [string]$_.Host -ceq $role } | ForEach-Object { foreach ($dutyRoot in @($_.DutyRoots)) { [string]$dutyRoot } })")
	dutyParents := strings.Index(text, "$dutyParents = @($provision.State | Where-Object { [string]$_.Host -ceq $role -and -not [string]::IsNullOrWhiteSpace([string]$_.DutyParent) } | ForEach-Object { [string]$_.DutyParent })")
	ownerPaths := strings.Index(text, "$paths = @($stateRoots + $localRoleRoots + $dutyRoots)")
	traversableParents := strings.Index(text, "$parentPaths = @(\"$remoteRoot/state\", \"$remoteRoot/state-roles\", \"$remoteRoot/roles\", \"$remoteRoot/duty\", \"$remoteRoot/endpoint\", \"$remoteRoot/service\") + $dutyParents")
	assignment := -1
	writableCheck := -1
	if ownerPaths >= 0 {
		if offset := strings.Index(text[ownerPaths:], "; chown -R ardents-endpoint:ardents-endpoint "); offset >= 0 {
			assignment = ownerPaths + offset
		}
		if offset := strings.Index(text[ownerPaths:], `$verificationCommand = 'set -eu; for path in ' + $pathList + '; do runuser -u ardents-endpoint -- test -d "$path"; runuser -u ardents-endpoint -- test -w "$path"; done'`); offset >= 0 {
			writableCheck = ownerPaths + offset
		}
	}
	if stateRoots < 0 || localRoleRoots < stateRoots || dutyRoots < localRoleRoots || dutyParents < dutyRoots || ownerPaths < dutyParents || traversableParents < dutyParents || assignment < ownerPaths || writableCheck < assignment || strings.Contains(text, "$writableChecks =") {
		t.Fatal("qualification preparer does not assign every host State and local role root to the shared runtime owner")
	}

	provisioningBody, err := os.ReadFile(filepath.Join(root, "tests", "qualification", "stream-network-two-host", "fixturecommand", "qualification-network", "provisioning.go"))
	if err != nil {
		t.Fatal(err)
	}
	runtimePlansBody, err := os.ReadFile(filepath.Join(root, "tests", "qualification", "stream-network-two-host", "fixturecommand", "qualification-network", "runtime_plans.go"))
	if err != nil {
		t.Fatal(err)
	}
	provisioning := string(provisioningBody) + string(runtimePlansBody)
	for _, required := range []string{
		"LocalRoleStateRoot",
		"DutyRoots",
		"DutyParent",
		`path.Join(result.RemoteRoot, "roles", fmt.Sprintf("node-%02d", index))`,
		`path.Join(base, "spends")`,
		`path.Join(base, "issuer")`,
		`path.Join(base, "descriptors")`,
		`path.Join(base, "admission")`,
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

func TestQualificationStartsRouteNodesAsOnePreparedGroup(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "tests", "qualification", "stream-network-two-host", "run-windows.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	start := strings.Index(text, "function Start-RouteNodes")
	stop := strings.Index(text, "function Stop-RouteNodes")
	if start < 0 || stop <= start {
		t.Fatal("qualification runner has no bounded Route Node start function")
	}
	function := text[start:stop]
	prepared := strings.Index(function, "$preparedNodes +=")
	grouped := strings.Index(function, "$preparedNodes | Group-Object { [string]$_.Machine }")
	started := strings.Index(function, "systemd-run --no-block --unit '$unit'")
	ready := strings.Index(function, "read Route Node $index journal")
	if prepared < 0 || grouped < prepared || started < grouped || ready < started {
		t.Fatal("qualification runner does not prepare every Route Node before group start and readiness")
	}
	if strings.Contains(function, "Group-Object Machine") {
		t.Fatal("qualification runner groups ordered dictionaries by a property name that PowerShell resolves as empty")
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

func TestQualificationWaitsBeforeConsumingAClosingAdmissionWindow(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(root, "tests", "qualification", "stream-network-two-host", "admission-window.ps1")
	runnerPath := filepath.Join(root, "tests", "qualification", "stream-network-two-host", "run-windows.ps1")
	runner, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(runner)
	for _, required := range []string{
		". (Join-Path $PSScriptRoot 'admission-window.ps1')",
		"Get-QualificationAdmissionWindowDelay -Now ([DateTimeOffset]::UtcNow) -MinimumRemaining ([TimeSpan]::FromMinutes(15))",
		"Start-Sleep -Seconds $delaySeconds",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("qualification runner lacks admission-window guard %q", required)
		}
	}
	guard := strings.LastIndex(text, "Get-QualificationAdmissionWindowDelay -Now")
	remoteWork := strings.Index(text, "try {\n    foreach ($hostName")
	if guard < 0 || remoteWork < guard {
		t.Fatal("qualification runner starts remote work before the admission-window guard")
	}

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

func TestQualificationSmokeAllowsCompleteRetainedSetup(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := os.ReadFile(filepath.Join(root, "internal", "endpoint", "stream_qualification_connections_linux.go"))
	if err != nil {
		t.Fatal(err)
	}
	readerText := string(reader)
	for _, required := range []string{
		"qualificationIntroductionInterval = 1250 * time.Millisecond",
		"for index := 0; index < 64; index++",
	} {
		if !strings.Contains(readerText, required) {
			t.Fatalf("retained Reader setup no longer has the qualified schedule %q", required)
		}
	}
	runner, err := os.ReadFile(filepath.Join(root, "tests", "qualification", "stream-network-two-host", "run-windows.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(runner), "$smokeDeadline = [DateTime]::UtcNow.AddMinutes(3)") {
		t.Fatal("qualification smoke deadline cannot contain the measured retained Reader setup")
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
