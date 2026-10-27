//go:build linux && network_signing_native

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"golang.org/x/sys/unix"
)

// This root driver executes only the canonical installed command. The outside
// controller owns the private manager and every original descendant through
// join. Matching a receipt is a consistency check, never PO bootstrap consent.
// Execution requires that separate direct decision before this driver is run.
func TestForwardingInstalledNativeLifetimeAndRefill(t *testing.T) {
	runForwardingInstalledLifetime(t, "lifetime-refill")
}

func TestForwardingInstalledNativeRetirementHistoryMissing(t *testing.T) {
	runForwardingInstalledLifetime(t, "retirement-history-missing")
}

func TestForwardingInstalledNativeRetirementHistoryDamaged(t *testing.T) {
	runForwardingInstalledLifetime(t, "retirement-history-damaged")
}

func TestForwardingInstalledNativeUnknownDomainJoin(t *testing.T) {
	runForwardingInstalledLifetime(t, "unknown-domain-join")
}

func runForwardingInstalledLifetime(t *testing.T, selected string) {
	t.Helper()
	const base = "/var/lib/forwarding-coherent-initial"
	if os.Getenv("ARDENTS_FORWARDING_INSTALLED_CASE") != selected {
		t.Fatal("explicit case differs from the original prepared stand")
	}
	forwardingInstalledPrivateStand(t)
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	presentedPath := filepath.Join(base, "delivery/forwarding-presentations.json")
	presentedInfo, err := os.Lstat(presentedPath)
	if err != nil || presentedInfo.Mode() != 0600 || presentedInfo.Sys().(*syscall.Stat_t).Uid != 0 || presentedInfo.Sys().(*syscall.Stat_t).Nlink != 1 {
		t.Fatal("original private presentation custody differs", err)
	}
	presentedRaw := read(presentedPath)
	defer clear(presentedRaw)
	var input forwardingInstalledChannels
	defer func() {
		for _, channel := range input.Channels {
			for _, token := range channel.Tokens {
				clear(token)
			}
		}
	}()
	decode := json.NewDecoder(bytes.NewReader(presentedRaw))
	decode.DisallowUnknownFields()
	if err := decode.Decode(&input); err != nil {
		t.Fatal(err)
	}
	if err := decode.Decode(new(any)); err != io.EOF || input.Schema != "ardents-forwarding-test-presentations-v1" || !input.Deadline.After(time.Now().Add(time.Minute)) || input.Channels[0].Nonce == input.Channels[1].Nonce {
		t.Fatal("invalid environment: exact finite current channel inputs required", err)
	}
	canonical, err := json.Marshal(input)
	canonical = append(canonical, '\n')
	defer clear(canonical)
	if err != nil || !bytes.Equal(canonical, presentedRaw) {
		t.Fatal("noncanonical or nonfinite private channel input", err)
	}
	digests := make(map[[32]byte]bool)
	for _, channel := range input.Channels {
		if channel.Nonce == [32]byte{} {
			t.Fatal("missing original local channel nonce")
		}
		for _, token := range channel.Tokens {
			sum := sha256.Sum256(token)
			if len(token) != 354 || digests[sum] {
				t.Fatal("missing or repeated original finite credential")
			}
			digests[sum] = true
		}
	}
	installation := filepath.Join(base, "runtime/node")
	bindingPath := filepath.Join(installation, "binding.json")
	var binding struct {
		Generation string
		UID, GID   uint32
	}
	if json.Unmarshal(read(bindingPath), &binding) != nil || len(binding.Generation) != 64 || binding.UID < 100 || binding.UID > 999 || binding.GID != binding.UID {
		t.Fatal("invalid environment: original completed Node binding required")
	}
	planPath := filepath.Join(installation, "generations", binding.Generation, "forwarding-plan.json")
	plan, err := forwarding.DecodePlan(read(planPath))
	if err != nil {
		t.Fatal(err)
	}
	peerPath := filepath.Join(base, "delivery/forwarding-peer-presentations.json")
	peerInfo, err := os.Lstat(peerPath)
	if err != nil || peerInfo.Mode() != 0600 || peerInfo.Sys().(*syscall.Stat_t).Uid != 0 || peerInfo.Sys().(*syscall.Stat_t).Nlink != 1 {
		t.Fatal("original private Interior presentations differ", err)
	}
	peerRaw := read(peerPath)
	defer clear(peerRaw)
	var peer forwardingInstalledChannels
	if json.Unmarshal(peerRaw, &peer) != nil || peer.Schema != input.Schema || peer.NetworkID != input.NetworkID || peer.StateGeneration != input.StateGeneration || peer.StateDigest != input.StateDigest || peer.ProfileDigest != input.ProfileDigest || peer.Deadline != input.Deadline || peer.NodeID == input.NodeID || peer.PublicKey == input.PublicKey {
		t.Fatal("Interior input scope differs from canonical cohort")
	}
	peerCanonical, err := json.Marshal(peer)
	peerCanonical = append(peerCanonical, '\n')
	defer clear(peerCanonical)
	if err != nil || !bytes.Equal(peerRaw, peerCanonical) {
		t.Fatal("noncanonical Interior input", err)
	}
	defer func() {
		for _, c := range peer.Channels {
			for _, token := range c.Tokens {
				clear(token)
			}
		}
	}()
	nonces := map[[32]byte]bool{input.Channels[0].Nonce: true, input.Channels[1].Nonce: true}
	for _, c := range peer.Channels {
		if c.Nonce == [32]byte{} || nonces[c.Nonce] {
			t.Fatal("Interior channel nonce aliases another original channel")
		}
		nonces[c.Nonce] = true
		for _, token := range c.Tokens {
			sum := sha256.Sum256(token)
			if len(token) != 354 || digests[sum] {
				t.Fatal("Interior credential absent or reused")
			}
			digests[sum] = true
		}
	}
	peerPlanRaw := read(filepath.Join(base, "component-peer/forwarding-plan.json"))
	peerPlan, err := forwarding.DecodePlan(peerPlanRaw)
	if err != nil || peerPlan.NodeID != peer.NodeID || peerPlan.HostingRoot != plan.HostingRoot || peerPlan.Network.Root == plan.Network.Root || peerPlan.Network.NetworkID != plan.Network.NetworkID {
		t.Fatal("separate Interior State/shared provider plan differs", err)
	}
	forwardingInstalledBootstrapInputs(t, map[string][]byte{
		"presentations": presentedRaw, "binding": read(bindingPath), "plan": read(planPath),
		"bundle": read(filepath.Join(base, "bundle.bin")), "profile": read(filepath.Join(base, "profile.bin")),
		"peer_presentations": peerRaw, "peer_plan": peerPlanRaw, "peer_certificate": read(filepath.Join(base, "delivery/peer-certificate/node-certificate.pem")),
	})
	program := filepath.Join(base, "delivery/distribution/ardents-node-linux-amd64")
	distribution, history := filepath.Dir(program), filepath.Join(base, "release-history")
	forwardingInstalledArtifact(t, program, "ardents-node")
	originals := make(map[string]os.FileInfo)
	for _, path := range []string{plan.Network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.Certificate, plan.PrivateKey, planPath, bindingPath, "/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow", "/etc/systemd/system/ardents-forwarding.service"} {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		originals[path] = info
	}
	fixed := make(map[string][]byte)
	for _, path := range []string{plan.Certificate, planPath, bindingPath, "/etc/passwd", "/etc/group", "/etc/systemd/system/ardents-forwarding.service"} {
		fixed[path] = read(path)
	}
	defer func() {
		for _, raw := range fixed {
			clear(raw)
		}
	}()
	stopClock, clockDone := make(chan struct{}), make(chan error, 1)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			now := time.Now()
			if err := os.Chtimes(plan.Network.ClockObservationFile, now, now); err != nil {
				clockDone <- err
				return
			}
			select {
			case <-stopClock:
				clockDone <- nil
				return
			case <-ticker.C:
			}
		}
	}()
	defer func() {
		close(stopClock)
		if err := <-clockDone; err != nil {
			t.Error("original clock writer", err)
		}
	}()
	// A fresh client owner reauthenticates actual retained signed input. The
	// prepared JSON and an earlier verified observation supply no live permit.
	keys := make(map[[32]byte]ed25519.PublicKey)
	for _, public := range plan.Network.Authorities {
		keys[sha256.Sum256(public[:])] = append(ed25519.PublicKey(nil), public[:]...)
	}
	client, err := state.OpenRetained(state.Config{Root: filepath.Join(base, "delivery/client-state"), NetworkID: plan.Network.NetworkID, Authorities: keys, Threshold: plan.Network.Threshold, ClosedProfileAuthority: plan.Network.ProfileAuthority[:], AcceptedProfile: "ardents-route-v3", Clock: time.Now, ClockObservationFile: plan.Network.ClockObservationFile})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	}()
	current := func() {
		t.Helper()
		view, err := client.CurrentRuntime()
		if err != nil {
			t.Fatal(err)
		}
		profile := view.Profile()
		member, err := view.Member(input.NodeID, view.ObservedAt())
		if err != nil || profile.Network != input.NetworkID || profile.Generation != input.StateGeneration || profile.EpochDigest != input.StateDigest || profile.Digest != input.ProfileDigest || member.NodeID != plan.NodeID || member.PublicKey != input.PublicKey || member.Endpoint != input.Endpoint || member.DutyGeneration != input.DutyGeneration || member.RoleDomain != 4 || member.Subrole != 1 || member.CarrierProfile != string(transport.ClosedCarrierTCP) {
			t.Fatal("fresh signed client facts differ", err)
		}
		interior, err := view.Member(peer.NodeID, view.ObservedAt())
		if err != nil || interior.PublicKey != peer.PublicKey || interior.Endpoint != peer.Endpoint || interior.DutyGeneration != peer.DutyGeneration || interior.RoleDomain != 4 || interior.Subrole != 2 || interior.CarrierProfile != string(transport.ClosedCarrierTCP) || interior.FamilyID == member.FamilyID {
			t.Fatal("fresh signed Interior facts differ", err)
		}
	}
	command := func(expected string, args ...string) {
		t.Helper()
		// Do not use CommandContext to kill a root controller merely because a
		// wait expired. Its real command retains original cleanup until join;
		// the outside private-scope controller owns a failed test's final join.
		raw, err := exec.Command(program, append([]string{"forwarding"}, args...)...).CombinedOutput()
		if err != nil || !bytes.Equal(raw, []byte(expected)) {
			t.Fatal("canonical installed operation", args[0], err, string(raw))
		}
	}
	refuse := func(args ...string) {
		t.Helper()
		raw, err := exec.Command(program, append([]string{"forwarding"}, args...)...).CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || len(raw) > 4096 || bytes.Contains(raw, []byte(`"outcome"`)) {
			t.Fatal("canonical refusal unavailable", args[0], err)
		}
	}
	var previousInvocation string
	var previousSpend []byte
	var previousPeerSpend []byte
	var previousUsed uint64
	historyFiles := make(map[string][]byte)
	var previousChallenge string
	for invocation := range 2 {
		current()
		reference := forwardingInstalledRetainUnit(t)
		peerProcess := forwardingInstalledStartInterior(t, base, binding.UID, peer, peerPlan, invocation != 0)
		peerSpendPath := filepath.Join(peerPlan.SpendRoot, "closed-token-spends")
		peerBefore := read(peerSpendPath)
		forwardingInstalledSpend(t, peerBefore, peer, invocation*2)
		if invocation != 0 && !bytes.Equal(peerBefore, previousPeerSpend) {
			t.Fatal("Interior restart changed original spends")
		}
		operation, outcome := "start-initial", "forwarding-initial-start-completed"
		if invocation != 0 {
			operation, outcome = "restart", "forwarding-same-generation-restart-completed"
		}
		command(fmt.Sprintf("{\"outcome\":%q,\"runtime_authority\":false}\n", outcome), operation, distribution, history, "alpha", "coherent-private-forwarding", bindingPath)
		manager := forwardingInstalledManager(t)
		if manager["InvocationID"] == "" || manager["InvocationID"] == previousInvocation {
			t.Fatal("fresh same-generation invocation missing")
		}
		previousInvocation = manager["InvocationID"]
		pid, err := strconv.Atoi(manager["MainPID"])
		if err != nil || pid <= 1 {
			t.Fatal("actual fixed Node process missing", err)
		}
		forwardingInstalledWaitListening(t, manager["InvocationID"], pid)
		pin := forwardingInstalledPinProcess(t, pid, manager["ControlGroup"], binding.UID)
		defer pin.close(t)
		defer func(invocation string, pid int) {
			if !t.Failed() {
				return
			}
			// Preserve this original invocation's bounded public events before
			// the outside controller joins its private volatile journal.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			raw, err := exec.CommandContext(ctx, "journalctl", "--no-pager", "--output=cat", "_SYSTEMD_INVOCATION_ID="+invocation, "_PID="+strconv.Itoa(pid)).CombinedOutput()
			if len(raw) > 8192 {
				raw = raw[len(raw)-8192:]
			}
			t.Log("original installed invocation events", string(raw), err)
		}(previousInvocation, pid)
		startDirectory := "forwarding-startup"
		startJournal := installation + ".forwarding-first-start-journal"
		if invocation != 0 {
			startDirectory = fmt.Sprintf("forwarding-invocation-%08d", invocation)
			startJournal = installation + fmt.Sprintf(".forwarding-invocation-%08d-journal", invocation)
		}
		archivePath := filepath.Join(installation, startDirectory, "start-archived.json")
		archiveRaw := read(archivePath)
		var archive struct {
			Schema           string `json:"schema"`
			BindingSHA256    string `json:"binding_sha256"`
			Generation       string `json:"generation"`
			PID              uint32 `json:"pid"`
			InvocationID     string `json:"invocation_id"`
			ProcessDevice    uint64 `json:"process_device"`
			ProcessInode     uint64 `json:"process_inode"`
			CgroupDevice     uint64 `json:"cgroup_device"`
			CgroupInode      uint64 `json:"cgroup_inode"`
			Manager          string `json:"manager"`
			StartedRealtime  uint64 `json:"started_realtime"`
			StartedMonotonic uint64 `json:"started_monotonic"`
		}
		bindingHash := sha256.Sum256(fixed[bindingPath])
		processNative, groupNative := pin.processInfo.Sys().(*syscall.Stat_t), pin.groupInfo.Sys().(*syscall.Stat_t)
		if json.Unmarshal(archiveRaw, &archive) != nil || archive.Schema != "ardents-forwarding-start-archive-v1" || archive.BindingSHA256 != hex.EncodeToString(bindingHash[:]) || archive.Generation != binding.Generation || archive.PID != uint32(pid) || archive.InvocationID != previousInvocation || archive.ProcessDevice != uint64(processNative.Dev) || archive.ProcessInode != processNative.Ino || archive.CgroupDevice != uint64(groupNative.Dev) || archive.CgroupInode != groupNative.Ino || archive.Manager != forwardingInstalledManagerIdentity(t) || archive.StartedRealtime == 0 || archive.StartedMonotonic == 0 {
			t.Fatal("start archive differs from independently retained actual invocation")
		}
		forwardingInstalledJournal(t, startJournal, invocation, false)
		current()
		conn, err := routeTestOpenEndpoint(t.Context(), transport.ClosedRoleCarrierRequest{CarrierProfile: transport.ClosedCarrierTCP, Endpoint: input.Endpoint, ExpectedServer: input.PublicKey, Deadline: input.Deadline})
		if err != nil {
			t.Fatal("actual installed TLS contact", err)
		}
		// Channel cancellation and the driver retain the same first physical
		// Close result; a later raw TLS Close must not create another write.
		conn = transport.Retain(conn)
		defer conn.Close()
		if err := conn.SetDeadline(time.Now().Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		h := ardp.Hello{NetworkID: input.NetworkID, StateGeneration: input.StateGeneration, StateDigest: input.StateDigest, ProfileDigest: input.ProfileDigest, RecipientNodeID: input.NodeID, RecipientDutyGeneration: input.DutyGeneration, Purpose: ardp.PurposeForwarding, Deadline: input.Deadline, ChannelNonce: input.Channels[invocation].Nonce}
		hello, err := ardp.EncodeHello(h)
		if err != nil {
			t.Fatal(err)
		}
		if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindHello, Body: hello}); err != nil {
			t.Fatal(err)
		}
		spendPath := filepath.Join(plan.SpendRoot, "closed-token-spends")
		before := read(spendPath)
		forwardingInstalledSpend(t, before, input, invocation*2)
		if invocation != 0 && !bytes.Equal(previousSpend, before) {
			t.Fatal("restart reset or changed earlier spends")
		}
		var initialReserved uint64
		for number, token := range input.Channels[invocation].Tokens {
			current()
			if len(token) != 354 {
				t.Fatal("finite genuine token missing")
			}
			if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, token...)}); err != nil {
				t.Fatal(err)
			}
			ack, err := ardp.ReadFrame(conn)
			if err != nil {
				t.Fatalf("actual installed parent admission/refill %d: %v", number, err)
			}
			status, credit, err := ardp.DecodeAcceptFrame(ack)
			if err != nil || status != 0 || credit != 65536 {
				t.Fatal("actual installed admission/refill refused", status, credit, err)
			}
			spent := read(spendPath)
			forwardingInstalledSpend(t, spent, input, invocation*2+number+1)
			if !bytes.HasPrefix(spent, before) {
				t.Fatal("successful spend lost original prefix")
			}
			before = spent
			used, reserved := forwardingInstalledBudget(t, plan.HostingRoot)
			if used < previousUsed {
				t.Fatal("provider usage floor decreased")
			}
			previousUsed = used
			if number == 0 {
				initialReserved = plan.Work.Tx + plan.Work.Rx + plan.Termination.Tx + plan.Termination.Rx
				if reserved != initialReserved {
					t.Fatal("complete original physical coverage differs", reserved, initialReserved)
				}
			} else if reserved != initialReserved+988 {
				// Independent fixed wire debit: initial HELLO/ADMIT/ACCEPT617
				// plus the refill ADMIT371, without another termination reserve.
				t.Fatal("actual parent refill physical coverage differs", reserved, initialReserved+988)
			}
		}
		previousSpend = before
		// Transfer the sole parent reader to the genuine framing owner. A
		// successful refill replaced its allowance before the final ACCEPT debit.
		check := func() error {
			actual, err := client.CurrentRuntime()
			if err != nil {
				return err
			}
			if actual.Profile().Digest != input.ProfileDigest {
				return errors.New("original holder profile changed")
			}
			for _, expected := range []forwardingInstalledChannels{input, peer} {
				m, err := actual.Member(expected.NodeID, actual.ObservedAt())
				if err != nil || m.PublicKey != expected.PublicKey || m.DutyGeneration != expected.DutyGeneration {
					return errors.Join(errors.New("original holder member changed"), err)
				}
			}
			return nil
		}
		parent := channel.New(t.Context(), conn, input.Deadline, admission.ForwardClass.ByteLimit()-uint64(ardp.HeaderSize+5), check, false, channel.NewBudget(64<<20), nil)
		defer parent.Close()
		lane, err := parent.Open(t.Context(), t.Context(), ardp.EncodeOpen(ardp.Open{RecipientNodeID: peer.NodeID, RecipientDutyGeneration: peer.DutyGeneration, Purpose: uint8(ardp.PurposeForwarding), Deadline: peer.Deadline}, false))
		if err != nil {
			t.Fatal("actual installed forwarding child", err)
		}
		defer lane.Finish()
		progress := &forwardingInstalledTLSObservation{Conn: lane}
		tlsStarted := time.Now()
		secured, err := routeFixtureRoleTLS(t, progress, peer.PublicKey, peer.Deadline)
		if err != nil {
			handshakeElapsed := time.Since(tlsStarted)
			childResult := lane.Close()
			lane.Finish()
			parentResult := parent.Close()
			t.Log("original inner TLS failure after joined local holders", "elapsed_ms", handshakeElapsed.Milliseconds(), "received_bytes", progress.received.Load(), "sent_bytes", progress.sent.Load(), "child", childResult, "parent", parentResult)
			t.Fatal("actual forwarded Interior TLS", err)
		}
		inner := transport.Retain(secured)
		defer inner.Close()
		innerHello, err := ardp.EncodeHello(ardp.Hello{NetworkID: peer.NetworkID, StateGeneration: peer.StateGeneration, StateDigest: peer.StateDigest, ProfileDigest: peer.ProfileDigest, RecipientNodeID: peer.NodeID, RecipientDutyGeneration: peer.DutyGeneration, Purpose: ardp.PurposeForwarding, Deadline: peer.Deadline, ChannelNonce: peer.Channels[invocation].Nonce})
		if err != nil {
			t.Fatal(err)
		}
		if err := ardp.WriteFrame(inner, ardp.Frame{Kind: ardp.KindHello, Body: innerHello}); err != nil {
			t.Fatal("forwarded HELLO", err)
		}
		for number, token := range peer.Channels[invocation].Tokens {
			current()
			if err := ardp.WriteFrame(inner, ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, token...)}); err != nil {
				t.Fatal("forwarded genuine admission", err)
			}
			ack, err := ardp.ReadFrame(inner)
			if err != nil {
				t.Fatal("actual Interior admission/refill through installed Forwarding", err)
			}
			status, credit, err := ardp.DecodeAcceptFrame(ack)
			if err != nil || status != 0 || credit != 65536 {
				t.Fatal("actual Interior admission/refill refused", status, credit, err)
			}
			peerAfter := read(peerSpendPath)
			forwardingInstalledSpend(t, peerAfter, peer, invocation*2+number+1)
			if !bytes.HasPrefix(peerAfter, peerBefore) {
				t.Fatal("Interior successful spend lost original prefix")
			}
			peerBefore = peerAfter
		}
		previousPeerSpend = peerBefore
		_, sharedReserved := forwardingInstalledBudget(t, plan.HostingRoot)
		wantReserved := plan.Work.Tx + plan.Work.Rx + plan.Termination.Tx + plan.Termination.Rx + peerPlan.Work.Tx + peerPlan.Work.Rx + peerPlan.Termination.Tx + peerPlan.Termination.Rx + 2*988
		if sharedReserved != wantReserved {
			t.Fatal("complete two-leg coverage in the original shared budget differs", sharedReserved, wantReserved)
		}
		// A concurrent explicit restart cannot replace this still-live original,
		// whose real parent, forwarded child and both reservations remain held.
		installationBefore, err := os.ReadDir(installation)
		if err != nil {
			t.Fatal(err)
		}
		journalsBefore, err := os.ReadDir(filepath.Dir(installation))
		if err != nil {
			t.Fatal(err)
		}
		startBefore := read(startJournal)
		refused, restartErr := exec.Command(program, "forwarding", "restart", distribution, history, "alpha", "coherent-private-forwarding", bindingPath).CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(restartErr, &exit) || exit.ExitCode() != 2 || len(refused) > 4096 || bytes.Contains(refused, []byte(`"outcome"`)) {
			t.Fatal("active original did not deny concurrent restart", restartErr)
		}
		installationAfter, err := os.ReadDir(installation)
		if err != nil || len(installationBefore) != len(installationAfter) {
			t.Fatal("refused active restart created an installation attempt", err)
		}
		for index, entry := range installationBefore {
			if entry.Name() != installationAfter[index].Name() {
				t.Fatal("refused active restart changed installation inventory")
			}
		}
		journalsAfter, err := os.ReadDir(filepath.Dir(installation))
		if err != nil || len(journalsBefore) != len(journalsAfter) {
			t.Fatal("refused active restart created a sibling attempt journal", err)
		}
		for index, entry := range journalsBefore {
			if entry.Name() != journalsAfter[index].Name() {
				t.Fatal("refused active restart changed sibling journal inventory")
			}
		}
		stillOriginal := forwardingInstalledManager(t)
		_, retainedReserved := forwardingInstalledBudget(t, plan.HostingRoot)
		if stillOriginal["InvocationID"] != previousInvocation || stillOriginal["MainPID"] != manager["MainPID"] || stillOriginal["ActiveState"] != "active" || retainedReserved != wantReserved || !bytes.Equal(read(startJournal), startBefore) || !bytes.Equal(read(spendPath), previousSpend) || !bytes.Equal(read(peerSpendPath), previousPeerSpend) {
			t.Fatal("refused concurrent restart altered the held original, spends or reservations")
		}
		// Retain the real peer and its original native scope through root Stop.
		// Closing the client first would manufacture a peer-reset failure.
		unknownJoin := selected == "unknown-domain-join"
		if unknownJoin {
			// A real crash removes native users without manufacturing the missing
			// domain report. Signal the retained original pidfd, never an auxiliary
			// process set or a PID that could have been reused. Native join follows.
			if err := unix.PidfdSendSignal(pin.pidfd, unix.SIGKILL, nil, 0); err != nil {
				t.Fatal("original invocation crash control", err)
			}
			pin.waitAfterCrash(t)
		} else {
			command("{\"outcome\":\"forwarding-original-stop-completed\",\"restart_admitted\":false}\n", "stop", history, bindingPath)
		}
		pin.joined(t)
		if unknownJoin {
			refuse("stop", history, bindingPath)
		}
		// Stop deliberately interrupts this holder's still-live channel. Retain
		// its operational outcome separately; physical cleanup must still join.
		parentResult := parent.Close()
		if err := parent.PhysicalFailure(); err != nil {
			if !unknownJoin {
				t.Fatal("original holder physical join", err)
			}
			t.Log("retained original holder cleanup failure after crash", err)
		}
		if parentResult != nil {
			t.Log("holder outcome after original installed native join", parentResult)
		}
		innerResult := inner.Close()
		laneResult := lane.Close()
		if physical, joined := lane.CleanupResult(); !joined || physical != nil {
			if !joined || !unknownJoin {
				t.Fatal("original forwarded lane physical join", joined, physical)
			}
			t.Log("retained forwarded lane cleanup failure after crash", physical)
		}
		if innerResult != nil || laneResult != nil {
			t.Log("interrupted forwarded holder outcome", innerResult, laneResult)
		}
		lane.Finish()
		peerProcess.stop(t)
		if !bytes.Equal(read(peerSpendPath), previousPeerSpend) {
			t.Fatal("joined Interior changed or refunded spends")
		}
		if err := conn.Close(); err != nil {
			t.Fatal("original client physical Close", err)
		}
		if unknownJoin {
			if _, err := os.Lstat(filepath.Join(installation, "forwarding-retirement-00000000/retirement-result.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("native crash fabricated completed domain retirement", err)
			}
			refuse("restart", distribution, history, "alpha", "coherent-private-forwarding", bindingPath)
			for _, path := range []string{filepath.Join(installation, "forwarding-invocation-00000001"), installation + ".forwarding-invocation-00000001-journal"} {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("unknown domain join admitted a successor attempt", path, err)
				}
			}
			_, reserved := forwardingInstalledBudget(t, plan.HostingRoot)
			wantRetained := plan.Work.Tx + plan.Work.Rx + plan.Termination.Tx + plan.Termination.Rx + 988
			if reserved != wantRetained || reserved == 0 || !bytes.Equal(read(spendPath), previousSpend) || !bytes.Equal(read(peerSpendPath), previousPeerSpend) || !bytes.Equal(read(startJournal), startBefore) {
				t.Fatal("unknown join reset original capacity, spends or startup history", reserved, wantRetained)
			}
			pin.close(t)
			reference.close(t)
			return
		}
		manager = forwardingInstalledManager(t)
		if manager["ActiveState"] != "inactive" || manager["MainPID"] != "0" || manager["ExecMainCode"] != "1" || manager["ExecMainStatus"] != "0" {
			t.Fatal("original fixed unit lacks matching clean exit", manager)
		}
		command("{\"outcome\":\"forwarding-start-history-verified\",\"runtime_authority\":false,\"retirement_proved\":false}\n", "inspect-start-history", history, bindingPath)
		retirementDirectory := filepath.Join(installation, fmt.Sprintf("forwarding-retirement-%08d", invocation))
		retirementJournal := installation + fmt.Sprintf(".forwarding-retirement-%08d-journal", invocation)
		retirementPath := filepath.Join(retirementDirectory, "retirement-result.json")
		retirementRaw := read(retirementPath)
		var retired struct {
			Schema           string `json:"schema"`
			Attempt          uint32 `json:"attempt"`
			Generation       string `json:"generation"`
			BindingSHA256    string `json:"binding_sha256"`
			StartSHA256      string `json:"start_sha256"`
			PID              uint32 `json:"pid"`
			InvocationID     string `json:"invocation_id"`
			Manager          string `json:"manager"`
			Challenge        string `json:"challenge"`
			CompletionSHA256 string `json:"completion_sha256"`
			StartedRealtime  uint64 `json:"started_realtime"`
			StartedMonotonic uint64 `json:"started_monotonic"`
			ExitedRealtime   uint64 `json:"exited_realtime"`
			ExitedMonotonic  uint64 `json:"exited_monotonic"`
			Code, Status     int32
		}
		archiveHash := sha256.Sum256(archiveRaw)
		if json.Unmarshal(retirementRaw, &retired) != nil || retired.Schema != "ardents-forwarding-retirement-v1" || retired.Attempt != uint32(invocation) || retired.Generation != binding.Generation || retired.BindingSHA256 != archive.BindingSHA256 || retired.StartSHA256 != hex.EncodeToString(archiveHash[:]) || retired.PID != archive.PID || retired.InvocationID != archive.InvocationID || retired.Manager != archive.Manager || retired.StartedRealtime != archive.StartedRealtime || retired.StartedMonotonic != archive.StartedMonotonic || retired.ExitedRealtime < retired.StartedRealtime || retired.ExitedMonotonic < retired.StartedMonotonic || retired.Code != 1 || retired.Status != 0 || len(retired.Challenge) != 64 || strings.Trim(retired.Challenge, "0123456789abcdef") != "" || retired.Challenge == strings.Repeat("0", 64) || retired.Challenge == previousChallenge || len(retired.CompletionSHA256) != 64 {
			t.Fatal("retirement bytes differ from the original joined invocation")
		}
		previousChallenge = retired.Challenge
		forwardingInstalledJournal(t, retirementJournal, invocation, true)
		for _, pending := range []string{filepath.Join(installation, startDirectory, "forwarding-start-pending"), filepath.Join(retirementDirectory, "retirement-pending")} {
			if _, err := os.Lstat(pending); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("original completion remains unpublished", pending, err)
			}
		}
		for path, raw := range historyFiles {
			if !bytes.Equal(raw, read(path)) {
				t.Fatal("later invocation changed original committed history", path)
			}
		}
		for _, path := range []string{startJournal, archivePath, retirementJournal, retirementPath} {
			historyFiles[path] = read(path)
		}
		if !bytes.Equal(read(spendPath), previousSpend) {
			t.Fatal("joined stop changed or refunded spends")
		}
		used, reserved := forwardingInstalledBudget(t, plan.HostingRoot)
		if reserved != 0 || used < previousUsed {
			t.Fatal("original physical reservation not returned", reserved, used, previousUsed)
		}
		previousUsed = used
		for path, original := range originals {
			info, err := os.Lstat(path)
			if err != nil || !os.SameFile(original, info) || original.Mode() != info.Mode() {
				t.Fatal("retained root/credential/account identity changed", path, err)
			}
			oldNative, newNative := original.Sys().(*syscall.Stat_t), info.Sys().(*syscall.Stat_t)
			if oldNative.Uid != newNative.Uid || oldNative.Gid != newNative.Gid || !original.IsDir() && (oldNative.Mtim != newNative.Mtim || oldNative.Ctim != newNative.Ctim || original.Size() != info.Size()) {
				t.Fatal("original protected input/account access or mutation times changed", path)
			}
		}
		for path, original := range fixed {
			if !bytes.Equal(original, read(path)) {
				t.Fatal("fixed original input/account bytes changed", path)
			}
		}
		reference.close(t)
		pin.close(t)
	}
	if selected == "retirement-history-missing" || selected == "retirement-history-damaged" {
		path := filepath.Join(installation, "forwarding-retirement-00000001/retirement-result.json")
		original, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		// End the fresh case in explicit refusal. No original is restored,
		// adopted or used to retry installation after this controlled fault.
		if selected == "retirement-history-missing" {
			if err := os.Rename(path, path+".withheld"); err != nil {
				t.Fatal(err)
			}
		} else {
			fd, err := unix.Open(path, unix.O_WRONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
			if err != nil {
				t.Fatal(err)
			}
			file := os.NewFile(uintptr(fd), "original-retirement-history")
			_, writeErr := file.WriteAt([]byte{'!'}, 0)
			if err := errors.Join(writeErr, file.Sync(), file.Close()); err != nil {
				t.Fatal(err)
			}
		}
		refuse("restart", distribution, history, "alpha", "coherent-private-forwarding", bindingPath)
		for _, absent := range []string{filepath.Join(installation, "forwarding-invocation-00000002"), installation + ".forwarding-invocation-00000002-journal"} {
			if _, err := os.Lstat(absent); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("damaged retirement admitted another original invocation", absent, err)
			}
		}
		manager := forwardingInstalledManager(t)
		_, reserved := forwardingInstalledBudget(t, plan.HostingRoot)
		if manager["MainPID"] != "0" || reserved != 0 || !bytes.Equal(read(filepath.Join(plan.SpendRoot, "closed-token-spends")), previousSpend) || !bytes.Equal(read(filepath.Join(peerPlan.SpendRoot, "closed-token-spends")), previousPeerSpend) {
			t.Fatal("retirement history refusal started effects or changed spent rights")
		}
		if err := original.Close(); err != nil {
			t.Fatal("original withheld/damaged history descriptor join", err)
		}
	}
}

type forwardingInstalledChannels struct {
	Schema          string    `json:"schema"`
	NetworkID       [32]byte  `json:"network_id"`
	StateGeneration [32]byte  `json:"state_generation"`
	StateDigest     [32]byte  `json:"state_digest"`
	ProfileDigest   [32]byte  `json:"profile_digest"`
	NodeID          [32]byte  `json:"node_id"`
	DutyGeneration  uint64    `json:"duty_generation"`
	Endpoint        string    `json:"endpoint"`
	PublicKey       [32]byte  `json:"public_key"`
	Deadline        time.Time `json:"deadline"`
	Channels        [2]struct {
		Nonce  [32]byte  `json:"nonce"`
		Tokens [2][]byte `json:"tokens"`
	} `json:"channels"`
}

func forwardingInstalledPrivateStand(t *testing.T) {
	t.Helper()
	marker, err := os.ReadFile("/etc/forwarding-initial-component-marker")
	if err != nil || string(marker) != "ardents-forwarding-coherent-preparation-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid environment: original private Forwarding stand required", err)
	}
	var pins struct {
		PrivateRoot     [2]uint64            `json:"private_root"`
		OuterNamespaces map[string][2]uint64 `json:"outer_namespaces"`
	}
	raw, err := os.ReadFile("/artifacts/pins.json")
	if err != nil || json.Unmarshal(raw, &pins) != nil || len(pins.OuterNamespaces) != 6 {
		t.Fatal("invalid environment: independently pinned original namespaces required", err)
	}
	info, err := os.Stat("/")
	if err != nil {
		t.Fatal(err)
	}
	native := info.Sys().(*syscall.Stat_t)
	if pins.PrivateRoot != [2]uint64{uint64(native.Dev), native.Ino} {
		t.Fatal("private root substituted")
	}
	for kind, outer := range pins.OuterNamespaces {
		info, err := os.Stat("/proc/self/ns/" + kind)
		if err != nil {
			t.Fatal(err)
		}
		native := info.Sys().(*syscall.Stat_t)
		if outer == [2]uint64{uint64(native.Dev), native.Ino} {
			t.Fatal("host namespace retained", kind)
		}
	}
	forwardingInstalledArtifact(t, os.Args[0], "forwarding-installed.test")
}

func forwardingInstalledArtifact(t *testing.T, path, name string) {
	t.Helper()
	var pins struct {
		Artifacts map[string]string `json:"artifacts"`
	}
	raw, err := os.ReadFile("/artifacts/pins.json")
	if err != nil || json.Unmarshal(raw, &pins) != nil {
		t.Fatal("original artifact pins missing", err)
	}
	raw, err = os.ReadFile(path)
	sum := sha256.Sum256(raw)
	if err != nil || pins.Artifacts[name] != hex.EncodeToString(sum[:]) {
		t.Fatal("independently pinned actual artifact differs", name, err)
	}
}

func forwardingInstalledBootstrapInputs(t *testing.T, inputs map[string][]byte) {
	t.Helper()
	raw, err := os.ReadFile("/artifacts/forwarding-bootstrap-receipt.json")
	if err != nil {
		t.Fatal("invalid environment: new exact completed bootstrap receipt absent", err)
	}
	sum := sha256.Sum256(raw)
	if os.Getenv("ARDENTS_FORWARDING_INSTALLED_BOOTSTRAP_SHA256") != hex.EncodeToString(sum[:]) {
		t.Fatal("invalid environment: outside independently reviewed bootstrap pin differs")
	}
	var receipt struct {
		Schema         string                       `json:"schema"`
		CaseInputs     map[string]map[string]string `json:"case_inputs"`
		SourceManifest string                       `json:"source_manifest_sha256"`
		Artifacts      map[string]string            `json:"artifacts"`
		Cases          []string                     `json:"cases"`
	}
	if json.Unmarshal(raw, &receipt) != nil || receipt.Schema != "ardents-forwarding-bootstrap-receipt-v1" {
		t.Fatal("invalid environment: Forwarding bootstrap scope differs")
	}
	var pins struct {
		SourceManifest string            `json:"source_manifest"`
		Artifacts      map[string]string `json:"artifacts"`
	}
	pinnedRaw, err := os.ReadFile("/artifacts/pins.json")
	wantCases := []string{"lifetime-refill", "retirement-history-missing", "retirement-history-damaged", "unknown-domain-join"}
	if err != nil || json.Unmarshal(pinnedRaw, &pins) != nil || receipt.SourceManifest != pins.SourceManifest || len(receipt.Artifacts) != len(pins.Artifacts) || strings.Join(receipt.Cases, "\x00") != strings.Join(wantCases, "\x00") || len(receipt.CaseInputs) != len(wantCases) {
		t.Fatal("actual source/artifact/case scope differs from reviewed receipt", err)
	}
	for _, name := range wantCases {
		if len(receipt.CaseInputs[name]) != 8 {
			t.Fatal("incomplete case-specific bootstrap inputs", name)
		}
		for _, input := range []string{"binding", "plan", "bundle", "profile", "presentations", "peer_plan", "peer_presentations", "peer_certificate"} {
			sum := receipt.CaseInputs[name][input]
			if len(sum) != 64 || strings.Trim(sum, "0123456789abcdef") != "" {
				t.Fatal("case-specific bootstrap input pin differs", name, input)
			}
		}
	}
	caseInputs, exists := receipt.CaseInputs[os.Getenv("ARDENTS_FORWARDING_INSTALLED_CASE")]
	if !exists {
		t.Fatal("original case absent from reviewed bootstrap scope")
	}
	for name, sum := range pins.Artifacts {
		if receipt.Artifacts[name] != sum {
			t.Fatal("actual artifact differs from reviewed receipt", name)
		}
	}
	for name, raw := range inputs {
		sum := sha256.Sum256(raw)
		if caseInputs[name] != hex.EncodeToString(sum[:]) {
			t.Fatal("actual prepared input differs from reviewed receipt", name)
		}
	}
}

func forwardingInstalledManager(t *testing.T) map[string]string {
	t.Helper()
	raw, err := exec.Command("systemctl", "show", "ardents-forwarding.service", "--property=ActiveState,MainPID,InvocationID,ControlGroup,ExecMainCode,ExecMainStatus").CombinedOutput()
	if err != nil {
		t.Fatal("original manager observation", err, string(raw))
	}
	result := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			t.Fatal("manager observation malformed")
		}
		result[key] = value
	}
	if len(result) != 6 {
		t.Fatal("manager observation incomplete")
	}
	return result
}

// Independent native oracle: the archive must retain the actual broker and
// unique manager, not merely a nonempty owner name that Stop cannot match.
func forwardingInstalledManagerIdentity(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	query := func(method string, args ...string) string {
		t.Helper()
		command := append([]string{"--system", "--json=short", "call", "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", method}, args...)
		raw, err := exec.CommandContext(ctx, "busctl", command...).CombinedOutput()
		var reply struct {
			Type string   `json:"type"`
			Data []string `json:"data"`
		}
		if err != nil || json.Unmarshal(raw, &reply) != nil || reply.Type != "s" || len(reply.Data) != 1 {
			t.Fatal("independent native manager identity unavailable", err)
		}
		return reply.Data[0]
	}
	var result string
	for range 2 {
		broker, owner := strings.ToLower(query("GetId")), query("GetNameOwner", "s", "org.freedesktop.systemd1")
		if len(broker) != 32 || strings.Trim(broker, "0123456789abcdef") != "" || !strings.HasPrefix(owner, ":") || len(owner) > 255 {
			t.Fatal("independent native manager identity malformed")
		}
		current := broker + "/" + owner
		if result != "" && result != current {
			t.Fatal("original native manager identity changed")
		}
		result = current
	}
	return result
}

func forwardingInstalledWaitListening(t *testing.T, invocation string, pid int) {
	t.Helper()
	until := time.Now().Add(15 * time.Second)
	for {
		raw, err := exec.Command("journalctl", "--no-pager", "--output=cat", "_SYSTEMD_INVOCATION_ID="+invocation, "_PID="+strconv.Itoa(pid)).CombinedOutput()
		if err != nil {
			t.Fatal("original invocation journal unavailable", err, string(raw))
		}
		if bytes.Contains(raw, []byte("{\"operation\":\"forwarding.start-installed\",\"phase\":\"listening\"}\n")) {
			return
		}
		observed := forwardingInstalledManager(t)
		if observed["InvocationID"] != invocation || observed["MainPID"] != strconv.Itoa(pid) || observed["ActiveState"] != "active" || !time.Now().Before(until) {
			t.Fatal("same original installed invocation did not reach listening", string(raw), observed)
		}
		// This is bounded observation of one original start, never a Start or
		// connection retry. Actual TLS authentication still follows readiness.
		time.Sleep(20 * time.Millisecond)
	}
}

// This test observation counts only actual returned ciphertext bytes. It never
// supplies admission, a second reader or a substitute physical-retirement owner.
type forwardingInstalledTLSObservation struct {
	net.Conn
	received, sent atomic.Uint64
}

func (conn *forwardingInstalledTLSObservation) Read(raw []byte) (int, error) {
	n, err := conn.Conn.Read(raw)
	if n > 0 {
		conn.received.Add(uint64(n))
	}
	return n, err
}

func (conn *forwardingInstalledTLSObservation) Write(raw []byte) (int, error) {
	n, err := conn.Conn.Write(raw)
	if n > 0 {
		conn.sent.Add(uint64(n))
	}
	return n, err
}

// Preserve traversal to the original lane's joined physical-retirement witness.
func (conn *forwardingInstalledTLSObservation) NetConn() net.Conn { return conn.Conn }

// This independent native reference keeps systemd's original execution facts
// observable through the strict exit oracle. It is released and physically
// joined before the next invocation, and supplies no installed admission.
type forwardingInstalledUnitReference struct {
	command           *exec.Cmd
	input             io.WriteCloser
	cancel            context.CancelFunc
	started, done     chan struct{}
	startErr, waitErr error
	once              sync.Once
	result            error
}

type forwardingUnitReferenceOutput struct {
	ready chan struct{}
	line  bytes.Buffer
	once  sync.Once
}

func (output *forwardingUnitReferenceOutput) Write(raw []byte) (int, error) {
	for position, value := range raw {
		if output.line.Len() >= 1024 {
			return position, errors.New("fixed-unit reference output exceeded its bound")
		}
		if value == '\n' {
			if output.line.String() == "forwarding-original-unit-referenced" {
				output.once.Do(func() { close(output.ready) })
			}
			output.line.Reset()
		} else {
			_ = output.line.WriteByte(value)
		}
	}
	return len(raw), nil
}

func forwardingInstalledRetainUnit(t *testing.T) *forwardingInstalledUnitReference {
	t.Helper()
	const program = "/artifacts/forwarding-manager.test"
	forwardingInstalledArtifact(t, program, "forwarding-manager.test")
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 4*time.Minute)
	reference := &forwardingInstalledUnitReference{cancel: cancel, started: make(chan struct{}), done: make(chan struct{})}
	reference.command = exec.CommandContext(ctx, program, "-test.run=^TestForwardingLoadedNativeUnitReference$", "-test.count=1", "-test.timeout=4m")
	reference.command.Env = append(os.Environ(), "ARDENTS_FORWARDING_UNIT_REFERENCE=original")
	reference.command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	output := &forwardingUnitReferenceOutput{ready: make(chan struct{})}
	reference.command.Stdout, reference.command.Stderr = output, io.Discard
	var err error
	reference.input, err = reference.command.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal("original unit reference input unavailable", err)
	}
	t.Cleanup(func() { reference.close(t) })
	// Linux parent-death custody belongs to this original creating OS thread;
	// retain it through Start, the whole held reference and actual child Wait.
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		reference.startErr = reference.command.Start()
		close(reference.started)
		if reference.startErr == nil {
			reference.waitErr = reference.command.Wait()
		} else {
			reference.waitErr = reference.startErr
		}
		close(reference.done)
	}()
	<-reference.started
	if reference.startErr != nil {
		cancel()
		t.Fatal("original unit reference launch failed", reference.startErr)
	}
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-output.ready:
		return reference
	case <-reference.done:
		cancel()
		t.Fatal("original unit reference ended before readiness")
	case <-timer.C:
		cancel()
		reference.close(t)
		t.Fatal("original unit reference readiness unavailable")
	}
	return nil
}

func (reference *forwardingInstalledUnitReference) close(t *testing.T) {
	t.Helper()
	reference.once.Do(func() {
		<-reference.started
		var releaseErr error
		if reference.startErr == nil {
			_, releaseErr = reference.input.Write([]byte{'r'})
		}
		inputErr := reference.input.Close()
		<-reference.done
		reference.result = errors.Join(releaseErr, inputErr, reference.waitErr)
		reference.cancel()
	})
	if reference.result != nil {
		t.Fatal("original unit reference physical join failed", reference.result)
	}
}

type forwardingInstalledProcessPin struct {
	pidfd       int
	events      *os.File
	info        os.FileInfo
	process     *os.File
	processInfo os.FileInfo
	group       *os.File
	groupInfo   os.FileInfo
}

func forwardingInstalledPinProcess(t *testing.T, pid int, group string, uid uint32) *forwardingInstalledProcessPin {
	t.Helper()
	if group != "/system.slice/ardents-forwarding.service" {
		t.Fatal("fixed original cgroup differs", group)
	}
	status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	want := fmt.Sprintf("%d\t%d\t%d\t%d", uid, uid, uid, uid)
	if err != nil || !bytes.Contains(status, []byte("Uid:\t"+want+"\n")) || !bytes.Contains(status, []byte("Gid:\t"+want+"\n")) {
		t.Fatal("actual non-root Node principal differs", err)
	}
	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		t.Fatal("original pidfd unavailable", err)
	}
	pin := &forwardingInstalledProcessPin{pidfd: fd}
	t.Cleanup(func() { pin.close(t) })
	pin.process, err = os.Open(fmt.Sprintf("/proc/%d", pid))
	if err != nil {
		t.Fatal(err)
	}
	pin.processInfo, err = pin.process.Stat()
	if err != nil {
		t.Fatal(err)
	}
	pin.group, err = os.Open("/sys/fs/cgroup" + group)
	if err != nil {
		t.Fatal(err)
	}
	pin.groupInfo, err = pin.group.Stat()
	if err != nil {
		t.Fatal(err)
	}
	pin.events, err = os.Open("/sys/fs/cgroup" + group + "/cgroup.events")
	if err != nil {
		t.Fatal(err)
	}
	pin.info, err = pin.events.Stat()
	var fs syscall.Statfs_t
	if err != nil || syscall.Fstatfs(int(pin.events.Fd()), &fs) != nil || fs.Type != 0x63677270 {
		t.Fatal("original native cgroup-v2 events unavailable", err)
	}
	if pin.readJoined(t) {
		t.Fatal("original live Node scope already joined")
	}
	return pin
}

func (pin *forwardingInstalledProcessPin) readJoined(t *testing.T) bool {
	t.Helper()
	info, err := pin.events.Stat()
	if err != nil || !os.SameFile(pin.info, info) {
		t.Fatal("original kernel events descriptor changed", err)
	}
	if _, err := pin.events.Seek(0, io.SeekStart); err != nil {
		if errors.Is(err, syscall.ENODEV) {
			return true
		}
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(pin.events, 1025))
	if len(raw) == 0 && errors.Is(err, syscall.ENODEV) {
		return true // Only this retained actual cgroup-v2 core descriptor.
	}
	if err != nil || len(raw) > 1024 || !bytes.Contains(raw, []byte("populated ")) {
		t.Fatal("original kernel join observation unavailable", err)
	}
	return bytes.Contains(raw, []byte("populated 0\n"))
}

func (pin *forwardingInstalledProcessPin) joined(t *testing.T) {
	t.Helper()
	fds := []unix.PollFd{{Fd: int32(pin.pidfd), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, 0)
	if err != nil || n != 1 || fds[0].Revents&unix.POLLIN == 0 || !pin.readJoined(t) {
		t.Fatal("original pidfd/kernel scope not independently joined", err)
	}
}

func (pin *forwardingInstalledProcessPin) waitAfterCrash(t *testing.T) {
	t.Helper()
	until := time.Now().Add(5 * time.Second)
	fds := []unix.PollFd{{Fd: int32(pin.pidfd), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, 5000)
	if err != nil || n != 1 || fds[0].Revents&unix.POLLIN == 0 {
		t.Fatal("original crash process did not join within observation bound", err)
	}
	for !pin.readJoined(t) {
		if t.Context().Err() != nil || !time.Now().Before(until) {
			t.Fatal("original crash scope did not join within observation bound")
		}
		// Observe only the retained kernel descriptor. A signal, timeout or
		// missing pathname cannot stand in for the original physical join.
		time.Sleep(10 * time.Millisecond)
	}
}

func (pin *forwardingInstalledProcessPin) close(t *testing.T) {
	t.Helper()
	if pin.events != nil {
		if err := pin.events.Close(); err != nil {
			t.Error(err)
		}
		pin.events = nil
	}
	if pin.pidfd >= 0 {
		if err := unix.Close(pin.pidfd); err != nil {
			t.Error(err)
		}
		pin.pidfd = -1
	}
	for _, slot := range []**os.File{&pin.process, &pin.group} {
		if *slot != nil {
			if err := (*slot).Close(); err != nil {
				t.Error(err)
			}
			*slot = nil
		}
	}
}

func forwardingInstalledJournal(t *testing.T, path string, invocation int, retirement bool) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > 128<<10 || raw[len(raw)-1] != '\n' {
		t.Fatal("complete bounded original journal unavailable", err)
	}
	first := "first-start"
	if invocation != 0 {
		first = "invocation"
	}
	schema := "ardents-forwarding-start-journal-v1"
	phases := []string{first, "directory-intent", "directory-born", "directory-ready", "file-intent", "file-born", "file-ready", "file-intent", "file-born", "file-ready", "socket-intent", "socket-born", "socket-ready", "file-intent", "file-born", "file-ready", "start-attempt", "invocation-observed", "completion-received", "file-intent", "file-born", "file-ready", "archived", "ack-attempt", "ack-written"}
	if retirement {
		schema = "ardents-forwarding-retirement-journal-v1"
		phases = []string{"stop", "directory-intent", "directory-born", "directory-ready", "file-intent", "file-born", "file-ready", "socket-intent", "socket-born", "socket-ready", "interrupt-attempt", "domain-joined", "native-joined", "native-closed", "file-intent", "file-born", "file-ready", "retired"}
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != len(phases) {
		t.Fatal("original journal count differs", len(lines), len(phases))
	}
	for index, line := range lines {
		var entry struct {
			Schema   string `json:"schema"`
			Sequence uint32 `json:"sequence"`
			Phase    string `json:"phase"`
			Path     string `json:"path"`
			Device   uint64 `json:"device"`
			Inode    uint64 `json:"inode"`
			SHA256   string `json:"sha256"`
			Mode     uint32 `json:"mode"`
			UID      uint32 `json:"uid"`
			GID      uint32 `json:"gid"`
		}
		if json.Unmarshal(line, &entry) != nil || entry.Schema != schema || entry.Sequence != uint32(index) || entry.Phase != phases[index] || entry.UID != 0 {
			t.Fatal("original transition ordering differs", index)
		}
		canonical, err := json.Marshal(entry)
		if err != nil || !bytes.Equal(canonical, line) {
			t.Fatal("journal canonical bytes differ", index, err)
		}
		if entry.Device == 0 && entry.Inode == 0 {
			continue
		}
		name := filepath.Base(entry.Path)
		if name == "forwarding-start-pending" || name == "retirement-pending" {
			continue // Historical births; successful publication unlinks them.
		}
		info, err := os.Lstat(entry.Path)
		if err != nil {
			t.Fatal("original recorded native resource missing", index, err)
		}
		native := info.Sys().(*syscall.Stat_t)
		if entry.Device != uint64(native.Dev) || entry.Inode != native.Ino || native.Uid != 0 {
			t.Fatal("original journal resource identity differs", index)
		}
		if strings.HasSuffix(entry.Phase, "-ready") || entry.Phase == first || entry.Phase == "stop" || entry.Phase == "archived" || entry.Phase == "retired" {
			if entry.Mode != uint32(info.Mode().Perm()) || entry.GID != native.Gid {
				t.Fatal("original recorded access differs", index)
			}
		}
	}
}

func forwardingInstalledBudget(t *testing.T, root string) (uint64, uint64) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "budget.json"))
	var envelope struct {
		State  json.RawMessage `json:"state"`
		SHA256 string          `json:"sha256"`
	}
	if err != nil || json.Unmarshal(raw, &envelope) != nil {
		t.Fatal("retained provider budget unavailable", err)
	}
	sum := sha256.Sum256(envelope.State)
	var budget struct {
		Schema   string `json:"schema"`
		Used     uint64 `json:"used"`
		Reserved uint64 `json:"reserved"`
	}
	if envelope.SHA256 != hex.EncodeToString(sum[:]) || json.Unmarshal(envelope.State, &budget) != nil || budget.Schema != "ardents-hosting-budget-v1" {
		t.Fatal("independent provider accounting bytes invalid")
	}
	return budget.Used, budget.Reserved
}

func forwardingInstalledSpend(t *testing.T, raw []byte, input forwardingInstalledChannels, count int) {
	t.Helper()
	want := append([]byte("ARDSPN01"), input.NetworkID[:]...)
	want = append(want, input.ProfileDigest[:]...)
	want = append(want, input.NodeID[:]...)
	want = binary.BigEndian.AppendUint64(want, input.DutyGeneration)
	if len(raw) != 112+count*41 || !bytes.Equal(raw[:min(len(raw), 112)], want) {
		t.Fatal("original current-hour spend inventory differs", len(raw), count)
	}
	for number := range count {
		token := input.Channels[number/2].Tokens[number%2]
		sum := sha256.Sum256(token)
		record := raw[112+number*41 : 112+(number+1)*41]
		window := int64(binary.BigEndian.Uint64(record[32:40]))
		if !bytes.Equal(record[:32], sum[:]) || record[40] != 1 || window%3600 != 0 || !time.Now().Before(time.Unix(window+3600, 0)) {
			t.Fatal("independent original token spend differs", number)
		}
	}
}

// The remote leg is an actual Receiver component under the same Node principal,
// with its own State and receiving history and the original shared host budget.
// It is not another installed Node or a replacement for its Guard/producer.
// The root driver transfers only the two original read-only credential FDs;
// it neither reads private bytes nor transfers a live State/budget/root owner.
type forwardingInstalledInterior struct {
	command           *exec.Cmd
	started           chan struct{}
	wait              chan struct{}
	done              chan struct{}
	startErr, waitErr error
	input             io.WriteCloser
	output            *bufio.Scanner
	diagnostic        bytes.Buffer
	pidfd             int
	stopped           bool
	allowRouteFailure bool
}

func forwardingInstalledStartInterior(t *testing.T, base string, uid uint32, input forwardingInstalledChannels, plan forwarding.Plan, retained bool) *forwardingInstalledInterior {
	t.Helper()
	if plan.SpendRoot != filepath.Join(base, "component-peer/spend") || uid < 100 || uid > 999 {
		t.Fatal("exact remote component root/principal required")
	}
	if !retained {
		if err := os.Mkdir(plan.SpendRoot, 0700); err != nil {
			t.Fatal("fresh Interior spend root", err)
		}
		if err := os.Chown(plan.SpendRoot, int(uid), int(uid)); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Lstat(plan.SpendRoot)
	if err != nil || info.Mode() != os.ModeDir|0700 || info.Sys().(*syscall.Stat_t).Uid != uid || info.Sys().(*syscall.Stat_t).Gid != uid {
		t.Fatal("Interior receiving root is foreign", err)
	}
	open := func(path string) *os.File {
		t.Helper()
		fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if err != nil {
			t.Fatal("original component credential descriptor", err)
		}
		file := os.NewFile(uintptr(fd), path)
		info, err := file.Stat()
		if err != nil || info.Mode() != 0600 || !info.Mode().IsRegular() || info.Sys().(*syscall.Stat_t).Uid != 0 || info.Sys().(*syscall.Stat_t).Gid != 0 || info.Sys().(*syscall.Stat_t).Nlink != 1 {
			_ = file.Close()
			t.Fatal("original component credential custody differs", err)
		}
		return file
	}
	certificate := open(filepath.Join(base, "delivery/peer-certificate/node-certificate.pem"))
	defer certificate.Close()
	key := open(filepath.Join(base, "delivery/canonical-preparation", fmt.Sprintf("node-key-%02x", input.NodeID[0]), "node-key.pem"))
	defer key.Close()
	process := &forwardingInstalledInterior{pidfd: -1, allowRouteFailure: os.Getenv("ARDENTS_FORWARDING_INSTALLED_CASE") == "unknown-domain-join",
		started: make(chan struct{}), wait: make(chan struct{}), done: make(chan struct{})}
	process.command = exec.Command(os.Args[0], "-test.run=^TestForwardingInstalledNativeInteriorPeer$", "-test.count=1", "-test.timeout=3m")
	process.command.Env = append(os.Environ(), "ARDENTS_FORWARDING_INTERIOR_COMPONENT=original-readonly-credentials", "ARDENTS_FORWARDING_INTERIOR_RETAINED=0")
	if retained {
		process.command.Env = append(process.command.Env, "ARDENTS_FORWARDING_INTERIOR_RETAINED=1")
	}
	process.command.ExtraFiles = []*os.File{certificate, key}
	process.command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uid, Gid: uid, Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
	process.command.Stderr = &process.diagnostic
	process.input, err = process.command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := process.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	process.output = bufio.NewScanner(output)
	// Pdeathsig is tied to the creating Linux thread. Keep that exact thread
	// through actual Wait, after the caller has drained its original stdout.
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		process.startErr = process.command.Start()
		close(process.started)
		if process.startErr == nil {
			<-process.wait
			process.waitErr = process.command.Wait()
		} else {
			process.waitErr = process.startErr
		}
		close(process.done)
	}()
	<-process.started
	if process.startErr != nil {
		closed := errors.Join(process.input.Close(), output.Close())
		<-process.done
		t.Fatal("original Interior launch", errors.Join(process.startErr, closed))
	}
	t.Cleanup(func() {
		if !process.stopped {
			process.stop(t)
		}
	})
	process.pidfd, err = unix.PidfdOpen(process.command.Process.Pid, 0)
	if err != nil {
		t.Fatal("original Interior pidfd", err)
	}
	if !process.output.Scan() || process.output.Text() != "forwarding-interior-component-listening" {
		t.Fatal("genuine Interior listener unavailable", process.output.Text(), process.output.Err())
	}
	return process
}

func (process *forwardingInstalledInterior) stop(t *testing.T) {
	t.Helper()
	if process.stopped {
		t.Fatal("original Interior was already joined")
	}
	_, writeErr := process.input.Write([]byte{'s'})
	closeErr := process.input.Close()
	joined := false
	routeFailed := false
	var unexpected []string
	for process.output.Scan() {
		switch process.output.Text() {
		case "forwarding-interior-component-joined":
			if joined {
				unexpected = append(unexpected, "duplicate joined report")
			}
			joined = true
		case "forwarding-interior-component-joined-with-route-failure":
			if joined || !process.allowRouteFailure {
				unexpected = append(unexpected, "unexpected Route failure completion")
			}
			joined, routeFailed = true, true
		case "PASS":
		default:
			unexpected = append(unexpected, process.output.Text())
		}
	}
	close(process.wait)
	<-process.done
	waitErr := process.waitErr
	poll := []unix.PollFd{{Fd: int32(process.pidfd), Events: unix.POLLIN}}
	_, pollErr := unix.Poll(poll, 0)
	fdErr := unix.Close(process.pidfd)
	process.stopped = true
	if err := errors.Join(writeErr, closeErr, process.output.Err(), waitErr, pollErr, fdErr); err != nil || !joined || poll[0].Revents&unix.POLLIN == 0 || len(unexpected) != 0 || (process.diagnostic.Len() != 0) != routeFailed {
		t.Fatal("original Interior domain/native completion unavailable", err, joined, unexpected, process.diagnostic.String())
	}
	if routeFailed {
		t.Log("retained remote Route cleanup failure after original crash", process.diagnostic.String())
	}
}

func TestForwardingInstalledNativeInteriorPeer(t *testing.T) {
	const base = "/var/lib/forwarding-coherent-initial"
	if os.Getenv("ARDENTS_FORWARDING_INTERIOR_COMPONENT") != "original-readonly-credentials" || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getuid() != os.Geteuid() || os.Getgid() != os.Geteuid() || os.Getegid() != os.Geteuid() {
		t.Fatal("invalid environment: original non-root Interior component required")
	}
	groups, err := os.Getgroups()
	if err != nil || len(groups) != 0 {
		t.Fatal("Interior supplementary groups differ", err)
	}
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	planRaw := read(filepath.Join(base, "component-peer/forwarding-plan.json"))
	plan, err := forwarding.DecodePlan(planRaw)
	if err != nil {
		t.Fatal(err)
	}
	var binding struct{ UID, GID uint32 }
	if json.Unmarshal(read(filepath.Join(base, "runtime/node/binding.json")), &binding) != nil || int(binding.UID) != os.Geteuid() || binding.GID != binding.UID {
		t.Fatal("Interior principal differs from original Node binding")
	}
	forwardingInstalledArtifact(t, os.Args[0], "forwarding-installed.test")
	forwardingInstalledBootstrapInputs(t, map[string][]byte{"peer_plan": planRaw})
	credential := func(fd uintptr, private bool) (*os.File, []byte) {
		t.Helper()
		file := os.NewFile(fd, "original-interior-credential")
		if file == nil {
			t.Fatal("original credential descriptor absent")
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("original credential descriptor differs", err)
		}
		native := info.Sys().(*syscall.Stat_t)
		if native.Uid != 0 || native.Gid != 0 || native.Nlink != 1 || info.Mode().Perm() != 0600 || info.Size() <= 0 || info.Size() > 64<<10 {
			t.Fatal("original root credential custody differs")
		}
		flags, err := unix.FcntlInt(file.Fd(), unix.F_GETFL, 0)
		if err != nil || flags&unix.O_ACCMODE != unix.O_RDONLY {
			t.Fatal("credential descriptor is writable", err)
		}
		raw, err := io.ReadAll(io.LimitReader(file, 64<<10+1))
		if err != nil || int64(len(raw)) != info.Size() {
			t.Fatal("original credential read failed", err)
		}
		if !private {
			forwardingInstalledBootstrapInputs(t, map[string][]byte{"peer_certificate": raw})
		}
		return file, raw
	}
	certificateFile, certificateRaw := credential(3, false)
	defer certificateFile.Close()
	keyFile, keyRaw := credential(4, true)
	defer keyFile.Close()
	defer func() { clear(certificateRaw); clear(keyRaw) }()
	certificate, err := tls.X509KeyPair(certificateRaw, keyRaw)
	clear(keyRaw)
	if err != nil {
		t.Fatal(err)
	}
	key, ok := certificate.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		t.Fatal("canonical Interior key kind differs")
	}
	defer clear(key)
	keys := make(map[[32]byte]ed25519.PublicKey)
	for _, public := range plan.Network.Authorities {
		keys[sha256.Sum256(public[:])] = append(ed25519.PublicKey(nil), public[:]...)
	}
	owner, err := state.OpenRetained(state.Config{Root: plan.Network.Root, NetworkID: plan.Network.NetworkID, Authorities: keys, Threshold: plan.Network.Threshold, ClosedProfileAuthority: plan.Network.ProfileAuthority[:], AcceptedProfile: "ardents-route-v3", Clock: time.Now, ClockObservationFile: plan.Network.ClockObservationFile})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member(plan.NodeID, view.ObservedAt())
	if err != nil || member.RoleDomain != 4 || member.Subrole != 2 || !bytes.Equal(key.Public().(ed25519.PublicKey), member.PublicKey[:]) {
		t.Fatal("actual Interior duty/key differs", err)
	}
	duty, err := view.RetainDuty(member.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	authority := role.Authority{Current: owner.CurrentRuntime, Duty: duty, Profile: view.Profile().ProfileBinding}
	p := view.Profile()
	recipient := receiving.Receiver{NetworkID: p.Network, StateGeneration: p.Generation, StateDigest: p.EpochDigest, ProfileDigest: p.Digest, NodeID: member.NodeID, DutyGeneration: member.DutyGeneration}
	observe := func() (receiving.Observation, error) {
		actual, err := owner.CurrentRuntime()
		if err != nil {
			return receiving.Observation{}, err
		}
		if _, err := authority.Member(); err != nil {
			return receiving.Observation{}, err
		}
		m, err := actual.Member(recipient.NodeID, actual.ObservedAt())
		if err != nil || m.DutyGeneration != recipient.DutyGeneration {
			return receiving.Observation{}, errors.Join(errors.New("Interior duty changed"), err)
		}
		profile := actual.Profile()
		facts := admission.AuthorityFacts{NetworkID: profile.Network, StateGeneration: profile.Generation, StateDigest: profile.EpochDigest, Digest: profile.Digest, IssuanceAuthorityKey: profile.IssuanceAuthorityKey, IssuerNodeID: profile.IssuerNodeID, IssuerDutyGeneration: profile.IssuerDutyGeneration, Epoch: profile.Epoch, NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, TokenKeyCount: uint8(len(profile.TokenKeys))}
		if len(profile.TokenKeys) > len(facts.TokenKeys) {
			return receiving.Observation{}, errors.New("Interior token inventory exceeds bound")
		}
		for index, public := range profile.TokenKeys {
			facts.TokenKeys[index] = admission.TokenKey{WindowStart: public.WindowStart, Class: public.Class, SPKI: public.SPKI}
		}
		end := m.NotAfter()
		if profile.NotAfter.Before(end) {
			end = profile.NotAfter
		}
		return receiving.Observation{Profile: facts, Receiver: recipient, Now: actual.ObservedAt(), NotAfter: end}, nil
	}
	openReceiving := receiving.Open
	if os.Getenv("ARDENTS_FORWARDING_INTERIOR_RETAINED") == "1" {
		openReceiving = receiving.OpenRetained
	}
	spend, err := openReceiving(plan.SpendRoot, recipient, observe)
	if err != nil {
		t.Fatal(err)
	}
	defer spend.Close()
	budget, err := hosting.Open(plan.HostingRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	var reservationMu sync.Mutex
	reservations := make(map[receiver.Channel]*hosting.Reservation)
	releaseReservation := func(held *hosting.Reservation) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return held.Release(ctx)
	}
	// This component admits one actual inner channel per invocation. The finite
	// work and its complete Forward allowance are reserved before durable spend.
	remote, err := receiver.Listen(t.Context(), receiver.ReceiverConfig{Authority: authority, Certificate: certificate,
		Admit: func(ctx context.Context, c receiver.Channel, token []byte) (receiving.Grant, error) {
			if c.Hello.Purpose != ardp.PurposeForwarding {
				return receiving.Grant{}, errors.New("Interior component purpose differs")
			}
			return spend.Accept(ctx, admission.ForwardClass, token, c.Hello.Deadline, func() (func() error, error) {
				held, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: plan.Work, Termination: plan.Termination, WorkUntil: c.Hello.Deadline, HoldUntil: c.Hello.Deadline.Add(5 * time.Second)})
				if err != nil {
					return nil, err
				}
				release := func() error { return releaseReservation(held) }
				limit := admission.ForwardClass.ByteLimit()
				if err := budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
					return nil, errors.Join(err, release())
				}
				reservationMu.Lock()
				reservations[c] = held
				reservationMu.Unlock()
				return c.HoldReservation(func() error {
					reservationMu.Lock()
					delete(reservations, c)
					reservationMu.Unlock()
					return release()
				})
			})
		},
		Refill: func(ctx context.Context, c receiver.Channel, prior receiving.Grant, remaining uint64, token []byte) (receiving.Grant, error) {
			limit := admission.ForwardClass.ByteLimit()
			if c.Hello.Purpose != ardp.PurposeForwarding || remaining == 0 || remaining >= limit {
				return receiving.Grant{}, errors.New("Interior refill scope differs")
			}
			reservationMu.Lock()
			held := reservations[c]
			reservationMu.Unlock()
			return spend.Refill(ctx, prior, remaining, token, func() (func() error, error) {
				if err := budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
					return nil, err
				}
				delta := limit - remaining
				additional, err := budget.ReserveAdditionalJoint(ctx, held, hosting.JointTraffic{Tx: delta, Rx: delta, Total: delta})
				if err != nil {
					return nil, err
				}
				return c.HoldReservation(func() error { return releaseReservation(additional) })
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	fmt.Fprintln(os.Stdout, "forwarding-interior-component-listening")
	var stop [1]byte
	if _, err := io.ReadFull(os.Stdin, stop[:]); err != nil || stop[0] != 's' {
		t.Fatal("original component stop missing", err)
	}
	// Every actual remote borrower joins before purpose/Hosting/State roots and
	// the transferred original credential descriptors return to their owner.
	routeErr := remote.Close()
	if routeErr != nil && os.Getenv("ARDENTS_FORWARDING_INSTALLED_CASE") != "unknown-domain-join" {
		t.Error("Interior Receiver join", routeErr)
	}
	if err := spend.Close(); err != nil {
		t.Error("Interior receiving join", err)
	}
	if err := budget.Close(); err != nil {
		t.Error("Interior Hosting join", err)
	}
	clear(key)
	if err := keyFile.Close(); err != nil {
		t.Error("Interior original key descriptor join", err)
	}
	if err := certificateFile.Close(); err != nil {
		t.Error("Interior original certificate descriptor join", err)
	}
	if err := owner.Close(); err != nil {
		t.Error("Interior State join", err)
	}
	if !t.Failed() {
		if routeErr != nil {
			fmt.Fprintln(os.Stderr, routeErr)
			fmt.Fprintln(os.Stdout, "forwarding-interior-component-joined-with-route-failure")
		} else {
			fmt.Fprintln(os.Stdout, "forwarding-interior-component-joined")
		}
	}
}
