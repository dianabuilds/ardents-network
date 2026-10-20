//go:build linux && node_process_native

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	routejoin "github.com/dianabuilds/ardents-network/internal/successor/route/join"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The independently pinned canonical commands provide both original Rendezvous
// invocations. Signed State, genuine Stock, Forwarding and Hosting provide the
// actual transport. No fixture supplies an accepting pair or native observation.
func TestNativeJoinDuplexAndReopenTCP(t *testing.T) { nativeJoinWorkload(t, "duplex") }
func TestNativeJoinUndercoverageBeforeSpendTCP(t *testing.T) {
	nativeJoinWorkload(t, "undercoverage")
}

func nativeJoinWorkload(t *testing.T, mode string) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			var provisioned map[[32]byte][2]string
			certificates := make(map[[32]byte]tls.Certificate)
			f, sockets, originalCertificates := newJoinRouteFixtureConfigured(t, carrier, func(f *networkAdmissionFixture) {
				if mode == "certificate" {
					provisioned = provisionNativeNodeCertificates(t, f, certificates, [32]byte{20}, [32]byte{21})
				}
			})
			for id, certificate := range certificates {
				originalCertificates[id] = certificate
			}
			certificates = originalCertificates
			userStock, publisherStock := joinRouteStock(t, f, admission.AllocationUser), joinRouteStock(t, f, admission.AllocationPublisher)
			servers, _ := startJoinRouteReceiverRange(t, f, sockets, certificates, 19)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			var clockOnce sync.Once
			stopClock := func() { clockOnce.Do(closeClock) }
			defer stopClock()
			processes := map[[32]byte]*nativeForwardingProcess{}
			plans := map[[32]byte]map[string]any{}
			before := map[[32]byte][]byte{}
			for _, id := range [][32]byte{{20}, {21}} {
				var cert, key string
				if pair, ok := provisioned[id]; ok {
					cert, key = pair[0], pair[1]
				} else {
					cert, key = nativeDescriptorCredential(t, certificates[id])
				}
				work := uint64(32 << 20)
				if mode == "undercoverage" {
					work--
				}
				plan := map[string]any{"network": seedNativeNodeState(t, f, bundle, clock), "node_id": id, "process_profile": processresource.JoinProfile, "participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": t.TempDir(), "hosting_root": routeProcessBudget(t), "certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: work}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
				sockets[id]()
				processes[id] = startNativeJoin(t, binary, hostingConfig(t, plan), id[0])
				plans[id] = plan
				before[id], err = os.ReadFile(filepath.Join(plan["spend_root"].(string), "closed-token-spends"))
				if err != nil {
					t.Fatal(err)
				}
			}
			makePlan := func(domain uint8) routePrefixPlan {
				root := t.TempDir()
				return routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), SourceInteriorRoot: filepath.Join(root, "source-interior"), HostingRoot: routeProcessBudget(t), Domain: domain, Deadline: time.Now().Add(60 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			}
			userPlan, publisherPlan := makePlan(1), makePlan(3)
			userContext, err := newRouteJoinContext(t.Context(), userPlan, f.authority, userStock)
			if err != nil {
				t.Fatal(err)
			}
			defer userContext.close()
			user, err := userContext.open(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer user.close()
			publisherContext, err := newRouteJoinContext(t.Context(), publisherPlan, f.authority, publisherStock)
			if err != nil {
				t.Fatal(err)
			}
			defer publisherContext.close()
			publisher, err := publisherContext.open(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer publisher.close()
			recipient, err := user.recipient(0)
			if err != nil {
				t.Fatal(err)
			}
			intent := routeJoinIntent{Node: recipient.Node, Generation: recipient.Generation, Deadline: minRouteDeadline(userPlan.Deadline, publisherPlan.Deadline, recipient.NotAfter), SetupDeadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}
			if _, err := rand.Read(intent.Secret[:]); err != nil {
				t.Fatal(err)
			}
			if _, err := rand.Read(intent.Context[:]); err != nil {
				t.Fatal(err)
			}
			publisherIntent := intent
			if mode == "context" || mode == "secret" {
				intent.SetupDeadline = time.Now().Add(3 * time.Second).UTC().Truncate(time.Second)
				publisherIntent = intent
				if mode == "context" {
					publisherIntent.Context[0] ^= 1
				} else {
					publisherIntent.Secret[0] ^= 1
				}
			}

			type outcome struct {
				side   int
				stream net.Conn
				err    error
			}
			opened := make(chan outcome, 2)
			go func() { s, e := user.join(t.Context(), intent); opened <- outcome{0, s, e} }()
			go func() { s, e := publisher.join(t.Context(), publisherIntent); opened <- outcome{1, s, e} }()
			var streams [2]net.Conn
			var failure error
			for range 2 {
				r := <-opened
				streams[r.side] = r.stream
				failure = errors.Join(failure, r.err)
			}
			defer func() {
				for _, s := range streams {
					if s != nil {
						_ = s.Close()
					}
				}
			}()
			if mode == "undercoverage" {
				if failure == nil {
					t.Fatal("undercovered complete class admitted")
				}
				for id, plan := range plans {
					after, e := os.ReadFile(filepath.Join(plan["spend_root"].(string), "closed-token-spends"))
					if e != nil || !bytes.Equal(before[id], after) {
						t.Fatal("undercoverage spent durable rights", e)
					}
				}
				_ = user.close()
				_ = publisher.close()
				_ = userContext.close()
				_ = publisherContext.close()
				for _, p := range processes {
					p.stop(t, true)
				}
				return
			}
			if mode == "context" || mode == "secret" {
				if failure == nil {
					t.Fatal("mismatched original JOIN accepted")
				}
				for id, plan := range plans {
					if id == recipient.Node {
						raw, e := os.ReadFile(filepath.Join(plan["spend_root"].(string), "closed-token-spends"))
						if e != nil || len(raw) <= len(before[id]) {
							t.Fatal("genuine refusal never spent", e)
						}
					}
				}
				_ = user.close()
				_ = publisher.close()
				_ = userContext.close()
				_ = publisherContext.close()
				for _, p := range processes {
					p.stop(t, true)
				}
				return
			}

			if failure != nil {
				t.Fatal("actual original paired JOIN", failure)
			}
			// More than one credit window, bounded duplex bytes and directional EOF.
			for side := range 2 {
				payload := bytes.Repeat([]byte{byte(71 + side)}, 96<<10)
				done := make(chan error, 1)
				go func() { _, e := streams[side].Write(payload); done <- e }()
				got := make([]byte, len(payload))
				_, e := io.ReadFull(streams[1-side], got)
				if e = errors.Join(e, <-done); e != nil || !bytes.Equal(got, payload) {
					t.Fatal("native JOIN duplex byte oracle", e)
				}
				if side == 0 && (mode == "caller" || mode == "process-loss" || mode == "clock-loss") {
					p := processes[recipient.Node]
					if mode == "caller" {
						p.stop(t, true)
					} else {
						if mode == "clock-loss" {
							stopClock()
						} else if e := exec.Command("systemctl", "set-property", "--runtime", p.unit, "CPUQuota=50%").Run(); e != nil {
							t.Fatal(e)
						}
						e := p.command.Wait()
						p.joined = true
						_ = p.stdout.Close()
						var exit *exec.ExitError
						var event map[string]string
						if !errors.As(e, &exit) || exit.ExitCode() != 1 || json.Unmarshal([]byte(p.diagnostic.String()), &event) != nil || event["outcome"] != "failed" {
							t.Fatal("original post-I/O loss became success", e, p.diagnostic.String())
						}
					}
					for _, stream := range streams {
						_ = stream.Close()
					}
					_ = user.close()
					_ = publisher.close()
					_ = userContext.close()
					_ = publisherContext.close()
					for id, plan := range plans {
						raw, e := os.ReadFile(filepath.Join(plan["spend_root"].(string), "closed-token-spends"))
						if e != nil || id == recipient.Node && len(raw) <= len(before[id]) {
							t.Fatal("post-I/O loss refunded spends", e)
						}
					}
					if mode != "clock-loss" {
						for _, other := range processes {
							if !other.joined {
								other.stop(t, true)
							}
						}
					}
					return
				}

				if e := streams[side].(*routejoin.Joined).CloseWrite(); e != nil {
					t.Fatal(e)
				}
				var one [1]byte
				if n, e := streams[1-side].Read(one[:]); n != 0 || e != io.EOF {
					t.Fatal("directional EOF", n, e)
				}
			}
			for _, s := range streams {
				if err := s.Close(); err != nil {
					t.Fatal("typed CLOSE", err)
				}
			}
			if err := errors.Join(wrapNativeJoinClose("user", user.close()), wrapNativeJoinClose("publisher", publisher.close()), wrapNativeJoinClose("user-context", userContext.close()), wrapNativeJoinClose("publisher-context", publisherContext.close())); err != nil {
				t.Fatal(err)
			}
			for _, server := range servers {
				if err := server.Close(); err != nil {
					t.Fatal(err)
				}
			}
			for id, p := range processes {
				p.stop(t)
				plan := plans[id]
				after, e := os.ReadFile(filepath.Join(plan["spend_root"].(string), "closed-token-spends"))
				if e != nil {
					t.Fatal(e)
				}
				if id == recipient.Node && len(after) <= len(before[id]) {
					t.Fatal("no independent receiving spend")
				}
				budget, e := hosting.Open(plan["hosting_root"].(string))
				if e != nil {
					t.Fatal(e)
				}
				snapshot, e := budget.Observe(t.Context())
				if closeErr := budget.Close(); e != nil || closeErr != nil || snapshot.ReservedBytes != 0 {
					t.Fatal("unjoined original reservations", e, closeErr, snapshot.ReservedBytes)
				}
				// A separate invocation reopens exact roots and the same ephemeral endpoint.
				fresh := startNativeJoin(t, binary, hostingConfig(t, plan), id[0])
				fresh.stop(t)
				retained, e := os.ReadFile(filepath.Join(plan["spend_root"].(string), "closed-token-spends"))
				if e != nil || !bytes.Equal(retained, after) {
					t.Fatal("reopen reset spends", e)
				}
			}
		})
	}
}

func launchNativeJoin(t *testing.T, binary, plan string, index byte) *nativeForwardingProcess {
	t.Helper()
	unit := fmt.Sprintf("ardents-join-%d-%d.service", os.Getpid(), index)
	p := &nativeForwardingProcess{unit: unit}
	p.command = exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", binary, "node", "join", "--config", plan)
	p.command.Stderr = &p.diagnostic
	var err error
	p.stdout, err = p.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !p.joined {
			_ = exec.Command("systemctl", "stop", unit).Run()
			_ = p.command.Wait()
			_ = p.stdout.Close()
			p.joined = true
		}
		if t.Failed() {
			t.Log("original Join retained diagnostic", p.diagnostic.String())
		}
	})
	return p
}

func startNativeJoin(t *testing.T, binary, plan string, index byte) *nativeForwardingProcess {
	t.Helper()
	p := launchNativeJoin(t, binary, plan, index)
	var event map[string]string
	if err := json.NewDecoder(p.stdout).Decode(&event); err != nil || event["operation"] != "node.join" || event["phase"] != "listening" {
		t.Fatal("genuine original Join startup refused", event, err, p.diagnostic.String())
	}
	raw, err := exec.Command("systemctl", "show", p.unit, "-p", "MainPID", "--value").Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Sscanf(string(raw), "%d", &p.pid); err != nil || p.pid < 2 {
		t.Fatal("original Join MainPID unavailable", err)
	}
	return p
}

func TestNativeJoinOriginalCommandLossAfterIOTCP(t *testing.T) {
	for _, mode := range []string{"caller", "process-loss", "clock-loss"} {
		t.Run(mode, func(t *testing.T) { nativeJoinWorkload(t, mode) })
	}
}
func TestNativeJoinMismatchedPairRefusesTCP(t *testing.T) {
	for _, mode := range []string{"context", "secret"} {
		t.Run(mode, func(t *testing.T) { nativeJoinWorkload(t, mode) })
	}
}

func wrapNativeJoinClose(owner string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("original %s close: %w", owner, err)
}
