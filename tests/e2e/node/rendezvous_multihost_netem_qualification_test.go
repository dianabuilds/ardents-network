//go:build h4_2_multihost

package state_test

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const (
	h42NetemDelayMode    = "delay"
	h42NetemDropMode     = "drop"
	h42NetemImpairedMode = "impaired"
)

// TestH42MultiHostRendezvousKernelNetemRelay applies netem only inside a
// disposable VPS Docker namespace between direct Windows legs and the real
// remote product Rendezvous. It is not host loss, public-path loss, recovery,
// availability, NAT, MTU, reordering, active-probe, or fallback evidence.
func TestH42MultiHostRendezvousKernelNetemRelay(t *testing.T) {
	environment := requireH42MultiHostEnvironment(t)
	remoteEndpoint := net.JoinHostPort(environment.host, strconv.Itoa(environment.port))
	fixture := newRendezvousStateFixture(t, remoteEndpoint)
	stage := stageH42RemoteRendezvous(t, fixture, environment)
	buildH42LinuxNetemRelay(t, filepath.Join(stage, "netem-relay"))
	remote := h42RemoteRendezvous{environment: environment}
	t.Cleanup(func() { remote.remove(t) })
	remote.start(t, stage)
	remote.waitReady(t)

	t.Run("kernel delay retains exact carriage", func(t *testing.T) {
		relay := newH42NetemRelay(environment, "delay")
		t.Cleanup(func() { relay.remove(t) })
		relay.prepare(t, remote)
		relay.start(t, h42NetemDelayMode)
		relay.waitReady(t)
		initiator, responder := h42OpenNetemRelayPair(t, relay.endpoint(), fixture, 0xd1)
		defer h42CloseNetemRelayPair(t, initiator, responder)
		started := time.Now()
		h42CarryNetemRelayPayload(t, initiator, responder, "H4-2 kernel netem delayed exact carriage")
		if elapsed := time.Since(started); elapsed < 350*time.Millisecond || elapsed > 4*time.Second {
			t.Fatalf("two-way kernel netem delay elapsed %s, want [350ms,4s]", elapsed)
		}
		if statistics := relay.statistics(t); !strings.Contains(statistics, "qdisc netem") {
			t.Fatalf("netem relay statistics did not identify netem qdisc:\n%s", statistics)
		}
	})

	t.Run("kernel loss prevents attachment and records drops", func(t *testing.T) {
		relay := newH42NetemRelay(environment, "drop")
		t.Cleanup(func() { relay.remove(t) })
		relay.prepare(t, remote)
		relay.start(t, h42NetemDropMode)
		relay.waitReady(t)
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		_, err := openRendezvousProcessLeg(ctx, relay.endpoint(), fixture.initiator.certificate, fixture.rendezvous.public,
			fixture.leg([32]byte{0xd2}, route.InitiatorRole))
		if err == nil {
			t.Fatal("100% kernel netem loss unexpectedly exposed an authenticated attachment")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("100%% kernel netem loss = %v, want caller deadline exhaustion", err)
		}
		if statistics := relay.statistics(t); !regexp.MustCompile(`dropped [1-9][0-9]*`).MatchString(statistics) {
			t.Fatalf("kernel netem loss recorded no dropped packets:\n%s", statistics)
		}
	})

	t.Run("kernel delay loss reorder retains exact large carriage", func(t *testing.T) {
		relay := newH42NetemRelay(environment, "impaired")
		t.Cleanup(func() { relay.remove(t) })
		relay.prepare(t, remote)
		relay.start(t, h42NetemImpairedMode)
		relay.waitReady(t)
		initiator, responder := h42OpenNetemRelayPair(t, relay.endpoint(), fixture, 0xd3)
		defer h42CloseNetemRelayPair(t, initiator, responder)
		h42CarryNetemRelayPayload(t, initiator, responder, strings.Repeat("H4-2 netem ", (256<<10)/len("H4-2 netem ")))
		statistics := relay.statistics(t)
		if !strings.Contains(statistics, "loss 5%") || !strings.Contains(statistics, "reorder 10%") ||
			!regexp.MustCompile(`requeues [1-9][0-9]*`).MatchString(statistics) {
			t.Fatalf("kernel netem impaired carriage did not retain declared loss/reorder and observed requeues:\n%s", statistics)
		}
	})
}

func h42OpenNetemRelayPair(t *testing.T, endpoint string, fixture rendezvousStateFixture, marker byte) (*tls.Conn, *tls.Conn) {
	t.Helper()
	attachment := [32]byte{marker}
	initiator, err := openRendezvousProcessLeg(t.Context(), endpoint, fixture.initiator.certificate, fixture.rendezvous.public,
		fixture.leg(attachment, route.InitiatorRole))
	if err != nil {
		t.Fatalf("open kernel-netem Initiator leg: %v", err)
	}
	responder, err := openRendezvousProcessLeg(t.Context(), endpoint, fixture.responder.certificate, fixture.rendezvous.public,
		fixture.leg(attachment, route.ResponderRole))
	if err != nil {
		_ = initiator.Close()
		t.Fatalf("open kernel-netem Responder leg: %v", err)
	}
	return initiator, responder
}

func h42CloseNetemRelayPair(t *testing.T, initiator, responder *tls.Conn) {
	t.Helper()
	_ = initiator.Close()
	_ = responder.Close()
}

func h42CarryNetemRelayPayload(t *testing.T, initiator, responder *tls.Conn, payload string) {
	t.Helper()
	if _, err := initiator.Write([]byte(payload)); err != nil {
		t.Fatalf("write kernel-netem payload: %v", err)
	}
	if received := readProcessExact(t, responder, len(payload)); string(received) != payload {
		t.Fatalf("kernel-netem carriage = %q, want %q", received, payload)
	}
}

type h42NetemRelay struct {
	environment          h42MultiHostEnvironment
	container, directory string
}

func newH42NetemRelay(environment h42MultiHostEnvironment, suffix string) h42NetemRelay {
	return h42NetemRelay{environment: environment, container: environment.container + "-netem-" + suffix,
		directory: environment.remoteDirectory + "-netem-" + suffix}
}

func (relay h42NetemRelay) endpoint() string {
	return net.JoinHostPort(relay.environment.host, strconv.Itoa(relay.environment.port+3))
}

func (relay h42NetemRelay) prepare(t *testing.T, remote h42RemoteRendezvous) {
	t.Helper()
	command := fmt.Sprintf("set -eu; test ! -e %[1]s; mkdir -m 700 %[1]s; cp %[2]s/netem-relay %[1]s/netem-relay; chmod 755 %[1]s/netem-relay",
		h42ShellQuote(relay.directory), h42ShellQuote(remote.environment.remoteDirectory))
	if output, err := remote.run(t, command); err != nil {
		t.Fatalf("prepare isolated H4-2 kernel-netem relay: %v\n%s", err, output)
	}
}

func (relay h42NetemRelay) start(t *testing.T, mode string) {
	t.Helper()
	delay := ""
	if mode == h42NetemDelayMode {
		delay = " -delay 200ms"
	}
	command := fmt.Sprintf("set -eu; ! docker container inspect %[1]s >/dev/null 2>&1; docker run --detach --name %[1]s --cap-drop ALL --cap-add NET_ADMIN --pids-limit 64 --memory 128m --cpus 0.5 --read-only -p %[2]d:%[2]d -v %[3]s:/work:ro -v /usr/sbin/tc:/usr/sbin/tc:ro -v /lib/x86_64-linux-gnu:/lib/x86_64-linux-gnu:ro -v /lib64:/lib64:ro --workdir /work golang:1.26.6 /work/netem-relay -listen :%[2]d -target %[4]s -mode %[5]s%[6]s",
		h42ShellQuote(relay.container), relay.environment.port+3, h42ShellQuote(relay.directory),
		h42ShellQuote(net.JoinHostPort(relay.environment.host, strconv.Itoa(relay.environment.port))), h42ShellQuote(mode), delay)
	remote := h42RemoteRendezvous{environment: relay.environment}
	if output, err := remote.run(t, command); err != nil {
		t.Fatalf("start H4-2 kernel-netem relay: %v\n%s", err, output)
	}
}

func (relay h42NetemRelay) waitReady(t *testing.T) {
	t.Helper()
	remote := h42RemoteRendezvous{environment: relay.environment}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		logs, err := remote.run(t, fmt.Sprintf("docker logs %s 2>&1 || true", h42ShellQuote(relay.container)))
		if err == nil && strings.Contains(logs, "netem-relay-ready") {
			return
		}
		status, err := remote.run(t, fmt.Sprintf("docker inspect --format '{{.State.Status}}' %s", h42ShellQuote(relay.container)))
		if err != nil || strings.TrimSpace(status) != "running" {
			t.Fatalf("H4-2 kernel-netem relay exited before ready: %v / %s\n%s", err, status, logs)
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("H4-2 kernel-netem relay did not reach ready\n%s", relay.logs(t))
}

func (relay h42NetemRelay) statistics(t *testing.T) string {
	t.Helper()
	remote := h42RemoteRendezvous{environment: relay.environment}
	output, err := remote.run(t, fmt.Sprintf("docker exec %s /usr/sbin/tc -s qdisc show dev eth0", h42ShellQuote(relay.container)))
	if err != nil {
		t.Fatalf("read H4-2 kernel-netem statistics: %v\n%s", err, output)
	}
	return output
}

func (relay h42NetemRelay) logs(t *testing.T) string {
	t.Helper()
	remote := h42RemoteRendezvous{environment: relay.environment}
	output, _ := remote.run(t, fmt.Sprintf("docker logs %s 2>&1 || true", h42ShellQuote(relay.container)))
	return output
}

func (relay h42NetemRelay) remove(t *testing.T) {
	t.Helper()
	remote := h42RemoteRendezvous{environment: relay.environment}
	command := fmt.Sprintf("set -eu; docker rm -f %[1]s >/dev/null 2>&1 || true; if [ -e %[2]s ]; then rm -rf -- %[2]s; fi; test ! -e %[2]s; ! docker container inspect %[1]s >/dev/null 2>&1",
		h42ShellQuote(relay.container), h42ShellQuote(relay.directory))
	if output, err := remote.runCleanup(command); err != nil {
		t.Errorf("remove H4-2 kernel-netem relay: %v\n%s", err, output)
	}
}

func buildH42LinuxNetemRelay(t *testing.T, destination string) {
	t.Helper()
	command := exec.Command("go", "build", "-o", destination, "./tests/e2e/node/fixturecommand/netem-relay")
	command.Dir = filepath.Join("..", "..", "..")
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOARCH=amd64", "GOOS=linux", "GOTOOLCHAIN=local", "GOCACHE="+t.TempDir(), "GOMODCACHE="+t.TempDir())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("cross-build H4-2 kernel-netem relay: %v\n%s", err, output)
	}
	if err := os.Chmod(destination, 0o755); err != nil {
		t.Fatal(err)
	}
}
