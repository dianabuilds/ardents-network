package node_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestRouteRejectsInvalidHelloBeforeSpend sends independently encoded wire
// bytes over real role TLS to the compiled receiving command.
func TestRouteRejectsInvalidHelloBeforeSpend(t *testing.T) {
	testRouteReceiverAdmission(t, false)
}

func TestRouteReceiverClockLossAfterRefillSpend(t *testing.T) {
	testRouteReceiverAdmission(t, true)
}

func testRouteReceiverAdmission(t *testing.T, clockLoss bool) {
	t.Helper()
	nodeBinary := buildNodeCommand(t)
	processfixture.WaitAdmissionHour(t)
	spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, processfixture.TempDir(t))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	spec.Nodes[1].Endpoint = listener.Addr().String()
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	networkinputs.PrepareIssuer(t, &issuerPlan, inventory, spec, bundle.Nodes[0], sha256.Sum256(bundle.Profile))
	clock, stopClock := networkinputs.StartObservedClock(t)
	stopClockOnce := sync.OnceFunc(stopClock)
	defer stopClockOnce()
	var authorityKey [32]byte
	copy(authorityKey[:], spec.Authority.Public().(ed25519.PublicKey))
	plan := networkinputs.SeedNetworkState(t, spec.NetworkID, authorityKey, bundle, clock)
	config, err := plan.Config()
	if err != nil {
		t.Fatal(err)
	}
	authority, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	view, viewErr := authority.CurrentRuntime()
	if viewErr != nil {
		t.Fatal(viewErr)
	}
	member, memberErr := view.Member(spec.Nodes[1].NodeID, view.ObservedAt())
	facts, _, factsErr := funding.NetworkFacts(view)
	closeErr := authority.Close()
	if memberErr != nil || factsErr != nil || closeErr != nil {
		t.Fatal("signed receiver authority", memberErr, factsErr, closeErr)
	}
	certificate := processfixture.RoleCertificate(t, spec.Nodes[1].PrivateKey)
	certPath, keyPath := processfixture.WriteNativeCredential(t, certificate)
	spendRoot := processfixture.TempDir(t)
	budgetRoot := processfixture.ProcessBudget(t)
	receiver := map[string]any{"network": plan, "node_id": member.NodeID, "spend_root": spendRoot, "hosting_root": budgetRoot,
		"certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 16 << 20, Rx: 16 << 20},
		"termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	receiverConfig := processfixture.WriteConfig(t, receiver)
	peer := startRouteBootstrapPeer(t, nodeBinary, receiverConfig)
	journalPath := filepath.Join(spendRoot, "closed-token-spends")
	before, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal("original spending journal", err)
	}
	deadline := time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)
	if facts.NotAfter.Before(deadline) {
		deadline = facts.NotAfter
	}
	seed := func() state.AuthorityDeclaration {
		t.Helper()
		return networkinputs.SeedNetworkState(t, spec.NetworkID, authorityKey, bundle, clock)
	}
	holder, closeHolder := admissionLocalConsole(t, nodeBinary, "holder", map[string]any{"root": processfixture.TempDir(t), "network": seed(), "role": admission.AllocationUser})
	request := holder(holderCommand{Operation: "request", Maxima: [3]uint32{0, 5, 0}})
	if request.Outcome != "completed" {
		t.Fatal("holder request", request.Outcome)
	}
	permission := processfixture.FixturePermission(t, request.Request, facts, spec.Authority, time.Now())
	if reply := holder(holderCommand{Operation: "import", Digest: request.Digest, Payload: permission}); reply.Outcome != "completed" {
		t.Fatal("holder import", reply.Outcome)
	}
	challenge := token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
		ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration, Class: 2, WindowStart: time.Now().UTC().Truncate(time.Hour)}
	intent := stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{challenge, challenge, challenge, challenge, challenge}, Selection: stock.ExchangeBinding{ID: [32]byte{31}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter}
	batch := holder(holderCommand{Operation: "begin", Intent: intent})
	if batch.Outcome != "completed" {
		t.Fatal("holder begin", batch.Outcome)
	}
	issued := admissionLocalCommand(t, nodeBinary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch.Request, "kind": quota.Bootstrap})
	if issued.Outcome != "issued-offline" {
		t.Fatal("issuer response", issued.Outcome)
	}
	if reply := holder(holderCommand{Operation: "complete", Payload: issued.Response}); reply.Outcome != "completed" {
		t.Fatal("holder complete", reply.Outcome)
	}
	presentation := stock.Presentation{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest,
		ProfileDigest: facts.Digest, RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration, ChannelNonce: [32]byte{1}, Deadline: deadline}
	resultToken := holder(holderCommand{Operation: "take", Presentation: presentation, Class: 2})
	if resultToken.Outcome != "completed" || len(resultToken.Token) != 354 {
		t.Fatal("holder token", resultToken.Outcome, len(resultToken.Token))
	}
	presentation.ChannelNonce = [32]byte{2}
	lostToken := holder(holderCommand{Operation: "take", Presentation: presentation, Class: 2})
	if lostToken.Outcome != "completed" || len(lostToken.Token) != 354 {
		t.Fatal("lost-response token", lostToken.Outcome, len(lostToken.Token))
	}
	presentation.ChannelNonce = [32]byte{1}
	refillToken := holder(holderCommand{Operation: "take", Presentation: presentation, Class: 2})
	if refillToken.Outcome != "completed" || len(refillToken.Token) != 354 {
		t.Fatal("refill token", refillToken.Outcome, len(refillToken.Token))
	}
	presentation.ChannelNonce = [32]byte{3}
	secondParentToken := holder(holderCommand{Operation: "take", Presentation: presentation, Class: 2})
	lostRefillToken := holder(holderCommand{Operation: "take", Presentation: presentation, Class: 2})
	if secondParentToken.Outcome != "completed" || len(secondParentToken.Token) != 354 || lostRefillToken.Outcome != "completed" || len(lostRefillToken.Token) != 354 {
		t.Fatal("lost-refill tokens", secondParentToken.Outcome, lostRefillToken.Outcome)
	}
	closeHolder()
	defer clear(secondParentToken.Token)
	defer clear(lostRefillToken.Token)
	defer clear(refillToken.Token)
	defer clear(resultToken.Token)
	defer clear(lostToken.Token)
	base := make([]byte, 16, 16+209)
	copy(base, "ARDP")
	binary.BigEndian.PutUint16(base[4:6], 3)
	base[6] = 1
	binary.BigEndian.PutUint32(base[12:16], 209)
	for _, value := range [][32]byte{facts.NetworkID, facts.StateGeneration, facts.StateDigest, facts.Digest, member.NodeID} {
		base = append(base, value[:]...)
	}
	base = binary.BigEndian.AppendUint64(base, member.DutyGeneration)
	base = append(base, 7) // forwarding
	base = append(base, 1)
	base = append(base, make([]byte, 31)...)
	base = binary.BigEndian.AppendUint64(base, uint64(deadline.Unix()))
	if len(base) != 225 {
		t.Fatal("independent HELLO fixture has wrong length", len(base))
	}
	open := func(t *testing.T) *tls.Conn {
		t.Helper()
		cfg := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, NextProtos: []string{"ardents-route-v3"},
			InsecureSkipVerify: true, // Independently pin the signed Node key below.
			VerifyConnection: func(s tls.ConnectionState) error {
				if s.NegotiatedProtocol != "ardents-route-v3" || len(s.PeerCertificates) != 1 {
					return errors.New("role TLS identity absent")
				}
				cert := s.PeerCertificates[0]
				key, ok := cert.PublicKey.(ed25519.PublicKey)
				if !ok || !bytes.Equal(key, member.PublicKey[:]) {
					return errors.New("role TLS key differs from signed State")
				}
				if time.Now().Before(cert.NotBefore) || time.Now().After(cert.NotAfter) {
					return errors.New("role TLS certificate expired")
				}
				return cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature)
			}}
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp", member.Endpoint, cfg)
		if err != nil {
			t.Fatal("role TLS", err)
		}
		return conn
	}
	var result [1]byte
	if !clockLoss {
		// A correct HELLO waits for ADMIT: this control detects an independently
		// encoded frame rejected before the field-specific negative cases.
		control := open(t)
		if _, err := control.Write(base); err != nil {
			t.Fatal(err)
		}
		if err := control.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		if _, err := control.Read(result[:]); !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatal("valid HELLO did not await Admission", err)
		}
		if err := control.Close(); err != nil {
			t.Fatal(err)
		}
		budgetPath := filepath.Join(budgetRoot, "budget.json")
		beforeBudget, err := os.ReadFile(budgetPath)
		if err != nil {
			t.Fatal("original Hosting budget", err)
		}
		for _, fault := range []string{"recipient", "duty", "purpose", "state", "profile", "deadline"} {
			t.Run(fault, func(t *testing.T) {
				frame := bytes.Clone(base)
				switch fault {
				case "recipient":
					frame[16+128] ^= 1
				case "duty":
					frame[16+160+7]++
				case "purpose":
					frame[16+168] = 6
				case "state":
					frame[16+32] ^= 1
				case "profile":
					frame[16+96] ^= 1
				case "deadline":
					binary.BigEndian.PutUint64(frame[len(frame)-8:], uint64(member.NotAfter().Add(time.Second).Unix()))
				}
				// An invalid HELLO alone must be rejected before the receiver waits
				// for any token. The valid-HELLO control above proves that a correctly
				// encoded request does wait for ADMIT at this boundary.
				helloOnly := open(t)
				if _, err := helloOnly.Write(frame); err != nil {
					t.Fatal(err)
				}
				if err := helloOnly.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
					t.Fatal(err)
				}
				if _, err := helloOnly.Read(result[:]); err == nil || errors.Is(err, os.ErrDeadlineExceeded) {
					t.Fatal("invalid HELLO was accepted or awaited Admission", err)
				}
				if err := helloOnly.Close(); err != nil {
					t.Fatal(err)
				}
				conn := open(t)
				defer conn.Close()
				admit := make([]byte, 16, 16+355)
				copy(admit, "ARDP")
				binary.BigEndian.PutUint16(admit[4:6], 3)
				admit[6] = 2
				binary.BigEndian.PutUint32(admit[12:16], 355)
				admit = append(admit, 2)
				admit = append(admit, resultToken.Token...)
				frame = append(frame, admit...)
				if _, err := conn.Write(frame); err != nil {
					t.Fatal(err)
				}
				if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
					t.Fatal(err)
				}
				if _, err := conn.Read(result[:]); err == nil || errors.Is(err, os.ErrDeadlineExceeded) {
					t.Fatal("invalid HELLO was accepted or awaited Admission", err)
				}
				journal, err := os.ReadFile(journalPath)
				if err != nil || !bytes.Equal(before, journal) {
					t.Fatal("invalid HELLO changed spending journal", err)
				}
				budget, err := os.ReadFile(budgetPath)
				if err != nil || !bytes.Equal(beforeBudget, budget) {
					t.Fatal("invalid HELLO changed Hosting budget", err)
				}
			})
		}
		afterInvalid, err := os.ReadFile(journalPath)
		if err != nil || !bytes.Equal(before, afterInvalid) {
			t.Fatal("invalid HELLO changed spending journal", err)
		}
	}
	// The same genuine token must be accepted with the original HELLO. This
	// proves it was spendable and was not consumed by any invalid attempt.
	positive := open(t)
	admit := make([]byte, 16, 16+355)
	copy(admit, "ARDP")
	binary.BigEndian.PutUint16(admit[4:6], 3)
	admit[6] = 2
	binary.BigEndian.PutUint32(admit[12:16], 355)
	admit = append(admit, 2)
	admit = append(admit, resultToken.Token...)
	if _, err := positive.Write(append(bytes.Clone(base), admit...)); err != nil {
		t.Fatal(err)
	}
	if err := positive.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var acceptance [21]byte
	if _, err := io.ReadFull(positive, acceptance[:]); err != nil || string(acceptance[:4]) != "ARDP" || acceptance[6] != 5 || acceptance[16] != 0 {
		t.Fatal("valid token was not accepted", err, acceptance[:16])
	}
	afterInitial, err := os.ReadFile(journalPath)
	if err != nil || bytes.Equal(before, afterInitial) {
		t.Fatal("accepted token did not change spending journal", err)
	}
	// Refill the same real Forwarding parent with a second genuine right. The
	// two ADMIT frames exercise the actual receiver's incremental reservation
	// and spending path, not a reconstructed receiving owner.
	refillAdmit := bytes.Clone(admit)
	copy(refillAdmit[17:], refillToken.Token)
	if _, err := positive.Write(refillAdmit); err != nil {
		t.Fatal("Forwarding refill", err)
	}
	if _, err := io.ReadFull(positive, acceptance[:]); err != nil || string(acceptance[:4]) != "ARDP" || acceptance[6] != 5 || acceptance[16] != 0 {
		t.Fatal("Forwarding refill was not accepted", err, acceptance[:16])
	}
	afterFirst, err := os.ReadFile(journalPath)
	if err != nil || bytes.Equal(afterInitial, afterFirst) {
		t.Fatal("refill did not durably spend a fresh right", err)
	}
	// Reserve the remaining host capacity through the real Hosting owner. A
	// subsequent refill must refuse before spending its still-valid right.
	budgetOwner, err := hosting.Open(budgetRoot)
	if err != nil {
		t.Fatal("open shared Hosting budget", err)
	}
	available, err := budgetOwner.Observe(t.Context())
	if err != nil || available.RemainingBytes <= 1501 {
		t.Fatal("observe refill capacity", available.RemainingBytes, err)
	}
	blocker, err := budgetOwner.Reserve(t.Context(), hosting.ReservationRequest{
		Work: hosting.Traffic{Tx: available.RemainingBytes - 1501}, Termination: hosting.Traffic{Tx: 1},
		WorkUntil: deadline, HoldUntil: deadline.Add(time.Second),
	})
	if err != nil {
		t.Fatal("reserve competing Hosting capacity", err)
	}
	blockedAdmit := bytes.Clone(admit)
	copy(blockedAdmit[17:], lostToken.Token)
	if _, err := positive.Write(blockedAdmit); err != nil {
		t.Fatal("capacity-limited refill", err)
	}
	if err := positive.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := positive.Read(acceptance[:]); err == nil || errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("capacity-limited refill accepted or stalled", err)
	}
	afterBlocked, err := os.ReadFile(journalPath)
	if err != nil || !bytes.Equal(afterFirst, afterBlocked) {
		t.Fatal("capacity refusal spent a fresh right", err)
	}
	if err := errors.Join(positive.Close(), blocker.ReleaseJoined(), budgetOwner.Close()); err != nil {
		t.Fatal("capacity refusal cleanup", err)
	}
	// Lose the second response after the real receiver has durably spent the
	// right. The client deliberately never reads the receiver's response.
	lostHello := bytes.Clone(base)
	lostHello[16+169] = 2
	lostAdmit := bytes.Clone(admit)
	copy(lostAdmit[17:], lostToken.Token)
	lost := open(t)
	if _, err := lost.Write(append(bytes.Clone(lostHello), lostAdmit...)); err != nil {
		t.Fatal("lost-response presentation", err)
	}
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	var afterLost []byte
waitForSpend:
	for {
		afterLost, err = os.ReadFile(journalPath)
		if err != nil {
			t.Fatal("lost-response spending journal", err)
		}
		if !bytes.Equal(afterFirst, afterLost) {
			break waitForSpend
		}
		select {
		case <-timer.C:
			t.Fatal("lost-response token was not durably spent")
		case <-t.Context().Done():
			t.Fatal("waiting for lost-response spend", t.Context().Err())
		case <-ticker.C:
		}
	}
	timer.Stop()
	ticker.Stop()
	if err := lost.Close(); err != nil {
		t.Fatal("close after lost response", err)
	}
	// Open a second real Forwarding parent, then abandon its refill response
	// only after the receiving owner's durable spend becomes visible.
	refillHello := bytes.Clone(base)
	refillHello[16+169] = 3
	secondAdmit := bytes.Clone(admit)
	copy(secondAdmit[17:], secondParentToken.Token)
	lostRefillAdmit := bytes.Clone(admit)
	copy(lostRefillAdmit[17:], lostRefillToken.Token)
	second := open(t)
	if _, err := second.Write(append(bytes.Clone(refillHello), secondAdmit...)); err != nil {
		t.Fatal("second parent admission", err)
	}
	if err := second.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(second, acceptance[:]); err != nil || string(acceptance[:4]) != "ARDP" || acceptance[6] != 5 || acceptance[16] != 0 {
		t.Fatal("second parent admission refused", err)
	}
	beforeLostRefill, err := os.ReadFile(journalPath)
	if err != nil || bytes.Equal(afterLost, beforeLostRefill) {
		t.Fatal("second parent did not spend its initial right", err)
	}
	if _, err := second.Write(lostRefillAdmit); err != nil {
		t.Fatal("lost-response refill", err)
	}
	refillTimer := time.NewTimer(3 * time.Second)
	defer refillTimer.Stop()
	refillTicker := time.NewTicker(10 * time.Millisecond)
	defer refillTicker.Stop()
	var afterLostRefill []byte
waitForRefillSpend:
	for {
		afterLostRefill, err = os.ReadFile(journalPath)
		if err != nil {
			t.Fatal("lost-refill spending journal", err)
		}
		if !bytes.Equal(beforeLostRefill, afterLostRefill) {
			break waitForRefillSpend
		}
		select {
		case <-refillTimer.C:
			t.Fatal("lost-response refill was not durably spent")
		case <-t.Context().Done():
			t.Fatal("waiting for lost-refill spend", t.Context().Err())
		case <-refillTicker.C:
		}
	}
	if clockLoss {
		stopClockOnce()
		if err := os.Remove(clock); err != nil {
			t.Fatal("remove receiver clock observation", err)
		}
	} else if err := processfixture.Interrupt(peer.command); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal("close after unread refill response", err)
	}
	joined := make(chan error, 1)
	go func() { joined <- peer.command.Wait() }()
	var waitErr error
	select {
	case waitErr = <-joined:
		peer.waited = true
	case <-time.After(3 * time.Second):
		_ = peer.command.Process.Kill()
		<-joined
		peer.waited = true
		t.Fatal("receiver did not join")
	}
	var exit *exec.ExitError
	expectedExit := 130
	if clockLoss {
		expectedExit = 1
	}
	if !errors.As(waitErr, &exit) || exit.ExitCode() != expectedExit {
		t.Fatal("receiver did not retain its terminal outcome", waitErr, peer.diagnostic.String())
	}
	if clockLoss {
		var terminal map[string]string
		for _, line := range bytes.Split(peer.diagnostic.Bytes(), []byte{'\n'}) {
			var event map[string]string
			if json.Unmarshal(line, &event) == nil && event["operation"] == "route.receive" && event["phase"] == "joined" {
				terminal = event
			}
		}
		if terminal["outcome"] != "failed" || terminal["stage"] != "terminal" {
			t.Fatal("receiver lost clock failure", terminal)
		}
		if err := os.WriteFile(clock, nil, 0600); err != nil {
			t.Fatal("restore receiver clock observation", err)
		}
		stopRestoredClock := networkinputs.RefreshObservedClock(t, clock)
		defer stopRestoredClock()
	}
	after, err := os.ReadFile(journalPath)
	if err != nil || !bytes.Equal(afterLostRefill, after) {
		t.Fatal("receiver shutdown changed spent rights", err)
	}
	// Reopen the same process roots and replay the exact accepted token. The
	// durable burn must refuse it without changing the retained journal.
	peer = startRouteBootstrapPeer(t, nodeBinary, receiverConfig)
	for _, packet := range [][]byte{append(bytes.Clone(base), admit...), append(bytes.Clone(base), refillAdmit...), append(bytes.Clone(lostHello), lostAdmit...), append(bytes.Clone(refillHello), secondAdmit...), append(bytes.Clone(refillHello), lostRefillAdmit...)} {
		replay := open(t)
		if _, err := replay.Write(packet); err != nil {
			t.Fatal(err)
		}
		if err := replay.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := replay.Read(result[:]); err == nil || errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatal("spent token was accepted or replay stalled", err)
		}
		if err := replay.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := processfixture.Interrupt(peer.command); err != nil {
		t.Fatal(err)
	}
	waitErr = peer.command.Wait()
	peer.waited = true
	if !errors.As(waitErr, &exit) || exit.ExitCode() != 130 {
		t.Fatal("reopened receiver did not join", waitErr, peer.diagnostic.String())
	}
	afterReplay, err := os.ReadFile(journalPath)
	if err != nil || !bytes.Equal(after, afterReplay) {
		t.Fatal("replay changed retained spending journal", err)
	}
	budget, err := hosting.Open(budgetRoot)
	if err != nil {
		t.Fatal(err)
	}
	observation, observeErr := budget.Observe(t.Context())
	closeErr = budget.Close()
	if observeErr != nil || closeErr != nil || observation.ReservedBytes != 0 {
		t.Fatal("invalid HELLO reserved capacity", observeErr, closeErr, observation.ReservedBytes)
	}
}

// TestRouteNodeBootstrapRestrictionsAndPendingLimit exercises the receiving
// process with authenticated Node TLS and independently encoded wire frames.
func TestRouteNodeBootstrapRestrictionsAndPendingLimit(t *testing.T) {
	binaryPath := buildNodeCommand(t)
	processfixture.WaitAdmissionHour(t)
	spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, processfixture.TempDir(t))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	spec.Nodes[1].Endpoint = listener.Addr().String()
	spec.Nodes[1].RoleDomain, spec.Nodes[1].Subrole = 1, 2
	listeners := make(map[[32]byte]net.Listener)
	certificates := make(map[[32]byte]tls.Certificate)
	processfixture.AddReceivingFixtureNode(t, &spec, listeners, certificates, spec.Nodes[1].Carrier, 13, 1, 1)
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal("signed Node state", err)
	}
	networkinputs.PrepareIssuer(t, &issuerPlan, inventory, spec, bundle.Nodes[0], sha256.Sum256(bundle.Profile))
	clock, stopClock := networkinputs.StartObservedClock(t)
	defer stopClock()
	var authorityKey [32]byte
	copy(authorityKey[:], spec.Authority.Public().(ed25519.PublicKey))
	plan := networkinputs.SeedNetworkState(t, spec.NetworkID, authorityKey, bundle, clock)
	config, err := plan.Config()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		_ = owner.Close()
		t.Fatal(err)
	}
	member, err := view.Member(spec.Nodes[1].NodeID, view.ObservedAt())
	facts, now, factsErr := funding.NetworkFacts(view)
	closeErr := owner.Close()
	if err != nil || factsErr != nil || closeErr != nil {
		t.Fatal("Node authority", err, factsErr, closeErr)
	}
	certPath, keyPath := processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, spec.Nodes[1].PrivateKey))
	spendRoot, budgetRoot := processfixture.TempDir(t), processfixture.ProcessBudget(t)
	receiver := map[string]any{"network": plan, "node_id": member.NodeID, "spend_root": spendRoot, "hosting_root": budgetRoot,
		"certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20},
		"termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	peer := startRouteBootstrapPeer(t, binaryPath, processfixture.WriteConfig(t, receiver))
	spendPath := filepath.Join(spendRoot, "closed-token-spends")
	initialSpend, err := os.ReadFile(spendPath)
	if err != nil {
		t.Fatal("initial Admission history", err)
	}
	budgetPath := filepath.Join(budgetRoot, "budget.json")
	initialBudget, err := os.ReadFile(budgetPath)
	if err != nil {
		t.Fatal("initial Hosting budget", err)
	}
	seed := func() state.AuthorityDeclaration {
		t.Helper()
		return networkinputs.SeedNetworkState(t, spec.NetworkID, authorityKey, bundle, clock)
	}
	holder, closeHolder := admissionLocalConsole(t, binaryPath, "holder", map[string]any{"root": processfixture.TempDir(t), "network": seed(), "role": admission.AllocationUser})
	request := holder(holderCommand{Operation: "request", Maxima: [3]uint32{0, 2, 0}})
	if request.Outcome != "completed" {
		t.Fatal("holder request", request.Outcome)
	}
	permission := processfixture.FixturePermission(t, request.Request, facts, spec.Authority, now)
	if reply := holder(holderCommand{Operation: "import", Digest: request.Digest, Payload: permission}); reply.Outcome != "completed" {
		t.Fatal("holder permission", reply.Outcome)
	}
	challenge := token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
		ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration, Class: 2, WindowStart: now.UTC().Truncate(time.Hour)}
	intent := stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{challenge, challenge}, Selection: stock.ExchangeBinding{ID: [32]byte{53}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter}
	batch := holder(holderCommand{Operation: "begin", Intent: intent})
	if batch.Outcome != "completed" {
		t.Fatal("holder batch", batch.Outcome)
	}
	issued := admissionLocalCommand(t, binaryPath, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch.Request, "kind": quota.Bootstrap})
	if issued.Outcome != "issued-offline" {
		t.Fatal("issuer result", issued.Outcome)
	}
	if reply := holder(holderCommand{Operation: "complete", Payload: issued.Response}); reply.Outcome != "completed" {
		t.Fatal("holder completion", reply.Outcome)
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		NextProtos: []string{"ardents-route-v3"}, Certificates: []tls.Certificate{certificates[[32]byte{13}]},
		InsecureSkipVerify: true, // Verify the signed receiver key and certificate below.
		VerifyConnection: func(s tls.ConnectionState) error {
			if s.NegotiatedProtocol != "ardents-route-v3" || len(s.PeerCertificates) != 1 {
				return errors.New("Node TLS identity absent")
			}
			cert := s.PeerCertificates[0]
			key, ok := cert.PublicKey.(ed25519.PublicKey)
			if !ok || !bytes.Equal(key, member.PublicKey[:]) || time.Now().Before(cert.NotBefore) || time.Now().After(cert.NotAfter) {
				return errors.New("Node TLS key or validity differs from signed State")
			}
			return cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature)
		}}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp", member.Endpoint, cfg)
	if err != nil {
		t.Fatal("authenticated Node TLS", err)
	}
	defer conn.Close()
	deadline := time.Now().Add(15 * time.Second).UTC().Truncate(time.Second)
	if facts.NotAfter.Before(deadline) {
		deadline = facts.NotAfter
	}
	var rights [2][]byte
	for i := range rights {
		presentation := stock.Presentation{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest,
			ProfileDigest: facts.Digest, RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration,
			ChannelNonce: [32]byte{byte(21 + i)}, Deadline: deadline}
		reply := holder(holderCommand{Operation: "take", Presentation: presentation, Class: 2})
		if reply.Outcome != "completed" || len(reply.Token) != 354 {
			t.Fatal("genuine Forward token", i, reply.Outcome, len(reply.Token))
		}
		rights[i] = reply.Token
		defer clear(rights[i])
	}
	closeHolder()
	hello := make([]byte, 16, 225)
	copy(hello, "ARDP")
	binary.BigEndian.PutUint16(hello[4:6], 3)
	hello[6] = 1
	binary.BigEndian.PutUint32(hello[12:16], 209)
	for _, value := range [][32]byte{facts.NetworkID, facts.StateGeneration, facts.StateDigest, facts.Digest, member.NodeID} {
		hello = append(hello, value[:]...)
	}
	hello = binary.BigEndian.AppendUint64(hello, member.DutyGeneration)
	hello = append(hello, 7, 1)
	hello = append(hello, make([]byte, 31)...)
	hello = binary.BigEndian.AppendUint64(hello, uint64(deadline.Unix()))
	if _, err := conn.Write(hello); err != nil {
		t.Fatal("Node HELLO", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var accepted [21]byte
	if _, err := io.ReadFull(conn, accepted[:]); err != nil || string(accepted[:4]) != "ARDP" || accepted[6] != 5 || accepted[16] != 0 {
		t.Fatal("Node HELLO was not accepted", err, accepted)
	}
	open := append([]byte(nil), member.NodeID[:]...)
	open = binary.BigEndian.AppendUint64(open, member.DutyGeneration)
	open = append(open, 7)
	open = binary.BigEndian.AppendUint64(open, uint64(deadline.Unix()))
	for i, body := range [][]byte{open, append(bytes.Clone(open), 2)} {
		lane := uint32(1 + 2*i)
		frame := make([]byte, 16, 16+len(body))
		copy(frame, "ARDP")
		binary.BigEndian.PutUint16(frame[4:6], 3)
		frame[6] = 4
		binary.BigEndian.PutUint32(frame[8:12], lane)
		binary.BigEndian.PutUint32(frame[12:16], uint32(len(body)))
		if _, err := conn.Write(append(frame, body...)); err != nil {
			t.Fatal("malformed Node OPEN", err)
		}
		if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var refusal [17]byte
		if _, err := io.ReadFull(conn, refusal[:]); err != nil || string(refusal[:4]) != "ARDP" || refusal[6] != 9 || binary.BigEndian.Uint32(refusal[8:12]) != lane || binary.BigEndian.Uint32(refusal[12:16]) != 1 || refusal[16] == 0 {
			t.Fatal("invalid restriction did not receive lane refusal", i, err, refusal)
		}
		spending, spendErr := os.ReadFile(spendPath)
		budget, budgetErr := os.ReadFile(budgetPath)
		if spendErr != nil || budgetErr != nil || !bytes.Equal(initialSpend, spending) || !bytes.Equal(initialBudget, budget) {
			t.Fatal("invalid restriction reached Admission or Hosting", i, spendErr, budgetErr)
		}
	}
	// A valid restricted OPEN on the same authenticated Carrier must reach
	// the real pending TLS reservation, so the refusals above are causal.
	valid := make([]byte, 16, 66)
	copy(valid, "ARDP")
	binary.BigEndian.PutUint16(valid[4:6], 3)
	valid[6] = 4
	binary.BigEndian.PutUint32(valid[8:12], 5)
	binary.BigEndian.PutUint32(valid[12:16], 50)
	valid = append(valid, open...)
	valid = append(valid, 1)
	if _, err := conn.Write(valid); err != nil {
		t.Fatal("valid restricted Node OPEN", err)
	}
	budget, err := hosting.Open(budgetRoot)
	if err != nil {
		t.Fatal("observe pending Hosting", err)
	}
	end := time.Now().Add(3 * time.Second)
	var oneReservation uint64
	for {
		observation, err := budget.Observe(t.Context())
		if err != nil {
			_ = budget.Close()
			t.Fatal("observe pending Hosting", err)
		}
		if observation.ReservedBytes != 0 {
			oneReservation = observation.ReservedBytes
			break
		}
		if time.Now().After(end) {
			_ = budget.Close()
			t.Fatal("valid restricted OPEN did not reserve pending TLS work")
		}
		time.Sleep(10 * time.Millisecond)
	}
	for lane := uint32(7); lane <= 11; lane += 2 {
		frame := bytes.Clone(valid)
		binary.BigEndian.PutUint32(frame[8:12], lane)
		if _, err := conn.Write(frame); err != nil {
			_ = budget.Close()
			t.Fatal("pending restricted Node OPEN", lane, err)
		}
	}
	// Each admitted lane reserves Hosting in its own goroutine. Observe
	// all four live reservations before sending the fifth OPEN.
	wantReserved := 4 * oneReservation
	limit := time.Now().Add(3 * time.Second)
	for {
		observation, err := budget.Observe(t.Context())
		if err != nil || observation.ReservedBytes > wantReserved {
			_ = budget.Close()
			t.Fatalf("pending Hosting reservations: got %d, want at most %d: %v", observation.ReservedBytes, wantReserved, err)
		}
		if observation.ReservedBytes == wantReserved {
			break
		}
		if time.Now().After(limit) {
			_ = budget.Close()
			t.Fatalf("pending Hosting reservations did not reach %d: last observation %d", wantReserved, observation.ReservedBytes)
		}
		time.Sleep(10 * time.Millisecond)
	}
	fifth := bytes.Clone(valid)
	binary.BigEndian.PutUint32(fifth[8:12], 13)
	if _, err := conn.Write(fifth); err != nil {
		_ = budget.Close()
		t.Fatal("fifth restricted Node OPEN", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		_ = budget.Close()
		t.Fatal(err)
	}
	var refusal [17]byte
	if _, err := io.ReadFull(conn, refusal[:]); err != nil || string(refusal[:4]) != "ARDP" || refusal[6] != 9 || binary.BigEndian.Uint32(refusal[8:12]) != 13 || binary.BigEndian.Uint32(refusal[12:16]) != 1 || refusal[16] == 0 {
		_ = budget.Close()
		t.Fatal("fifth pending bootstrap did not receive lane refusal", err, refusal)
	}
	observation, observeErr := budget.Observe(t.Context())
	spending, spendErr := os.ReadFile(spendPath)
	if observeErr != nil || spendErr != nil || observation.ReservedBytes != wantReserved || !bytes.Equal(initialSpend, spending) {
		_ = budget.Close()
		t.Fatalf("pending bootstrap after refusal: Hosting bytes %d, want %d; Admission changed %t; errors: %v / %v",
			observation.ReservedBytes, wantReserved, !bytes.Equal(initialSpend, spending), observeErr, spendErr)
	}
	if err := budget.Close(); err != nil {
		t.Fatal("close Hosting observer", err)
	}
	roleConfig := cfg.Clone()
	roleConfig.Certificates = nil
	roleConfig.VerifyConnection = cfg.VerifyConnection
	openRole := func(id uint32, nonce byte, right []byte) (*tls.Conn, error) {
		lane := &e2eRouteWireLane{Conn: conn, id: id}
		secured := tls.Client(lane, roleConfig)
		if err := secured.HandshakeContext(t.Context()); err != nil {
			return secured, err
		}
		inner := bytes.Clone(hello)
		inner[16+169] = nonce
		admit := make([]byte, 16, 16+355)
		copy(admit, "ARDP")
		binary.BigEndian.PutUint16(admit[4:6], 3)
		admit[6] = 2
		binary.BigEndian.PutUint32(admit[12:16], 355)
		admit = append(admit, 2)
		admit = append(admit, right...)
		if _, err := secured.Write(append(inner, admit...)); err != nil {
			return secured, err
		}
		var answer [21]byte
		if _, err := io.ReadFull(secured, answer[:]); err != nil {
			return secured, err
		}
		if string(answer[:4]) != "ARDP" || answer[6] != 5 || answer[16] != 0 {
			return secured, errors.New("role Admission did not accept")
		}
		return secured, nil
	}
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, restrictedErr := openRole(5, 21, rights[0])
	if restrictedErr == nil || errors.Is(restrictedErr, os.ErrDeadlineExceeded) {
		t.Fatal("restricted Bootstrap child accepted private Admission or stalled", restrictedErr)
	}
	afterRestricted, err := os.ReadFile(spendPath)
	if err != nil || !bytes.Equal(initialSpend, afterRestricted) {
		t.Fatal("restricted Bootstrap spent a Forward token", err)
	}
	ordinary := bytes.Clone(valid)
	binary.BigEndian.PutUint32(ordinary[8:12], 15)
	ordinary[len(ordinary)-1] = 0
	if _, err := conn.Write(ordinary); err != nil {
		t.Fatal("ordinary sibling OPEN", err)
	}
	if _, err := openRole(15, 22, rights[1]); err != nil {
		t.Fatal("ordinary sibling Admission", err)
	}
	afterOrdinary, err := os.ReadFile(spendPath)
	if err != nil || bytes.Equal(initialSpend, afterOrdinary) {
		t.Fatal("ordinary sibling did not durably spend its independent token", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal("Node Carrier close", err)
	}
	finishRouteJoinPeers(t, []*routeBootstrapPeer{peer}, []string{budgetRoot})
}

// e2eRouteWireLane transports one small TLS exchange through independently
// encoded ARDP BYTES frames. The real receiver owns all lane decisions.
type e2eRouteWireLane struct {
	net.Conn
	id      uint32
	pending []byte
}

func (l *e2eRouteWireLane) Write(p []byte) (int, error) {
	total := 0
	for len(p) != 0 {
		n := min(len(p), 16<<10)
		frame := make([]byte, 16, 16+n)
		copy(frame, "ARDP")
		binary.BigEndian.PutUint16(frame[4:6], 3)
		frame[6] = 6
		binary.BigEndian.PutUint32(frame[8:12], l.id)
		binary.BigEndian.PutUint32(frame[12:16], uint32(n))
		frame = append(frame, p[:n]...)
		if _, err := l.Conn.Write(frame); err != nil {
			return total, err
		}
		total += n
		p = p[n:]
	}
	return total, nil
}

func (l *e2eRouteWireLane) Read(p []byte) (int, error) {
	for len(l.pending) == 0 {
		var header [16]byte
		if _, err := io.ReadFull(l.Conn, header[:]); err != nil {
			return 0, err
		}
		size := binary.BigEndian.Uint32(header[12:16])
		if string(header[:4]) != "ARDP" || binary.BigEndian.Uint16(header[4:6]) != 3 || size > 16<<10 || binary.BigEndian.Uint32(header[8:12]) != l.id {
			return 0, errors.New("unexpected route lane frame")
		}
		body := make([]byte, size)
		if _, err := io.ReadFull(l.Conn, body); err != nil {
			return 0, err
		}
		switch header[6] {
		case 6:
			l.pending = body
		case 7:
			continue
		case 8, 9:
			return 0, io.EOF
		default:
			return 0, errors.New("unexpected route lane frame kind")
		}
	}
	n := copy(p, l.pending)
	l.pending = l.pending[n:]
	return n, nil
}
