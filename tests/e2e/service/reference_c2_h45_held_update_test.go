//go:build h4_5_rendezvous

package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type h45LifecycleResult struct {
	output []byte
	err    error
}

func TestH45InstalledRendezvousUpdatePreservesHeldPair(t *testing.T) {
	environment := requireH43MultiHostEnvironment(t)
	scenario := referenceC2Scenario{transparentApplication: true, heldRoute: true}
	deadline := time.Now().UTC().Truncate(time.Second).Add(5 * time.Minute)
	remote := h43RemoteC2{environment: environment}
	t.Cleanup(func() { remote.remove(t) })
	stage := stageH43RemoteC2(t, environment, deadline, scenario)
	deployment, pin := stageH45Bundle(t, stage.root, 1, 64<<20)
	nextDeployment, nextPin := stageH45Bundle(t, stage.root, 2, 64<<20)
	if nextDeployment != deployment {
		t.Fatal("H4-5 held update changed the deployment identity")
	}
	h43WriteFile(t, filepath.Join(stage.root, "run.sh"), []byte(h45RemoteRunner()), 0o700)
	removed := false
	t.Cleanup(func() {
		if removed {
			return
		}
		command := fmt.Sprintf(`set +e
touch %s/stop-clock 2>/dev/null || true
if [ -x %s ]; then
  %s contributor withdraw >/dev/null 2>&1 || true
  %s contributor remove --confirm %s >/dev/null 2>&1
  status=$?
else
  status=0
fi
exit "$status"`, h43ShellQuote(environment.remoteDirectory), h45ContributorCommand,
			h45ContributorCommand, h45ContributorCommand, h43ShellQuote(deployment))
		if output, err := remote.runCleanup(command); err != nil {
			t.Errorf("cleanup installed H4-5 held-update Contributor: %v\n%s", err, output)
		}
	})
	t.Cleanup(func() {
		if t.Failed() {
			h45RetainFailureEvidence(t, remote)
			h45RetainContributorFailureEvidence(t, remote)
		}
	})

	remote.start(t, stage.root)
	remote.waitFile(t, environment.remoteDirectory+"/sources-ready", deadline)
	applyBinary := environment.remoteDirectory + "/bundle-1/ardents-node"
	if output, err := remote.run(t, "set -eu; chmod 700 "+h43ShellQuote(applyBinary)); err != nil {
		t.Fatalf("restore H4-5 held-update executable mode: %v\n%s", err, output)
	}
	h45RunLifecycle(t, remote, "apply", applyBinary, fmt.Sprintf("contributor apply --bundle %s --manifest-pin %s",
		h43ShellQuote(environment.remoteDirectory+"/bundle-1"), h43ShellQuote(pin)))
	if output, err := remote.run(t, fmt.Sprintf("set -eu; printf 'ready\\n' >%s/contributor-ready",
		h43ShellQuote(environment.remoteDirectory))); err != nil {
		t.Fatalf("release H4-5 held-update topology roles: %v\n%s", err, output)
	}

	remote.copyFile(t, environment.remoteDirectory+"/publication.json", stage.publication, deadline)
	remote.copyFile(t, environment.remoteDirectory+"/gateway-profile.json", stage.gatewayProfile, deadline)
	remote.copyFile(t, environment.remoteDirectory+"/alpha-relay-ready.json", stage.relayReady, deadline)
	stageReferenceC2AlphaCorpus(t, h43LocalProductCommand(t, "ardents"), h43LocalProductCommand(t, "ardents-control"),
		stage.publication, stage.alphaAuthority, stage.alphaPrivate, filepath.Join(stage.localRoot, "alpha-floor"), stage.network, stage.alphaLink)

	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	defer cancel()
	user := startKillableCommand(ctx, stage.localRoot, stage.fixture, "user", stage.localConfig)
	remote.copyFile(t, environment.remoteDirectory+"/held-route-ready", filepath.Join(stage.localRoot, "held-route-ready"), deadline)
	h45WaitForHeldUser(t, filepath.Join(stage.localRoot, "held-route-user-ready"), user.result, deadline)

	h45StartDrainingCapture(t, remote)
	update := h45StartLifecycle(t, remote, "update-held", h45ContributorCommand, fmt.Sprintf("contributor apply --bundle %s --manifest-pin %s",
		h43ShellQuote(environment.remoteDirectory+"/bundle-2"), h43ShellQuote(nextPin)))
	h45AwaitDrainingCapture(t, remote)
	select {
	case result := <-update:
		t.Fatalf("H4-5 held update completed before admitted-pair release: %v\n%s", result.err, result.output)
	default:
	}
	if connection, err := net.DialTimeout("tcp", net.JoinHostPort(environment.host, fmt.Sprint(environment.port+1)), time.Second); err == nil {
		_ = connection.Close()
		t.Fatal("H4-5 updating Contributor accepted a new TCP connection while draining")
	}
	h45ReleaseHeldPair(t, remote, stage.localRoot)
	updateResult := h45AwaitLifecycle(t, update, 20*time.Second)
	if updateResult.err != nil {
		report, _ := remote.readFile(t, environment.remoteDirectory+"/contributor-update-held.err")
		t.Fatalf("H4-5 held update failed: %v\n%s\n%s", updateResult.err, updateResult.output, report)
	}
	updated, err := remote.readFile(t, environment.remoteDirectory+"/contributor-update-held.json")
	if err != nil || !strings.Contains(string(updated), `"generation":2`) || !strings.Contains(string(updated), `"lifecycle_state":"READY"`) {
		t.Fatalf("H4-5 held update report = %q / %v, want generation 2 READY", updated, err)
	}
	h45RunLifecycle(t, remote, "diagnose-held-successor", h45ContributorCommand, "contributor diagnose")
	h45CapturePlacement(t, remote)

	result := h45AwaitUser(t, user.result, deadline)
	h43AssertUserResult(t, result, scenario)
	if class := h45UserClass(t, result); class != "clean service connection close" {
		t.Fatalf("H4-5 held-update User class = %q, want clean service connection close", class)
	}
	remote.complete(t)
	remote.wait(t)
	for _, role := range []string{"initiator", "introduction", "responder", "gateway", "alpha-gateway", "alpha-relay"} {
		if roleResult := h43RemoteResult(t, remote, role); roleResult.Class != "drained" {
			t.Fatalf("H4-5 held-update remote %s class = %q, want drained", role, roleResult.Class)
		}
	}
	if application := h43RemoteResult(t, remote, "publisher-app"); application.Class != "held" {
		t.Fatalf("H4-5 held-update Publisher Application class = %q, want held", application.Class)
	}
	if _, err := remote.readFile(t, environment.remoteDirectory+"/rendezvous.log"); err == nil {
		t.Fatal("H4-5 held-update topology started the fixture Rendezvous")
	}

	h45RunLifecycle(t, remote, "drain", h45ContributorCommand, "contributor drain")
	h45RunLifecycle(t, remote, "withdraw", h45ContributorCommand, "contributor withdraw")
	h45RunLifecycle(t, remote, "remove", h45ContributorCommand, "contributor remove --confirm "+h43ShellQuote(deployment))
	removed = true
	h45RetainHeldUpdateEvidence(t, remote, result)
}

func h45LifecycleCommand(root, name, executable, arguments string) string {
	return fmt.Sprintf(`set +e
started=$(date +%%s%%N)
%s %s >%s/contributor-%s.json 2>%s/contributor-%s.err
status=$?
finished=$(date +%%s%%N)
printf 'schema=ardents-h4-5-operator-timing-v1\naction=%s\nstarted_unix_ns=%%s\nfinished_unix_ns=%%s\nexit_status=%%s\n' "$started" "$finished" "$status" >%s/contributor-%s.timing
exit "$status"`, h43ShellQuote(executable), arguments, h43ShellQuote(root), name, h43ShellQuote(root), name, name, h43ShellQuote(root), name)
}

func h45StartLifecycle(t *testing.T, remote h43RemoteC2, name, executable, arguments string) <-chan h45LifecycleResult {
	t.Helper()
	command := remote.command(t, h45LifecycleCommand(remote.environment.remoteDirectory, name, executable, arguments))
	result := make(chan h45LifecycleResult, 1)
	go func() {
		output, err := command.CombinedOutput()
		result <- h45LifecycleResult{output: output, err: err}
		close(result)
	}()
	return result
}

func h45WaitForHeldUser(t *testing.T, path string, result <-chan commandResult, deadline time.Time) {
	t.Helper()
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return
		}
		select {
		case early := <-result:
			t.Fatalf("H4-5 User exited before held-pair readiness: %v\n%s", early.err, early.output)
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("H4-5 User did not acknowledge the held pair at %s", path)
}

func h45StartDrainingCapture(t *testing.T, remote h43RemoteC2) {
	t.Helper()
	root := h43ShellQuote(remote.environment.remoteDirectory)
	command := fmt.Sprintf(`set -eu
cp /var/lib/ardents-contributor/installation.json %[1]s/contributor-update-held-pre-switch.json
grep -F '"generation":1' %[1]s/contributor-update-held-pre-switch.json >/dev/null
rm -f %[1]s/contributor-update-held-draining.captured %[1]s/contributor-update-held-draining.missed
(
  tries=0
  while [ "$tries" -lt 500 ]; do
    if [ -s /var/lib/private/ardents-contributor/diagnostics/lifecycle.json ] && grep -F '"state":"DRAINING"' /var/lib/private/ardents-contributor/diagnostics/lifecycle.json >/dev/null; then
      cp /var/lib/private/ardents-contributor/diagnostics/lifecycle.json %[1]s/contributor-update-held-draining.json
      systemctl show ardents-rendezvous-contributor.service -p ActiveState -p SubState -p MainPID >%[1]s/contributor-update-held-draining-systemd.txt
      printf 'captured\n' >%[1]s/contributor-update-held-draining.captured
      exit 0
    fi
    tries=$((tries + 1))
    sleep 0.01
  done
  printf 'missed\n' >%[1]s/contributor-update-held-draining.missed
) >%[1]s/contributor-update-held-draining-watch.log 2>&1 </dev/null &`, root)
	if output, err := remote.run(t, command); err != nil {
		t.Fatalf("start H4-5 held-update DRAINING capture: %v\n%s", err, output)
	}
}

func h45AwaitDrainingCapture(t *testing.T, remote h43RemoteC2) {
	t.Helper()
	path := remote.environment.remoteDirectory + "/contributor-update-held-draining.captured"
	remote.waitFile(t, path, time.Now().Add(8*time.Second))
}

func h45ReleaseHeldPair(t *testing.T, remote h43RemoteC2, localRoot string) {
	t.Helper()
	remotePath := h43ShellQuote(remote.environment.remoteDirectory + "/held-route-release")
	if output, err := remote.run(t, "set -eu; printf 'release\\n' >"+remotePath); err != nil {
		t.Fatalf("release H4-5 remote held pair: %v\n%s", err, output)
	}
	if err := os.WriteFile(filepath.Join(localRoot, "held-route-release"), []byte("release\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func h45AwaitLifecycle(t *testing.T, result <-chan h45LifecycleResult, timeout time.Duration) h45LifecycleResult {
	t.Helper()
	select {
	case observed := <-result:
		return observed
	case <-time.After(timeout):
		t.Fatal("H4-5 lifecycle command did not complete inside its bound")
		return h45LifecycleResult{}
	}
}

func h45AwaitUser(t *testing.T, result <-chan commandResult, deadline time.Time) commandResult {
	t.Helper()
	select {
	case observed := <-result:
		return observed
	case <-time.After(time.Until(deadline)):
		t.Fatal("H4-5 held-update User did not complete before the fixture deadline")
		return commandResult{}
	}
}

func h45UserClass(t *testing.T, result commandResult) string {
	t.Helper()
	line := strings.TrimSpace(string(result.output))
	if index := strings.LastIndex(line, "\n"); index >= 0 {
		line = line[index+1:]
	}
	var observed struct{ Class string }
	if err := json.Unmarshal([]byte(line), &observed); err != nil {
		t.Fatalf("decode H4-5 held-update User result: %v\n%s", err, result.output)
	}
	return observed.Class
}

func h45RetainHeldUpdateEvidence(t *testing.T, remote h43RemoteC2, user commandResult) {
	t.Helper()
	root := os.Getenv("ARDENTS_H4_5_EVIDENCE_DIR")
	if root == "" {
		return
	}
	if !filepath.IsAbs(root) {
		t.Fatal("ARDENTS_H4_5_EVIDENCE_DIR must be absolute")
	}
	h45PrepareEvidenceRoot(t, root)
	if err := os.WriteFile(filepath.Join(root, "remote-capture.txt"), remote.captureEvidence(t), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "user.stdout.log"), user.stdout, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "user.stderr.log"), user.stderr, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"contributor-apply.json", "contributor-apply.timing",
		"contributor-update-held.json", "contributor-update-held.timing",
		"contributor-update-held-draining.json", "contributor-update-held-pre-switch.json", "contributor-update-held-draining-systemd.txt",
		"contributor-diagnose-held-successor.json", "contributor-diagnose-held-successor.timing",
		"contributor-systemd.txt", "contributor-cgroup.txt",
		"contributor-drain.json", "contributor-drain.timing",
		"contributor-withdraw.json", "contributor-withdraw.timing",
		"contributor-remove.json", "contributor-remove.timing",
	} {
		contents, err := remote.readFile(t, remote.environment.remoteDirectory+"/"+name)
		if err != nil {
			t.Fatalf("retain H4-5 held-update evidence %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(root, name), contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func h45RetainContributorFailureEvidence(t *testing.T, remote h43RemoteC2) {
	t.Helper()
	root := os.Getenv("ARDENTS_H4_5_EVIDENCE_DIR")
	if root == "" || !filepath.IsAbs(root) {
		return
	}
	h45PrepareEvidenceRoot(t, root)
	command := `set +e
printf 'schema=ardents-h4-5-contributor-failure-v1\n[unit]\n'
systemctl show ardents-rendezvous-contributor.service -p ActiveState -p SubState -p MainPID -p CPUQuotaPerSecUSec -p MemoryHigh -p MemoryMax -p TasksMax -p LimitNOFILE
printf '[status]\n'
systemctl status ardents-rendezvous-contributor.service --no-pager --full
printf '[diagnostics]\n'
for file in /var/lib/private/ardents-contributor/diagnostics/*.json; do if [ -f "$file" ]; then printf '===%s===\n' "$(basename "$file")"; head -c 8192 "$file"; printf '\n'; fi; done
printf '[installation]\n'
head -c 8192 /var/lib/ardents-contributor/installation.json
printf '\n[journal]\n'
journalctl -u ardents-rendezvous-contributor.service --since "5 minutes ago" --no-pager --output=short-iso -n 200
exit 0`
	output, err := remote.runCleanup(command)
	if err != nil || len(output) == 0 || len(output) > 1<<20 {
		t.Errorf("capture H4-5 Contributor failure evidence: %v", err)
		return
	}
	name := "contributor-host-failure-" + remote.environment.container + ".txt"
	file, openErr := os.OpenFile(filepath.Join(root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if openErr != nil {
		t.Errorf("retain H4-5 Contributor failure evidence: %v", openErr)
		return
	}
	_, writeErr := file.WriteString(output)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Errorf("retain H4-5 Contributor failure evidence: %v / %v", writeErr, closeErr)
	}
}
