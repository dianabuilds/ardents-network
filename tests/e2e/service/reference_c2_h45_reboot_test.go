//go:build h4_5_rendezvous

package service_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestH45InstalledRendezvousRecoversFromKillAndHostReboot(t *testing.T) {
	environment := requireH43MultiHostEnvironment(t)
	environment.remoteDirectory = "/var/tmp/" + environment.container
	deadline := time.Now().UTC().Truncate(time.Second).Add(3 * time.Minute)
	remote := h43RemoteC2{environment: environment}
	t.Cleanup(func() { remote.remove(t) })
	t.Cleanup(func() {
		if t.Failed() {
			h45RetainFailureEvidence(t, remote)
		}
	})
	stage := stageH43RemoteC2(t, environment, deadline, referenceC2Scenario{transparentApplication: true})
	deployment, pin := stageH45Bundle(t, stage.root, 1)
	h43WriteFile(t, filepath.Join(stage.root, "run.sh"), []byte(h45SourcesOnlyRunner()), 0o700)
	removed := false
	t.Cleanup(func() {
		if removed {
			return
		}
		command := fmt.Sprintf(`set +e
docker update --restart=no %s >/dev/null 2>&1 || true
if [ -x %s ]; then
  %s contributor withdraw >/dev/null 2>&1 || true
  %s contributor remove --confirm %s >/dev/null 2>&1
  status=$?
else
  status=0
fi
exit "$status"`, h43ShellQuote(environment.container), h45ContributorCommand, h45ContributorCommand,
			h45ContributorCommand, h43ShellQuote(deployment))
		if output, err := remote.runCleanup(command); err != nil {
			t.Errorf("cleanup reboot-qualified H4-5 Contributor: %v\n%s", err, output)
		}
	})

	remote.start(t, stage.root)
	remote.waitFile(t, environment.remoteDirectory+"/sources-ready", deadline)
	applyBinary := environment.remoteDirectory + "/bundle-1/ardents-node"
	if output, err := remote.run(t, "set -eu; chmod 700 "+h43ShellQuote(applyBinary)); err != nil {
		t.Fatalf("restore H4-5 staged executable mode: %v\n%s", err, output)
	}
	h45RunLifecycle(t, remote, "apply-reboot", applyBinary, fmt.Sprintf("contributor apply --bundle %s --manifest-pin %s",
		h43ShellQuote(environment.remoteDirectory+"/bundle-1"), h43ShellQuote(pin)))
	h45RunLifecycle(t, remote, "diagnose-before-kill", h45ContributorCommand, "contributor diagnose")
	h45AssertAutomaticKillRecovery(t, remote)

	bootID := h45ReadBootID(t, remote)
	command := fmt.Sprintf(`set -eu
docker update --restart=unless-stopped %s >/dev/null
printf 'boot_id_before=%%s\n' %s >>%s/contributor-kill-reboot.txt
sync
systemd-run --quiet --unit=ardents-h45-reboot --on-active=2s /usr/bin/systemctl reboot`,
		h43ShellQuote(environment.container), h43ShellQuote(bootID), h43ShellQuote(environment.remoteDirectory))
	if output, err := remote.run(t, command); err != nil {
		t.Fatalf("schedule bounded H4-5 host reboot: %v\n%s", err, output)
	}
	h45WaitForNewBoot(t, remote, bootID, deadline)
	h45WaitForRebootedDuty(t, remote, deadline)
	h45RunLifecycle(t, remote, "diagnose-after-reboot", h45ContributorCommand, "contributor diagnose")
	h45RunLifecycle(t, remote, "withdraw-reboot", h45ContributorCommand, "contributor withdraw")
	h45RunLifecycle(t, remote, "remove-reboot", h45ContributorCommand, "contributor remove --confirm "+h43ShellQuote(deployment))
	h45RetainRestartEvidence(t, remote)
	removed = true
}

func h45AssertAutomaticKillRecovery(t *testing.T, remote h43RemoteC2) {
	t.Helper()
	root := h43ShellQuote(remote.environment.remoteDirectory)
	command := fmt.Sprintf(`set -eu
before=$(systemctl show ardents-rendezvous-contributor.service -p MainPID --value)
test "$before" -gt 1
systemctl kill -s SIGKILL ardents-rendezvous-contributor.service
tries=0
while [ "$tries" -lt 80 ]; do
  after=$(systemctl show ardents-rendezvous-contributor.service -p MainPID --value 2>/dev/null || true)
  if [ "$after" -gt 1 ] 2>/dev/null && [ "$after" != "$before" ] && systemctl is-active --quiet ardents-rendezvous-contributor.service; then
    printf 'sigkill_pid_before=%%s\nsigkill_pid_after=%%s\n' "$before" "$after" >%s/contributor-kill-reboot.txt
    exit 0
  fi
  tries=$((tries + 1))
  sleep 0.25
done
exit 1`, root)
	if output, err := remote.run(t, command); err != nil {
		t.Fatalf("H4-5 Contributor did not recover from SIGKILL: %v\n%s", err, output)
	}
}

func h45ReadBootID(t *testing.T, remote h43RemoteC2) string {
	t.Helper()
	output, err := remote.run(t, "set -eu; cat /proc/sys/kernel/random/boot_id")
	bootID := strings.TrimSpace(output)
	if err != nil || len(bootID) != 36 {
		t.Fatalf("read H4-5 boot identity: %v / %q", err, output)
	}
	return bootID
}

func h45WaitForNewBoot(t *testing.T, remote h43RemoteC2, previous string, deadline time.Time) {
	t.Helper()
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		output, err := remote.commandContext(ctx, "cat /proc/sys/kernel/random/boot_id").CombinedOutput()
		cancel()
		observed := strings.TrimSpace(string(output))
		if err == nil && len(observed) == 36 && observed != previous {
			command := fmt.Sprintf("printf 'boot_id_after=%%s\\n' %s >>%s/contributor-kill-reboot.txt",
				h43ShellQuote(observed), h43ShellQuote(remote.environment.remoteDirectory))
			if appendOutput, appendErr := remote.run(t, command); appendErr != nil {
				t.Fatalf("retain H4-5 reboot identity: %v\n%s", appendErr, appendOutput)
			}
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("H4-5 host did not return with a new boot identity inside three minutes")
}

func h45WaitForRebootedDuty(t *testing.T, remote h43RemoteC2, deadline time.Time) {
	t.Helper()
	container := h43ShellQuote(remote.environment.container)
	for time.Now().Before(deadline) {
		command := "set -eu; test \"$(docker inspect -f '{{.State.Running}}' " + container + ")\" = true; " +
			"systemctl is-active --quiet ardents-rendezvous-contributor.service; " +
			"test \"$(systemctl show ardents-rendezvous-contributor.service -p MainPID --value)\" -gt 1; " +
			h45ContributorCommand + " contributor diagnose | grep -F '\"lifecycle_state\":\"READY\"' >/dev/null"
		if _, err := remote.run(t, command); err == nil {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("H4-5 Sources and installed Contributor did not recover after host reboot")
}

func h45RetainRestartEvidence(t *testing.T, remote h43RemoteC2) {
	t.Helper()
	root := os.Getenv("ARDENTS_H4_5_EVIDENCE_DIR")
	if root == "" {
		return
	}
	if !filepath.IsAbs(root) {
		t.Fatal("ARDENTS_H4_5_EVIDENCE_DIR must be absolute")
	}
	h45PrepareEvidenceRoot(t, root)
	for _, name := range []string{"contributor-kill-reboot.txt", "contributor-apply-reboot.json",
		"contributor-diagnose-before-kill.json", "contributor-diagnose-after-reboot.json",
		"contributor-withdraw-reboot.json", "contributor-remove-reboot.json"} {
		contents, err := remote.readFile(t, remote.environment.remoteDirectory+"/"+name)
		if err != nil {
			t.Fatalf("retain H4-5 restart evidence %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(root, name), contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func h45SourcesOnlyRunner() string {
	return `#!/bin/sh
set -eu
work=/work
pids=""
cleanup() {
  status=$?
  trap - EXIT INT TERM
  for pid in $pids; do kill "$pid" 2>/dev/null || true; done
  for pid in $pids; do wait "$pid" 2>/dev/null || true; done
  exit "$status"
}
trap cleanup EXIT INT TERM
start_source() {
  name=$1
  "$work/ardents-node" source --config "$work/$name-plan.json" >"$work/$name.log" 2>"$work/$name.err" &
  pid=$!
  pids="$pids $pid"
  tries=0
  while [ "$tries" -lt 200 ]; do
    if grep -F '"kind":"source-ready"' "$work/$name.log" >/dev/null 2>&1; then return 0; fi
    tries=$((tries + 1))
    sleep 0.1
  done
  return 1
}
start_source source-a
start_source source-b
printf 'ready\n' >"$work/sources-ready"
while [ ! -e "$work/stop-sources" ]; do sleep 1; done
`
}
