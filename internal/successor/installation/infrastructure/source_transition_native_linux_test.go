//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// This profile launches only the test executable and native file/process
// mechanisms. No installed artifact, Source, State, Release, manager or
// accepting recovery is constructed. Each controller owns a private process,
// so its actual subreaper cannot consume another test's children.
func TestSourceTransitionNativeOriginalScope(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid native stand", err)
	}
	for _, scenario := range []string{"complete", "sync-uncertain", "close-uncertain", "complete-with-descendant", "cut-frozen", "cut-ack-outstanding", "cut-proof", "cut-proof-sync-uncertain", "cut-proof-close-uncertain"} {
		t.Run(scenario, func(t *testing.T) {
			root, err := os.MkdirTemp(os.Getenv("ARDENTS_SOURCE_STOPPED_FUTURE_PARENT"), "transition-")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(ctx, self, "-test.run=^TestSourceTransitionNativeDriver$", "-test.v")
			command.Env = append(os.Environ(), "ARDENTS_SOURCE_TRANSITION_NATIVE_ROLE=controller", "ARDENTS_SOURCE_TRANSITION_NATIVE_CASE="+scenario, "ARDENTS_SOURCE_TRANSITION_NATIVE_ROOT="+root)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("actual native controller %s: %v\n%s", scenario, err, output)
			}
			t.Logf("retained %s: %s\n%s", scenario, root, output)
		})
	}
}

func TestSourceTransitionNativeDriver(t *testing.T) {
	role := os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_ROLE")
	if role == "" {
		return
	}
	if role == "coordinator" {
		sourceTransitionNativeCoordinator(t)
		return
	}
	if role != "controller" {
		t.Fatal("invalid private native role")
	}
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	scenario := os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_CASE")
	if scenario == "external-cut" || scenario == "unsafe-external-cut" {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, syscall.SIGTERM)
		noticed := make(chan error, 1)
		go func() {
			<-ctx.Done()
			if os.Getenv("ARDENTS_SOURCE_CUT_NATIVE_DELAYED_NOTICE") == "1" {
				// The canonical command reports completion-unavailable after this
				// same interval while its original work remains physically held.
				time.Sleep(5 * time.Second)
			}
			_, err := fmt.Fprintln(os.Stdout, "native-original-cancellation-observed")
			noticed <- err
		}()
		defer func() {
			cancel()
			stop()
			if err := <-noticed; err != nil {
				t.Error(err)
			}
		}()
	}
	program, err := os.Open("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	programTransferred := false
	defer func() {
		if !programTransferred {
			if err := program.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	channel, endpoint, err := sourceTransitionSocketPair()
	if err != nil {
		t.Fatal(err)
	}
	channelTransferred := false
	defer func() {
		if !channelTransferred {
			if err := channel.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	nonce := strings.Repeat("a", 64)
	environment := []string{}
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "ARDENTS_SOURCE_TRANSITION_NATIVE_ROLE=") {
			environment = append(environment, value)
		}
	}
	environment = append(environment, "ARDENTS_SOURCE_TRANSITION_NATIVE_ROLE=coordinator", "ARDENTS_SOURCE_TRANSITION_CHILD=coordinator-v1", "ARDENTS_SOURCE_TRANSITION_FD=4", "ARDENTS_SOURCE_TRANSITION_NONCE="+nonce)
	scope, err := forkSourceTransition(program, endpoint, []string{os.Args[0], "-test.run=^TestSourceTransitionNativeDriver$", "-test.v"}, environment)
	if scope != nil {
		defer func() {
			if err := scope.join(); err != nil {
				t.Error(err)
			}
		}()
	}
	closed := endpoint.Close()
	if err != nil || closed != nil {
		t.Fatal(errors.Join(err, closed))
	}
	deadline, _ := ctx.Deadline()
	if strings.HasPrefix(scenario, "cut-proof") || scenario == "external-cut" {
		programTransferred, channelTransferred = true, true
		sourceCutNativeProof(t, ctx, cancel, program, channel, scope, nonce, deadline, scenario)
		return
	}
	if scenario != "cut-frozen" && scenario != "cut-ack-outstanding" {
		var cut *sourceTransitionCut
		cut, err = controlSourceTransition(ctx, scope, channel, nonce, "initial", deadline)
		if cut != nil {
			t.Error("unexpected cut candidate")
			_ = cut.close()
		}
		if scenario == "complete" && err != nil {
			t.Fatal("genuine completed native channel", err)
		}
		if scenario != "complete" && err == nil {
			t.Fatal("unknown original outcome accepted")
		}
		if scenario == "unsafe-external-cut" && (!errors.Is(err, context.Canceled) || scope.reaped != 1 || !scope.status.Signaled() || scope.status.Signal() != syscall.SIGKILL) {
			t.Fatal("unsafe original did not retain cancellation and exact original SIGKILL join", scope.reaped, scope.status, err)
		}
		if err := scope.join(); err != nil {
			t.Fatal(err)
		}
		t.Log("actual coordinator joined; original outcome", scope.status, err)
		return
	}
	if err := channel.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := writeSourceTransitionMessage(channel, sourceTransitionMessage{Schema: "ardents-source-transition-hello-v1", Nonce: nonce, Operation: "initial", Deadline: deadline.UnixNano()}); err != nil {
		t.Fatal(err)
	}
	ready, err := readSourceTransitionMessage(channel, int32(scope.pid))
	if err != nil || ready != (sourceTransitionMessage{Schema: "ardents-source-transition-ready-v1", Nonce: nonce, Operation: "initial"}) {
		t.Fatal("original ready", ready, err)
	}
	boundary, err := readSourceTransitionMessage(channel, int32(scope.pid))
	if err != nil || boundary.Sequence != 1 || boundary.Records != 2 {
		t.Fatal("original synced boundary", boundary, err)
	}
	if err := scope.waitStopped(ctx); err != nil {
		t.Fatal("packet was not an actual kernel stop", err)
	}
	observedBoundary, err := observeSourceTransitionCut(ctx, boundary)
	if err != nil {
		t.Fatal(err)
	}
	if err := observedBoundary.close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(boundary.Journal)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := os.Open(boundary.Journal)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("frozen original lease was not held", err)
	}
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	if scenario == "cut-ack-outstanding" {
		frozen, err := observeSourceTransitionCut(ctx, boundary)
		if err != nil {
			t.Fatal(err)
		}
		defer frozen.close()
		control := &sourceTransitionControl{scope: scope, channel: channel}
		if err := control.freeze(ctx, frozen); err != nil {
			t.Fatal(err)
		}
		outstanding, err := control.acknowledge(ctx)
		if err != nil || outstanding != frozen {
			t.Fatal("original ACK ownership did not leave frozen state", err)
		}
		// Cancel after the real decision changed, even before an ACK reaches
		// the socket. Unchanged bytes and actual SIGKILL/join must not make a
		// new eligible cut out of an outstanding/possibly delivered ACK.
		cancel()
		control.deny()
		if err := outstanding.close(); err != nil {
			t.Fatal(err)
		}
		candidate, err := control.finish(ctx)
		if candidate != nil {
			t.Error("outstanding ACK manufactured an eligible original cut")
			_ = candidate.close()
		}
		if err != nil || scope.reaped != 1 || !scope.status.Signaled() || scope.status.Signal() != syscall.SIGKILL {
			t.Fatal("actual original scope did not join", scope.reaped, scope.status, err)
		}
		after, err := os.ReadFile(boundary.Journal)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("original unknown-ACK prefix changed", err)
		}
		controlPath, resultPath, guard := sourceCutPaths(boundary.Journal)
		for _, path := range []string{controlPath, resultPath, guard, filepath.Join(os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_ROOT"), "next-effect")} {
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("outstanding ACK published an effect or cut proof", path, err)
			}
		}
		t.Log("unchanged native prefix and joined SIGKILL refused after ACK ownership became outstanding")
		return
	}
	if err := scope.deny(); err != nil {
		t.Fatal(err)
	}
	if err := scope.resume(); err == nil {
		t.Fatal("denied original process resumed")
	}
	if err := scope.join(); err != nil {
		t.Fatal(err)
	}
	if !scope.status.Signaled() || scope.status.Signal() != syscall.SIGKILL {
		t.Fatal("original kill not joined", scope.status)
	}
	if scope.reaped != 2 {
		t.Fatal("original coordinator and its actual adopted descendant not both reaped", scope.reaped)
	}
	prefix, err := openSourceRecoveryPrefix(ctx, boundary.Journal, "initial")
	if err != nil {
		t.Fatal("fresh original exclusion after actual join", err)
	}
	if !bytes.Equal(before, prefix.journal.raw) || len(prefix.records) != 2 {
		t.Fatal("cut changed original prefix")
	}
	if err := prefix.close(); err != nil {
		t.Fatal(err)
	}
	// No ACK was sent; the child's next record/file effect never happened.
	if _, err := os.Lstat(filepath.Join(os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_ROOT"), "next-effect")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unacknowledged next effect occurred", err)
	}
	t.Log("actual frozen original killed/joined, fresh flock and exact bytes retained; no recovery proof published")
}

func sourceTransitionNativeCoordinator(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c, selected, err := OpenSourceTransitionCoordinator("initial")
	if err != nil || !selected {
		t.Fatal("genuine inherited private channel", selected, err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	}()
	ctx := c.Context()
	root := os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_ROOT")
	if scenario := os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_CASE"); scenario == "cut-frozen" || scenario == "complete-with-descendant" {
		// A physical-scope control, not an eligible semantic recovery point:
		// deliberately keep an actual same-group child alive. Only the real
		// original controller may kill and reap this descendant after adoption.
		command := exec.Command(os.Args[0], "-test.run=^TestSourceTransitionNativeDescendant$")
		command.Env = append(os.Environ(), "ARDENTS_SOURCE_TRANSITION_NATIVE_DESCENDANT=1")
		command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		for {
			if _, err := os.Lstat(filepath.Join(root, "descendant-ready")); err == nil {
				break
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			if err := waitSourceTransitionTick(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	journal := filepath.Join(root, "original.source-initial-journal")
	next := filepath.Join(root, "next-effect")
	births, err := openSourceRecoverableInitialBirths(ctx, journal, map[string]bool{next: true})
	if err != nil {
		t.Fatal(err)
	}
	defer births.close()
	if err := births.bindTransition(strings.Repeat("b", 64), strings.Repeat("c", 64), "initial", ""); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_CASE") == "sync-uncertain" {
		births.afterSync = func(*os.File) error { return errors.New("actual Sync result uncertain") }
	}
	if err := births.record("admitted", root, nil, strings.Repeat("c", 64), 0, 999, 999); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_CASE") == "external-cut" {
		// Native framing/process control only: repeated observations are not a
		// valid Source initial grammar. Their actual Sync/STOP/ACK cycles let
		// an outside observer choose a later physical frontier without an
		// accepting Source, account, manager, Release or State fixture.
		records := 2
		if value := os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_RECORDS"); value != "" {
			var err error
			records, err = strconv.Atoi(value)
			if err != nil || records < 2 || records > 55 {
				t.Fatal("finite native observation prefix required", err)
			}
		}
		for count := 2; count < records; count++ {
			if err := births.record("admitted", root, nil, strings.Repeat("c", 64), 0, 999, 999); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := births.write(next, []byte("original next effect\n"), 0600, 0, 0, ctx.Err); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_CASE") == "close-uncertain" {
		if err := births.journal.file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := births.close(); err != nil {
		t.Fatal("original real Close failure retained", err)
	}
	if os.Getenv("ARDENTS_SOURCE_PHYSICAL_OBSERVER_WINDOW") == "pending-before-unlink" {
		// Native file mechanism only: the outside observer must hold the real
		// Remove entry after successful original Close, before this leaf is lost.
		if err := os.Remove(next); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSourceTransitionNativeDescendant(t *testing.T) {
	if os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_DESCENDANT") != "1" {
		return
	}
	if err := os.WriteFile(filepath.Join(os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_ROOT"), "descendant-ready"), []byte("actual original descendant\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for {
		time.Sleep(time.Hour)
	}
}
