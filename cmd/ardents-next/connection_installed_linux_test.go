//go:build linux && text_worker_installed

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/connection"
	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	publicationruntime "github.com/dianabuilds/ardents-network/internal/successor/publication/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// nonceResultInterruptionOnly checks every retained error leaf. A deliberate
// Reader interruption cannot excuse unrelated physical cleanup failures.
func nonceResultInterruptionOnly(err error) bool {
	if err == nil {
		return false
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		children := many.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !nonceResultInterruptionOnly(child) {
				return false
			}
		}
		return true
	}
	if err == io.EOF || err == context.Canceled || err == net.ErrClosed {
		return true
	}
	if failure, ok := err.(*net.OpError); ok {
		return failure.Op == "write" && errors.Is(failure, syscall.ECONNRESET)
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return nonceResultInterruptionOnly(one.Unwrap())
	}
	return false
}

// Both original workers are genuinely qualified by the selected installed
// environment. Signed fixture authority, independent local roots and actual
// Carriers establish an initial authentication component, not two separately
// installed Endpoints, recovery, Application-byte acceptance or Terminal success.
func runInstalledConnectionAuthentication(t *testing.T, carrier transport.CarrierProfile) {
	runInstalledConnectionAuthenticationFailure(t, carrier, "")
}

func runInstalledConnectionAuthenticationFailure(t *testing.T, carrier transport.CarrierProfile, boundary string) {
	failedSetup := boundary != ""
	if boundary != "" && boundary != "join" && boundary != "tls" && boundary != "result" {
		t.Fatal("unknown authentication failure boundary")
	}
	f, reservations, certificates := installedPublisherNetwork(t, carrier)
	deliveryGate := &publicationDeliveryCloseGate{}
	defer deliveryGate.release()
	if failedSetup {
		originalCurrent := f.current
		f.current = func() (network.RuntimeView, error) {
			if boundary == "tls" {
				return deliveryGate.observeTLSClientRead(originalCurrent)
			}
			return deliveryGate.observe(originalCurrent)
		}
		f.authority = networkAdmissionAuthority(f.current, func() error { return nil })
	}
	ctx, cancel := context.WithTimeout(t.Context(), 65*time.Second)
	defer cancel()
	plan, target := installedPublisherInstance(t, f.profile.NetworkID)
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), Network: f.profile.NetworkID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	var rendezvous *receiver.Receiver
	servers := installedPublisherReceiversObserved(t, f, reservations, certificates, store, func(err error) bool { return boundary == "result" && nonceResultInterruptionOnly(err) }, func(id [32]byte, _ receiver.ReceiverConfig, server *receiver.Receiver) {
		if id == [32]byte{41} {
			rendezvous = server
		}
	})
	local, err := executionruntime.New(execution.Config{ID: [32]byte{81}, Grants: []execution.Grant{
		{Principal: [32]byte{82}, Surface: execution.Administration},
		{Principal: [32]byte{83}, Surface: execution.Connection},
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := local.Close(); err != nil {
			t.Error(err)
		}
	})
	publisherWorker, err := local.LaunchPublisher(ctx, [32]byte{82}, []byte("initial Service authentication snapshot\n"))
	if err != nil {
		t.Fatal(err)
	}
	publisherOperation, err := publisherWorker.BeginOperation(ctx)
	if err != nil {
		t.Fatal(errors.Join(err, publisherWorker.Close()))
	}
	t.Cleanup(func() {
		publisherOperation.Close()
		if err := publisherWorker.Close(); err != nil && !(boundary == "result" && nonceResultInterruptionOnly(err)) {
			t.Error(err)
		}
	})
	holder := installedConnectionPublisherStock(t, f)
	root := t.TempDir()
	end := time.Now().Add(55 * time.Second).UTC().Truncate(time.Second)
	routePlan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "introduction"),
		SourceInteriorRoot: filepath.Join(root, "source"), ResponderInteriorRoot: filepath.Join(root, "responder"),
		HostingRoot: routeProcessBudget(t), Domain: 4, Deadline: end,
		Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	published, err := startPublicationHolder(publisherOperation.Context(), plan, routePlan, f.authority, holder, publisherOperation)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := published.close(); err != nil && !(boundary == "result" && nonceResultInterruptionOnly(err)) {
			t.Error(err)
		}
	})
	link, err := published.publish(ctx)
	if err != nil {
		t.Fatal(err)
	}
	publisher, ok := published.owner.(*publicationruntime.Publisher)
	if !ok {
		t.Fatal("production composition did not retain its genuine Publisher")
	}
	readerCaller, cancelReader := context.WithCancel(ctx)
	defer cancelReader()
	readerWorker, err := local.LaunchReader(readerCaller, [32]byte{83})
	if err != nil {
		t.Fatal(err)
	}
	readerOperation, err := readerWorker.BeginOperation(readerCaller)
	if err != nil {
		t.Fatal(errors.Join(err, readerWorker.Close()))
	}
	t.Cleanup(func() {
		cancelReader()
		readerOperation.Close()
		if err := readerWorker.Close(); !errors.Is(err, context.Canceled) {
			t.Error(err)
		}
	})
	var expectedSourceClose error
	source := installedConnectionInitiatorObserved(t, f, readerOperation, link, end.Add(-5*time.Second), nil, installedReaderRecoveryStock{expectedClose: &expectedSourceClose})
	type authenticated struct {
		owner *connection.AuthenticatedAttachment
		err   error
	}
	incoming := make(chan authenticated, 1)
	authCaller, cancelAuthCaller := context.WithCancel(ctx)
	defer cancelAuthCaller()
	go func() {
		owner, err := publisher.NextAuthentication(authCaller)
		incoming <- authenticated{owner, err}
	}()
	waited := false
	t.Cleanup(func() {
		cancel()
		if !waited {
			result := <-incoming
			if result.owner != nil {
				_ = result.owner.Close()
			}
		}
	})
	if failedSetup {
		deliveryGate.arm()
		type attempt struct {
			owner *connection.AuthenticatedAttachment
			err   error
		}
		attempted := make(chan attempt, 1)
		go func() { owner, err := source.AuthenticateInitial(); attempted <- attempt{owner, err} }()
		select {
		case <-deliveryGate.entered:
		case <-ctx.Done():
			t.Fatal("original authentication failure boundary not observed", boundary, ctx.Err())
		}
		if boundary == "result" {
			cancelReader()
			<-readerOperation.Context().Done()
		} else {
			if rendezvous == nil {
				t.Fatal("original Rendezvous absent")
			}
			if err := rendezvous.Close(); err != nil {
				t.Fatal("original Rendezvous join", err)
			}

		}
		deliveryGate.release()
		got := <-attempted
		if got.owner != nil || got.err == nil {
			if got.owner != nil {
				_ = got.owner.Close()
			}
			t.Fatal("interrupted original setup accepted authentication", boundary, got.err)
		}
		if boundary == "result" {
			cancelAuthCaller()
		}
		result := <-incoming
		waited = true
		if result.owner != nil || result.err == nil {
			if result.owner != nil {
				_ = result.owner.Close()
			}
			t.Fatal("Publisher accepted failed authentication", boundary, result.err)
		}
		if (readerOperation.CheckReader() == nil) != (boundary != "result") || publisherOperation.CheckPublisher() != nil {
			t.Fatal("failed authentication replaced original Jobs", boundary)
		}
		closeErr := source.Close()
		expectedSourceClose = closeErr
		if closeErr == nil || source.Close() != closeErr {
			t.Fatal("failed setup lost original retained Source result", boundary, closeErr)
		}
		if err := published.close(); err != nil && !(boundary == "result" && nonceResultInterruptionOnly(err)) {
			t.Fatal("Publisher physical retirement", err)
		}
		if boundary == "result" {
			physical, joined := publisher.CleanupResult()
			if !joined || physical != nil && !nonceResultInterruptionOnly(physical) {
				t.Fatal("RESULT interrupted Publisher not joined with exact failure", physical, joined)
			}
			if physical != nil && !errors.Is(published.close(), physical) {
				t.Fatal("Publisher Close lost its original physical failure")
			}
			t.Log("original RESULT Publisher physical failure retained", physical != nil)
		}
		reflected := reflect.ValueOf(publisher).Elem().FieldByName("history").Elem().FieldByName("entries")
		keys := reflected.MapKeys()
		if len(keys) != 1 {
			t.Fatal("accepted nonce lost after failed authentication", boundary, len(keys))
		}
		entry := reflected.MapIndex(keys[0])
		if !entry.Elem().FieldByName("accepted").Bool() {
			t.Fatal("actual failed setup nonce was not retained accepted", boundary)
		}
		t.Log("genuine accepted nonce retained after original authentication failure and Publisher join", boundary)

	} else {
		initiator, err := source.AuthenticateInitial()
		if err != nil {
			select {
			case peer := <-incoming:
				waited = true
				if peer.owner != nil {
					t.Cleanup(func() {
						if err := peer.owner.Close(); err != nil {
							t.Error(err)
						}
					})
				}
				t.Log("original Publisher authentication result before Reader failure", peer.err)
			default:
				t.Log("original Publisher authentication still pending at Reader failure")
			}
			t.Fatal("original Reader initial Service authentication", err)
		}
		t.Cleanup(func() {
			if err := initiator.Close(); err != nil {
				t.Error(err)
			}
		})
		result := <-incoming
		waited = true
		if result.owner != nil {
			t.Cleanup(func() {
				if err := result.owner.Close(); err != nil {
					t.Error(err)
				}
			})
		}
		if result.err != nil || result.owner == nil {
			t.Fatal("original Publisher initial Service authentication", result.err)
		}
		// No Application stream is opened before genuine recovery composition.
		// Both roles must still retain the same original qualified operations.
		if err := errors.Join(readerOperation.CheckReader(), publisherOperation.CheckPublisher()); err != nil {
			t.Fatal("original operation lost at authentication handoff", err)
		}
		raw, _, err := store.Lookup(target, f.profile.Digest, time.Now())
		if err != nil {
			t.Fatal("independent signed Store observation after authentication", err)
		}
		if _, err := reachability.Verify(raw, target, f.profile.NetworkID, f.profile.Digest, time.Now()); err != nil {
			t.Fatal("independent signed Store proof after authentication", err)
		}
		if err := errors.Join(initiator.Close(), result.owner.Close(), source.Close()); err != nil {
			t.Fatal("original authenticated physical joins", err)
		}
		if err := published.close(); err != nil && !(boundary == "result" && nonceResultInterruptionOnly(err)) {
			t.Fatal("original Publisher withdrawal and physical join", err)
		}

	}
	// This component deliberately stops before Application transfer. Retain
	// genuine Reader cancellation, never classify an unused Text worker as a
	// successful document exchange. Publication's caller is independent.
	cancelReader()
	<-readerOperation.Context().Done()
	if readerOperation.CheckReader() == nil {
		t.Fatal("canceled original Reader still admitted effects")
	}
	readerOperation.Close()
	if err := readerWorker.Close(); !errors.Is(err, context.Canceled) || readerWorker.CompletedCurrent() {
		t.Fatal("pre-Application Reader supplied successful completion", err)
	}
	if physical, joined := readerWorker.CleanupResult(); !joined || physical != nil {
		t.Fatal("original canceled Reader descendants did not join", physical)
	}
	publisherOperation.Close()
	if boundary == "result" {
		closed := publisherWorker.Close()
		physical, joined := publisherWorker.CleanupResult()
		if !joined || physical != nil && (!nonceResultInterruptionOnly(physical) || !errors.Is(closed, physical)) {
			t.Fatal("original RESULT worker lost physical failure or join", closed, physical, joined)
		}
		if physical != nil && publisherWorker.CompletedCurrent() {
			t.Fatal("failed RESULT cleanup became successful Job")
		}
	}
	if err := errors.Join(publisherWorker.Close(), local.Close()); err != nil && !(boundary == "result" && nonceResultInterruptionOnly(err)) {
		t.Fatal("original worker and operation joins", err)
	}
	for _, server := range servers {
		if failedSetup && boundary != "result" && server == rendezvous {
			continue
		}
		receivedErr := server.Close()
		if boundary == "result" && receivedErr != nil && nonceResultInterruptionOnly(receivedErr) {
			if repeated := server.Close(); repeated != receivedErr {
				t.Fatal("receiving Close lost original retained interruption", repeated, receivedErr)
			}
			t.Log("original receiving RESULT interruption retained")
		}
		if err := receivedErr; err != nil && !(boundary == "result" && nonceResultInterruptionOnly(err)) {
			t.Fatal("original receiving physical join", err)
		}
	}
}

// The real sole Reader and snapshot worker perform one bounded request after
// both original recovery sources are installed. A restarted genuine receiving
// Rendezvous keeps its same Admission/Hosting/Network owners and durable spends.
// This is a native component, still not two installed system managers.
func runInstalledConnectionText(t *testing.T, carrier transport.CarrierProfile, loss bool) {
	timing := ""
	if loss {
		timing = "before-request"
	}
	runInstalledConnectionTextLoss(t, carrier, timing)
}

func runInstalledConnectionTextLoss(t *testing.T, carrier transport.CarrierProfile, timing string) {
	reopen := timing == "durable-reopen"
	if reopen {
		timing = ""
	}
	var controlGate *installedControlWriteGate
	if strings.HasPrefix(timing, "control-") {
		parts := strings.SplitN(strings.TrimPrefix(timing, "control-"), "-", 3)
		if len(parts) != 3 || parts[0] != "publisher" && parts[0] != "initiator" || parts[1] != "terminal" && parts[1] != "receipt" && parts[1] != "confirmation" || parts[2] != "before-write" && parts[2] != "after-write" && parts[2] != "after-receive" {
			t.Fatal("unknown control write boundary")
		}
		controlGate = &installedControlWriteGate{role: parts[0], kind: parts[1], after: parts[2] != "before-write", entered: make(chan struct{}), resume: make(chan struct{})}
		if parts[2] == "after-receive" {
			controlGate.received = make(chan struct{})
		}
		t.Cleanup(controlGate.unblock)
	}
	if controlGate == nil && timing != "terminal-body" && timing != "" && timing != "before-request" && timing != "responder-parent" && timing != "source-parent" && timing != "source-stock-exhaustion" && timing != "publisher-source-parent" && timing != "response" && timing != "refresh-response" && timing != "refresh-pending" && timing != "publication-withdrawal" && timing != "request-terminal" && timing != "caller-observation" && timing != "close-observation" && timing != "publisher-observation" && timing != "publisher-close-observation" {
		t.Fatal("unknown Connection loss boundary")
	}
	pendingACK := timing == "refresh-pending"
	refresh := timing == "refresh-response" || pendingACK
	loss := controlGate != nil || timing == "terminal-body" || timing == "before-request" || timing == "response" || refresh || timing == "request-terminal"
	parentLoss := timing == "responder-parent"
	stockLoss := timing == "source-stock-exhaustion"
	sourceLoss := timing == "source-parent" || stockLoss
	publisherSourceLoss := timing == "publisher-source-parent"
	publisherObservation := timing == "publisher-observation" || timing == "publisher-close-observation"
	authorityLoss := timing == "caller-observation" || timing == "close-observation" || publisherObservation
	withdrawal := timing == "publication-withdrawal"
	expectedCancellation := authorityLoss || stockLoss || withdrawal
	recoveryStock := !stockLoss && (loss || parentLoss || sourceLoss || publisherSourceLoss)
	var expectedReaderClose error
	var expectedPublisherClose error
	f, reservations, certificates := installedPublisherNetwork(t, carrier, sourceLoss || publisherSourceLoss)
	f.receiverReopen = reopen
	var bodyEntered <-chan struct{}
	var releaseBody func()
	var observeBody func()
	if timing == "terminal-body" {
		observe, release, entered := installedPublisherTerminalBodyDelay()
		releaseBody, bodyEntered = release, entered
		observeBody = observe
		t.Cleanup(release)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 75*time.Second)
	defer cancel()
	var observationDelay func()
	var armObservation, releaseObservation func()
	var observationEntered <-chan struct{}
	if authorityLoss {
		observationDelay, armObservation, releaseObservation, observationEntered = installedConnectionObservationDelay()
		t.Cleanup(releaseObservation)
	}
	plan, target := installedPublisherInstance(t, f.profile.NetworkID)
	storeRoot := t.TempDir()
	descriptorGate := &publicationACKGate{root: storeRoot}
	defer descriptorGate.release()
	descriptorCurrent := f.current
	if pendingACK {
		descriptorCurrent = func() (network.RuntimeView, error) { return descriptorGate.observe(f.current) }
	}
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: storeRoot, Network: f.profile.NetworkID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	var originalJoin *receiver.Receiver
	var originalJoinConfig receiver.ReceiverConfig
	var injectedJoinFailure error
	var injectedJoinMu sync.Mutex
	responderParents := make(map[*receiver.Receiver]receiver.ReceiverConfig)
	parentFailures := make(map[*receiver.Receiver]error)
	sourceParents := make(map[*receiver.Receiver]receiver.ReceiverConfig)
	servers := installedPublisherReceiversObserved(t, f, reservations, certificates, store, func(err error) bool {
		// Only the exact original listener's retained injected failure is
		// expected. A different receiver or later cleanup failure still fails.
		injectedJoinMu.Lock()
		defer injectedJoinMu.Unlock()
		if loss && injectedJoinFailure != nil && err == injectedJoinFailure {
			return true
		}
		for _, failure := range parentFailures {
			if (parentLoss || sourceLoss || publisherSourceLoss || timing == "terminal-body") && failure != nil && err == failure {
				return true
			}
		}
		return false
	}, func(id [32]byte, config receiver.ReceiverConfig, server *receiver.Receiver) {
		if id == [32]byte{41} {
			originalJoin, originalJoinConfig = server, config
		}
		if parentLoss && (id == [32]byte{33} || id == [32]byte{34}) {
			responderParents[server] = config
		}
		if sourceLoss && (id == [32]byte{18} || id == [32]byte{19}) {
			sourceParents[server] = config
		}
		if publisherSourceLoss && (id == [32]byte{14} || id == [32]byte{15}) {
			sourceParents[server] = config
		}
	}, descriptorCurrent)
	local, err := executionruntime.New(execution.Config{ID: [32]byte{101}, Grants: []execution.Grant{
		{Principal: [32]byte{102}, Surface: execution.Administration}, {Principal: [32]byte{103}, Surface: execution.Connection}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := local.Close(); err != nil && !retainInstalledConnectionCancellation(t, err, expectedCancellation) {
			t.Error(err)
		}
	})
	snapshot := strings.Repeat("bounded recovered Text response\n", 16000)
	publisherCaller := ctx
	cancelPublisher := func() {}
	if publisherObservation || withdrawal {
		publisherCaller, cancelPublisher = context.WithCancel(ctx)
		defer cancelPublisher()
	}
	publisherWorker, err := local.LaunchPublisher(publisherCaller, [32]byte{102}, []byte(snapshot))
	if err != nil {
		t.Fatal(err)
	}
	publisherOperation, err := publisherWorker.BeginOperation(publisherCaller)
	if err != nil {
		t.Fatal(errors.Join(err, publisherWorker.Close()))
	}
	t.Cleanup(func() {
		publisherOperation.Close()
		if err := publisherWorker.Close(); err != nil {
			if publisherObservation && errors.Is(err, context.Canceled) {
				// Its unused protocol use retains the original Job's local
				// refusal. Actual physical cleanup is required separately below.
				t.Log("retained original unused Publisher cancellation", err)
			} else if !retainInstalledConnectionCancellation(t, err, expectedCancellation) {
				t.Error(err)
			}
		}
		if expectedCancellation {
			if physical, joined := publisherWorker.CleanupResult(); !joined || physical != nil || publisherWorker.CompletedCurrent() {
				t.Error("retired original Publisher did not physically join without Application completion", physical, joined)
			}
		}
	})
	root := t.TempDir()
	end := time.Now().Add(65 * time.Second).UTC().Truncate(time.Second)
	routePlan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "introduction"),
		SourceInteriorRoot: filepath.Join(root, "source"), ResponderInteriorRoot: filepath.Join(root, "responder"), HostingRoot: routeProcessBudget(t),
		Domain: 4, Deadline: end, Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	publisherAuthority := f.authority
	if controlGate != nil && controlGate.role == "publisher" {
		original := publisherAuthority.current
		publisherAuthority.current = func() (network.RuntimeView, error) { controlGate.observe(); return original() }
	}
	if controlGate != nil && controlGate.role == "initiator" && controlGate.received != nil {
		original := publisherAuthority.current
		publisherAuthority.current = func() (network.RuntimeView, error) { controlGate.observeReceived(); return original() }
	}
	if timing == "terminal-body" {
		original := publisherAuthority.current
		publisherAuthority.current = func() (network.RuntimeView, error) { observeBody(); return original() }
	}
	if sourceLoss || publisherSourceLoss {
		// Original independently selected Source legs are disjoint so this
		// actual receiver failure affects only the Reader's physical parent.
		routePlan.Exclusions = []route.Member{{NodeID: [32]byte{18}}, {NodeID: [32]byte{19}}}
	}
	if publisherObservation {
		current := publisherAuthority.current
		publisherAuthority.current = func() (network.RuntimeView, error) {
			view, err := current()
			if err == nil {
				observationDelay()
			}
			return view, err
		}
	}
	published, err := startPublicationHolder(publisherOperation.Context(), plan, routePlan, publisherAuthority, installedConnectionPublisherStock(t, f, parentLoss, publisherSourceLoss, recoveryStock), publisherOperation)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := published.close(); err != nil && !onlyInstalledConnectionExpectedFailure(err, expectedPublisherClose) && !retainInstalledConnectionCancellation(t, err, expectedCancellation) {
			t.Error(err)
		}
	})
	link, err := published.publish(ctx)
	if err != nil {
		t.Fatal(err)
	}
	publisher, ok := published.owner.(*publicationruntime.Publisher)
	if !ok {
		t.Fatal("genuine Publisher is absent")
	}
	readerCaller := ctx
	cancelReader := func() {}
	if expectedCancellation {
		readerCaller, cancelReader = context.WithCancel(ctx)
		defer cancelReader()
	}
	readerWorker, err := local.LaunchReader(readerCaller, [32]byte{103})
	if err != nil {
		t.Fatal(err)
	}
	readerOperation, err := readerWorker.BeginOperation(readerCaller)
	if err != nil {
		t.Fatal(errors.Join(err, readerWorker.Close()))
	}
	t.Cleanup(func() {
		readerOperation.Close()
		if err := readerWorker.Close(); err != nil {
			if expectedCancellation && errors.Is(err, context.Canceled) {
				// An unused qualified Reader retains its canceled protocol use,
				// including the retired Job's local refusal. Actual physical
				// failure is checked separately below, never ignored here.
				t.Log("retained original unused Reader cancellation", err)
			} else {
				t.Error(err)
			}
		}
		if expectedCancellation {
			if physical, joined := readerWorker.CleanupResult(); !joined || physical != nil || readerWorker.CompletedCurrent() {
				t.Error("canceled original Reader did not join without successful Application completion", physical, joined)
			}
		}
	})
	readerObservation := observationDelay
	if controlGate != nil && controlGate.role == "initiator" {
		readerObservation = controlGate.observe
	}
	if controlGate != nil && controlGate.role == "publisher" && controlGate.received != nil {
		readerObservation = controlGate.observeReceived
	}
	if publisherObservation {
		readerObservation = nil
	}
	source := installedConnectionInitiatorObserved(t, f, readerOperation, link, end.Add(-5*time.Second), readerObservation, installedReaderRecoveryStock{
		expectCancellation: expectedCancellation, loseSource: sourceLoss, separateSource: sourceLoss || publisherSourceLoss,
		reserveSource: recoveryStock, exhausted: stockLoss, expectedClose: &expectedReaderClose})
	publisherContract, err := publisherTextContract(publisherOperation)
	if err != nil {
		t.Fatal(err)
	}
	readerContract, err := readerOperation.BindStreamContract(512, (4<<20)+13)
	if err != nil {
		t.Fatal(err)
	}
	type accepted struct {
		stream *connection.Stream
		err    error
	}
	incoming := make(chan accepted, 1)
	go func() { stream, err := publisher.NextStream(ctx, publisherContract); incoming <- accepted{stream, err} }()
	waited := false
	t.Cleanup(func() {
		cancel()
		if !waited {
			peer := <-incoming
			if peer.stream != nil {
				_ = peer.stream.Close()
			}
		}
	})
	stream, err := source.OpenStream(readerContract)
	if err != nil {
		t.Fatal("Reader genuine composed stream", err)
	}
	t.Cleanup(func() {
		if err := stream.Close(); err != nil && !onlyInstalledConnectionExpectedFailure(err, expectedReaderClose) && !retainInstalledConnectionCancellation(t, err, expectedCancellation) {
			t.Error(err)
		}
	})
	peer := <-incoming
	waited = true
	if peer.err != nil || peer.stream == nil {
		t.Fatal("Publisher genuine composed stream", peer.err)
	}
	if controlGate != nil {
		selected := peer.stream
		if controlGate.role == "initiator" {
			selected = stream
		}
		controlGate.arm(selected)
		if controlGate.received != nil {
			controlGate.mu.Lock()
			controlGate.receiver = stream
			if controlGate.role == "initiator" {
				controlGate.receiver = peer.stream
			}
			controlGate.mu.Unlock()
		}
	}
	t.Cleanup(func() {
		if err := peer.stream.Close(); err != nil {
			if onlyInstalledConnectionExpectedFailure(err, expectedPublisherClose) && peer.stream.JoinedCleanup() == nil {
				t.Log("retained original withdrawn Publisher outcome", err)
			} else if expectedCancellation && errors.Is(err, context.Canceled) && peer.stream.JoinedCleanup() == nil {
				// Original Publisher revocation can retain the retired Job's
				// local refusal in its full work outcome. Its actual joined
				// physical receipt and both worker receipts still must be clean.
				t.Log("retained original Publisher Stream cancellation", err)
			} else if !retainInstalledConnectionCancellation(t, err, expectedCancellation) {
				t.Error(err)
			}
		}
	})
	if authorityLoss {
		if publisherObservation {
			assertInstalledConnectionDelayedObservationRetirement(t, ctx, cancelPublisher, publisherOperation.CheckPublisher, peer.stream, stream, armObservation, releaseObservation, observationEntered, timing == "publisher-close-observation")
		} else {
			assertInstalledConnectionDelayedObservationRetirement(t, ctx, cancelReader, readerOperation.CheckReader, stream, peer.stream, armObservation, releaseObservation, observationEntered, timing == "close-observation")
		}
		return
	}
	if withdrawal {
		assertInstalledConnectionPublicationWithdrawal(t, ctx, publisher, publisherOperation, readerOperation, peer.stream, stream)
		expectedPublisherClose = peer.stream.Close()
		expectedReaderClose = stream.Close()
		cancelPublisher()
		cancelReader()
		return
	}
	if refresh {
		firstRaw, firstStatus, refreshErr := store.Lookup(target, f.profile.Digest, time.Now())
		if refreshErr != nil || firstStatus != reachability.Accepted {
			t.Fatal("original Descriptor unavailable before refresh", refreshErr)
		}
		firstProof, refreshErr := reachability.Verify(firstRaw, target, f.profile.NetworkID, f.profile.Digest, time.Now())
		if refreshErr != nil {
			t.Fatal(refreshErr)
		}
		if pendingACK {
			descriptorGate.arm(t)
			refreshed := make(chan error, 1)
			go func() {
				got, err := publisher.Refresh(ctx)
				if err == nil && got != link {
					err = errors.New("pending refresh changed Link")
				}
				refreshed <- err
			}()
			joined := false
			defer func() {
				descriptorGate.release()
				if !joined {
					<-refreshed
				}
			}()
			select {
			case <-descriptorGate.entered:
			case err := <-refreshed:
				joined = true
				t.Fatal("refresh finished before actual pending ACK", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			raw, err := descriptorGate.record()
			if err != nil || len(raw) < 2 {
				t.Fatal("actual pending signed Descriptor absent", err)
			}
			candidate, err := reachability.Verify(raw[2:], target, f.profile.NetworkID, f.profile.Digest, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if candidate.Introduction.Revision != firstProof.Introduction.Revision+1 || candidate.Introduction.RecipientKey == firstProof.Introduction.RecipientKey {
				t.Fatal("pending recipient did not advance")
			}
			installedPublisherSubmitNonce(t, f, candidate, false)
			if n, err := stream.Write(nil); n != 0 || err != nil {
				t.Fatal("pending ACK retired original Reader stream", n, err)
			}
			if n, err := peer.stream.Write(nil); n != 0 || err != nil {
				t.Fatal("pending ACK retired original Publisher stream", n, err)
			}
			select {
			case err := <-refreshed:
				joined = true
				t.Fatal("new recipient refusal released held ACK", err)
			default:
			}
			descriptorGate.release()
			select {
			case err := <-refreshed:
				joined = true
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			t.Log("pending actual Descriptor ACK refused new recipient and preserved original Connection")
		} else if refreshed, refreshErr := publisher.Refresh(ctx); refreshErr != nil || refreshed != link {
			t.Fatal("same Publication refresh failed", refreshErr)
		}
		secondRaw, secondStatus, refreshErr := store.Lookup(target, f.profile.Digest, time.Now())
		if refreshErr != nil || secondStatus != reachability.Accepted {
			t.Fatal("refreshed Descriptor lacks genuine Store ACK", refreshErr)
		}
		secondProof, refreshErr := reachability.Verify(secondRaw, target, f.profile.NetworkID, f.profile.Digest, time.Now())
		if refreshErr != nil || secondProof.PublicationDigest != firstProof.PublicationDigest || secondProof.ProfileDigest != firstProof.ProfileDigest || secondProof.Target != firstProof.Target || secondProof.Introduction.Revision != firstProof.Introduction.Revision+1 || secondProof.Introduction.Slot == firstProof.Introduction.Slot || secondProof.Introduction.RecipientKey == firstProof.Introduction.RecipientKey {
			t.Fatal("refresh changed immutable Publication or failed to advance recipient", refreshErr)
		}
		t.Log("actual accepting Publication refreshed before original Connection response loss")
	}
	loseOriginal := func() {
		// Stop only the exact actual Rendezvous receiving transport. Original
		// operation, worker, Source and receiving spends/root remain unchanged.
		if originalJoin == nil {
			t.Fatal("original Rendezvous receiver is absent")
		}
		if err := originalJoin.Close(); err != nil {
			injectedJoinMu.Lock()
			injectedJoinFailure = err
			injectedJoinMu.Unlock()
			t.Log("retained original Rendezvous loss", err)
		}
		replacement, err := receiver.Listen(t.Context(), originalJoinConfig)
		if err != nil {
			t.Fatal("same-duty genuine Rendezvous restart", err)
		}
		t.Cleanup(func() {
			if err := replacement.Close(); err != nil {
				t.Error(err)
			}
		})
		// The original retained pointer remains a historical failed actor.
		// The replacement listener does not replace either Connection owner.
	}
	if timing == "before-request" {
		loseOriginal()
	}
	if parentLoss {
		if len(responderParents) != 2 {
			t.Fatal("original Responder Interior candidates are absent")
		}
		// Stop both genuine role-3 Interior candidates after initial Service
		// authentication. Exactly one carries the original selected parent;
		// no private Prefix handle or synthetic retirement is supplied here.
		for original, config := range responderParents {
			failure := original.Close()
			injectedJoinMu.Lock()
			parentFailures[original] = failure
			injectedJoinMu.Unlock()
			if failure != nil {
				t.Log("retained original Responder receiver loss", failure)
			}
			replacement, err := receiver.Listen(t.Context(), config)
			if err != nil {
				t.Fatal("same-duty genuine Responder restart", err)
			}
			t.Cleanup(func() {
				if err := replacement.Close(); err != nil {
					t.Error(err)
				}
			})
		}
	}
	if sourceLoss || publisherSourceLoss {
		if len(sourceParents) != 2 {
			t.Fatal("original Reader Source receivers absent")
		}
		for originalSource, originalSourceConfig := range sourceParents {
			failure := originalSource.Close()
			injectedJoinMu.Lock()
			parentFailures[originalSource] = failure
			injectedJoinMu.Unlock()
			if failure != nil {
				t.Log("retained original Reader Source receiver loss", failure)
			}
			replacement, err := receiver.Listen(t.Context(), originalSourceConfig)
			if err != nil {
				t.Fatal("same-duty genuine Reader Source restart", err)
			}
			t.Cleanup(func() {
				if err := replacement.Close(); err != nil {
					t.Error(err)
				}
			})
		}
	}
	if stockLoss {
		select {
		case <-stream.Context().Done():
		case <-ctx.Done():
			t.Fatal("underfunded original Source never refused recovery", ctx.Err())
		}
		body := []byte{0x5a}
		n, refusal := stream.Read(body)
		if n != 0 || body[0] != 0x5a || stock.TransferFailureStage(refusal) != "stock" {
			t.Fatal("Source exhaustion did not refuse before Application delivery", n, refusal)
		}
		if n, err := stream.Write([]byte("refused request")); n != 0 || stock.TransferFailureStage(err) != "stock" {
			t.Fatal("exhausted Connection accepted another Application effect", n, err)
		}
		expectedReaderClose = stream.Close()
		if stock.TransferFailureStage(expectedReaderClose) != "stock" || stream.Close() != expectedReaderClose || stream.JoinedCleanup() != nil {
			t.Fatal("underfunded Connection lost its original refusal or physical join", expectedReaderClose, stream.JoinedCleanup())
		}
		if err := errors.Join(readerOperation.CheckReader(), publisherOperation.CheckPublisher()); err != nil {
			t.Fatal("Stock failure replaced or retired an original Job", err)
		}
		cancel()
		return
	}
	publisherOutcome := observeConnectionOutcome(peer.stream)
	deliverPublisher := func() (bool, error) {
		return publisherOperation.DeliverPublisherStream(ctx, publisherContract, publisherOutcome)
	}
	var terminalObserved *observedConnectionEOF
	if timing == "request-terminal" {
		observed := &observedConnectionEOF{Stream: peer.stream, outcome: publisherOutcome, ctx: ctx, arrived: make(chan struct{}), resume: make(chan struct{})}
		t.Cleanup(observed.release)
		terminalObserved = observed
		deliverPublisher = func() (bool, error) {
			return publisherOperation.DeliverPublisherStream(ctx, publisherContract, observed)
		}
	}
	if transferred, err := deliverPublisher(); err != nil || !transferred {
		t.Fatal("original Publisher Text transfer", err)
	}
	var transferred bool
	var result *executionruntime.ReaderResult
	if timing == "response" || refresh {
		transferred, result, err = deliverReaderAfterResponseLoss(t, ctx, cancel, readerOperation, readerContract, stream, len(snapshot)+13, loseOriginal)
	} else if timing == "terminal-body" || controlGate != nil {
		type readerReturn struct {
			transferred bool
			result      *executionruntime.ReaderResult
			err         error
		}
		done := make(chan readerReturn, 1)
		go func() {
			transferred, result, err := readerOperation.DeliverReaderStream(readerContract, stream)
			done <- readerReturn{transferred, result, err}
		}()
		joined := false
		releaseHeld := releaseBody
		entered := bodyEntered
		if controlGate != nil {
			releaseHeld, entered = controlGate.unblock, controlGate.entered
		}
		defer func() {
			releaseHeld()
			if !joined {
				cancel()
				<-done
			}
		}()
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("original complete Terminal body observation absent", ctx.Err())
		}
		if controlGate != nil && controlGate.received != nil {
			select {
			case <-controlGate.received:
			case <-ctx.Done():
				t.Fatal("opposite original reader did not accept control before writer completion", ctx.Err())
			}
		}
		loseOriginal()
		releaseHeld()
		select {
		case got := <-done:
			joined = true
			transferred, result, err = got.transferred, got.result, got.err
		case <-ctx.Done():
			t.Fatal("original Terminal-body recovery incomplete", ctx.Err())
		}
		if controlGate != nil {
			t.Log("actual original control ticket delayed at write boundary; same Rendezvous joined and restarted", controlGate.role, controlGate.kind, controlGate.after, "peer accepted before finish", controlGate.received != nil)
		} else {
			t.Log("actual Publisher received second request-side record body before final observation; same Rendezvous joined and restarted")
		}
	} else if timing == "request-terminal" {
		transferred, result, err = deliverReaderAfterRequestTerminalLoss(t, ctx, cancel, readerOperation, readerContract, stream, terminalObserved, loseOriginal)
	} else {
		transferred, result, err = readerOperation.DeliverReaderStream(readerContract, stream)
	}
	if err != nil || !transferred || result == nil {
		t.Fatal("one original Text request", err)
	}
	defer result.Close()
	if err := readerOperation.CheckReader(); err != nil {
		t.Fatal("original Reader retired before Source join", err)
	}
	if body, err := result.TakeBytes(); err == nil || body != nil {
		clear(body)
		t.Fatal("Reader exposed result before original worker join")
	}
	// The final confirmation can still be in flight after Reader settlement.
	// Keep its original tail until the genuine Publisher has independently
	// published its own result; local Reader success is not that peer proof.
	if err := publisherOutcome.waitClean(ctx); err != nil {
		t.Fatal("original Publisher did not independently complete Terminal controls", err)
	}
	if err := source.Close(); err != nil {
		t.Fatal("original Reader Source join", err)
	}
	readerOperation.Close()
	if err := readerWorker.Close(); err != nil {
		t.Fatal("original Reader worker join", err)
	}
	body, err := result.TakeBytes()
	if err != nil || string(body) != snapshot {
		t.Fatal("original joined Text result differs", err)
	}
	clear(body)
	if err := published.close(); err != nil {
		t.Fatal("original Publisher Connection/private retirement", err)
	}
	publisherOperation.Close()
	if err := publisherWorker.Close(); err != nil {
		t.Fatal("original Publisher worker join", err)
	}
	for _, server := range servers {
		if _, stopped := sourceParents[server]; stopped {
			continue
		}
		if server == originalJoin && loss {
			continue
		}
		if _, stopped := responderParents[server]; stopped {
			continue // Exact original Close result was already retained above.
		}
		if err := server.Close(); err != nil {
			if timing == "terminal-body" && transport.IsPeerRetirementCause(err) {
				if server.Close() != err {
					t.Fatal("original receiving physical interruption lost on repeat", err)
				}
				injectedJoinMu.Lock()
				parentFailures[server] = err
				injectedJoinMu.Unlock()
				t.Log("retained actual original receiving peer retirement after Terminal-body interruption", err)
			} else {
				t.Fatal("original receiving join", err)
			}
		}
	}
	if reopen {
		verifyInstalledConnectionReopen(t, f, store, storeRoot, plan, target)
	}
}

func installedConnectionPublisherStock(t *testing.T, f *networkAdmissionFixture, parentLoss ...bool) *stock.Owner {
	prepareSource := len(parentLoss) >= 2 && parentLoss[1] || len(parentLoss) >= 3 && parentLoss[2]
	prepareParent := len(parentLoss) >= 1 && parentLoss[0] || prepareSource
	// A failed started TLS write can retire a physical parent even when the
	// injected loss names only JOIN. Provision its finite replacement before
	// signing; no rights are added after an actual failure or presentation.
	forwards := uint32(14)
	if prepareParent {
		forwards += 4
	}
	if prepareSource {
		forwards += 4
	}
	// Declare the finite scenario allocation BEFORE independent signing.
	// Production Publisher quotas and recovery proposals remain unchanged.
	holder := routePermissionStock(t, f, admission.AllocationPublisher, [3]uint32{2, forwards, 2})
	var attempt stock.Attempt
	installedPublisherBatches(t, f, func(command any) localAdmissionReply {
		request := command.(holderCommand)
		var result localAdmissionReply
		var err error
		if request.Operation == "begin" {
			if prepareSource && len(request.Intent.Challenges) >= 4 && request.Intent.Challenges[0].ReceiverNodeID == [32]byte{12} {
				request.Intent.Challenges = append(request.Intent.Challenges, request.Intent.Challenges[:4]...)
			}
			// Initial Service JOIN adds one real Forward presentation to the
			// existing Responder-prefix batch. Declare it before issuance;
			// no retry, refund or permission enlargement follows a failure.
			if len(request.Intent.Challenges) != 0 && request.Intent.Challenges[0].ReceiverNodeID == [32]byte{31} {
				if prepareParent {
					request.Intent.Challenges = append(request.Intent.Challenges, request.Intent.Challenges...)
				}
				extra := request.Intent.Challenges[0]
				extra.ReceiverNodeID = [32]byte{41}
				request.Intent.Challenges = append(append([]token.ClosedTokenContext(nil), request.Intent.Challenges...), extra)
				request.Intent.Challenges = append(request.Intent.Challenges, extra)
			}
			attempt, err = holder.Begin(request.Intent)
			if err == nil {
				result.Request, _, err = attempt.Request()
			}
		} else {
			err = attempt.Complete(request.Payload, nil)
		}
		if err == nil {
			result.Outcome = "completed"
		}
		return result
	})
	return holder
}

func installedConnectionInitiator(t *testing.T, f *networkAdmissionFixture, operation *executionruntime.Operation, link string, end time.Time) *connection.InitiatorSource {
	return installedConnectionInitiatorObserved(t, f, operation, link, end, nil, installedReaderRecoveryStock{})
}

type installedReaderRecoveryStock struct {
	expectCancellation bool
	loseSource         bool
	separateSource     bool
	reserveSource      bool
	exhausted          bool
	expectedClose      *error
}

func installedConnectionInitiatorObserved(t *testing.T, f *networkAdmissionFixture, operation *executionruntime.Operation, link string, end time.Time, afterObservation func(), recovery installedReaderRecoveryStock) *connection.InitiatorSource {
	sourceLoss, separateSource := recovery.loseSource, recovery.separateSource
	reserveSource := !recovery.exhausted && (sourceLoss || recovery.reserveSource)
	var physicalPreparations atomic.Int32
	forwardLimit := uint32(6)
	var exclusions []route.Member
	if reserveSource {
		forwardLimit += 4
	}
	if separateSource {
		exclusions = []route.Member{{NodeID: [32]byte{14}}, {NodeID: [32]byte{15}}}
	}
	holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{4, forwardLimit, 0})
	var forwards, controls []token.ClosedTokenContext
	challenge := func(id byte, class uint8) token.ClosedTokenContext {
		return token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest,
			IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: [32]byte{id}, ReceiverDutyGeneration: 9,
			Class: class, WindowStart: time.Now().UTC().Truncate(time.Hour)}
	}
	forwardNodes := []byte{12, 13, 14, 15, 41, 41}
	if separateSource {
		forwardNodes = []byte{12, 13, 18, 19, 41, 41}
	}
	for _, id := range forwardNodes {
		forwards = append(forwards, challenge(id, 2))
	}
	if reserveSource {
		for _, id := range forwardNodes[:4] {
			forwards = append(forwards, challenge(id, 2))
		}
	}
	for range 2 {
		controls = append(controls, challenge(17, 1), challenge(25, 1))
	}
	for i, challenges := range [][]token.ClosedTokenContext{forwards, controls} {
		attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Bootstrap: true, Deadline: f.profile.NotAfter,
			Selection: stock.ExchangeBinding{ID: [32]byte{byte(91 + i)}, ProfileDigest: f.profile.Digest}})
		if err != nil {
			t.Fatal(err)
		}
		raw, _, err := attempt.Request()
		if err != nil {
			t.Fatal(err)
		}
		issued := issuer.IssueCurrent(t.Context(), f.plan, raw, quota.Bootstrap, func() (admission.AuthorityFacts, time.Time, error) {
			return f.authority.issuer(f.plan.KeyBinding.Signer)
		})
		if issued.Outcome != "issued-offline" {
			t.Fatal("finite Reader fixture issuance refused", issued.Outcome)
		}
		if err := attempt.Complete(issued.Response, nil); err != nil {
			t.Fatal(err)
		}
	}
	root := t.TempDir()
	installation, err := selection.OpenInstallation(selection.InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: f.current, Exclusions: exclusions})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := installation.Close(); err != nil {
			t.Error(err)
		}
	})
	selected, err := installation.Borrow(selection.RoleConfig{InteriorRoot: filepath.Join(root, "source"), Domain: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := selected.Close(); err != nil {
			t.Error(err)
		}
	})
	budget, err := hosting.Open(routeProcessBudget(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := budget.Close(); err != nil {
			t.Error(err)
		}
	})
	routePlan := routePrefixPlan{Deadline: end, Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, Exclusions: exclusions}
	authority := f.authority
	if afterObservation != nil {
		current := authority.current
		authority.current = func() (network.RuntimeView, error) {
			view, err := current()
			if err == nil {
				afterObservation()
			}
			return view, err
		}
	}
	physical, leg, err := openPublicationPrefix(operation.Context(), routePlan, selected, budget, authority, holder)
	if err != nil {
		t.Fatal(err)
	}
	choice, err := selection.NewRendezvous(leg, f.current, exclusions)
	if err != nil {
		t.Fatal(errors.Join(err, physical.Close()))
	}
	history := &reachability.History{}
	t.Cleanup(func() {
		if err := history.Close(); err != nil {
			t.Error(err)
		}
	})
	source, err := connection.NewInitiatorSource(connection.InitiatorSourceConfig{Operation: operation, Source: physical,
		PrepareSource: func(caller context.Context) (prefix.Config, error) {
			config, err := prepareSelectedPublicationPrefix(caller, routePlan, leg, budget, authority, holder)
			if err == nil {
				physicalPreparations.Add(1)
			}
			return config, err
		},
		History: history, Rendezvous: choice, TargetLink: link, Network: f.profile.NetworkID, Profile: f.profile.Digest,
		WorkSafetyNotAfter: end, WorkSafetyMaximum: end, NoNewRecoveryAfter: end.Add(-5 * time.Second), Exclusions: exclusions})
	if err != nil {
		t.Fatal(errors.Join(err, physical.Close()))
	}
	t.Cleanup(func() {
		expected := afterObservation != nil || recovery.expectCancellation
		var known error
		if recovery.expectedClose != nil {
			known = *recovery.expectedClose
		}
		if err := source.Close(); err != nil && !onlyInstalledConnectionExpectedFailure(err, known) && !retainInstalledConnectionCancellation(t, err, expected && operation.Context().Err() != nil) {
			t.Error(err)
		}
		if sourceLoss && physicalPreparations.Load() == 0 {
			t.Error("Source loss never prepared a fresh physical generation")
		}
	})
	return source
}
