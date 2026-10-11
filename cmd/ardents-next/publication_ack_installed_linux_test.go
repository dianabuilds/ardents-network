//go:build linux && text_worker_installed

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// The callback only delays observation after the actual committed file changes.
// Every concurrent observation waits, including the receiving authority watcher;
// none can pass a final guard and emit ACK while another observer holds the gate.
// Store.Lookup must not run here: Store.Publish retains its own mutex through
// its post-write authority check. Public physical bytes are an independent oracle.
type publicationACKGate struct {
	mu       sync.Mutex
	root     string
	baseline [32]byte
	armed    bool
	entered  chan struct{}
	resume   chan struct{}
	once     sync.Once
}

// Only receiving dispatch's actual CLOSE writer waits here. Its successful
// Publisher RESULT has already been decoded by the production reader. Other
// currentness observations, including the replacement registration, remain live.
type publicationDeliveryCloseGate struct {
	mu      sync.Mutex
	armed   bool
	entered chan struct{}
	resume  chan struct{}
	once    sync.Once
}

func (g *publicationDeliveryCloseGate) arm() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.armed, g.entered, g.resume, g.once = true, make(chan struct{}), make(chan struct{}), sync.Once{}
}

func (g *publicationDeliveryCloseGate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.armed {
		g.armed = false
		close(g.resume)
	}
}

func (g *publicationDeliveryCloseGate) observe(current func() (network.RuntimeView, error)) (network.RuntimeView, error) {
	var callers [32]uintptr
	frames := goruntime.CallersFrames(callers[:goruntime.Callers(2, callers[:])])
	for {
		frame, more := frames.Next()
		if frame.Function == "github.com/dianabuilds/ardents-network/internal/successor/route/introduction.(*registrationDispatch).write" {
			g.mu.Lock()
			var resume <-chan struct{}
			if g.armed {
				g.once.Do(func() { close(g.entered) })
				resume = g.resume
			}
			g.mu.Unlock()
			if resume != nil {
				<-resume
			}
			break
		}
		if !more {
			break
		}
	}
	return current()
}

// Keep the identity of actual State refusals returned to these consumers.
// This observer always calls the genuine State; it cannot supply cached facts
// or make a failure accepting. Only the named durable-profile refusal is
// recorded, so unknown physical/storage failures cannot pass cleanup checks.
type publicationACKNetworkFailures struct {
	mu     sync.Mutex
	causes []error
}

func (observed *publicationACKNetworkFailures) current(current func() (network.RuntimeView, error)) (network.RuntimeView, error) {
	view, err := current()
	if err != nil && err.Error() == "closed profile is unavailable" {
		observed.mu.Lock()
		if len(observed.causes) < 4096 {
			observed.causes = append(observed.causes, err)
		}
		observed.mu.Unlock()
	}
	return view, err
}

func (observed *publicationACKNetworkFailures) contains(err error) bool {
	observed.mu.Lock()
	defer observed.mu.Unlock()
	for _, cause := range observed.causes {
		if cause == err {
			return true
		}
	}
	return false
}

func (g *publicationACKGate) record() ([]byte, error) {
	entries, err := os.ReadDir(filepath.Join(g.root, "records"))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if len(entry.Name()) == 64 {
			return os.ReadFile(filepath.Join(g.root, "records", entry.Name()))
		}
	}
	return nil, nil
}

func (g *publicationACKGate) arm(t *testing.T) {
	t.Helper()
	raw, err := g.record()
	if err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.baseline = sha256.Sum256(raw)
	g.armed, g.entered, g.resume, g.once = true, make(chan struct{}), make(chan struct{}), sync.Once{}
}

func (g *publicationACKGate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.armed {
		g.armed = false
		close(g.resume)
	}
}

func (g *publicationACKGate) observe(current func() (network.RuntimeView, error)) (network.RuntimeView, error) {
	raw, err := g.record()
	if err != nil {
		return network.RuntimeView{}, err
	}
	g.mu.Lock()
	var resume <-chan struct{}
	if g.armed && len(raw) != 0 && sha256.Sum256(raw) != g.baseline {
		g.once.Do(func() { close(g.entered) })
		resume = g.resume
	}
	g.mu.Unlock()
	if resume != nil {
		<-resume
	}
	return current()
}

// Pause only the original Source's post-decode observation. The actual frame,
// nonce and status are decoded by production Route before entering this private
// termination responsibility; no cached RuntimeView or accepting callback is
// supplied. A retained sealed Source error independently proves successful ACK.
func (g *publicationACKGate) observeResult(current func() (network.RuntimeView, error)) (network.RuntimeView, error) {
	var callers [32]uintptr
	frames := goruntime.CallersFrames(callers[:goruntime.Callers(2, callers[:])])
	decoded := false
	for {
		frame, more := frames.Next()
		if frame.Function == "github.com/dianabuilds/ardents-network/internal/successor/route/prefix.finishDescriptorResponse" {
			decoded = true
			break
		}
		if !more {
			break
		}
	}
	if decoded {
		g.mu.Lock()
		var resume <-chan struct{}
		if g.armed {
			g.once.Do(func() { close(g.entered) })
			resume = g.resume
		}
		g.mu.Unlock()
		if resume != nil {
			<-resume
		}
	}
	return current()
}

// The production composition receives a genuine installed snapshot operation.
// Direct command-handle calls permit concurrent Link observations while Publish
// waits on real I/O; the serial JSON console cannot express that observation.
func runInstalledPublisherACK(t *testing.T, carrier transport.CarrierProfile, retirement string) {
	f, reservations, certificates := installedPublisherNetwork(t, carrier)
	deliveryGate := &publicationDeliveryCloseGate{}
	resultLost := false
	if retirement == "delivery-close" {
		original := f.current
		f.current = func() (network.RuntimeView, error) { return deliveryGate.observe(original) }
		f.authority = networkAdmissionAuthority(f.current, func() error { return nil })
		defer deliveryGate.release()
	}
	receivingNetwork := f
	networkRetirement := retirement == "network" || retirement == "decoded-network"
	sourceACKGate := &publicationACKGate{}
	observedNetwork := &publicationACKNetworkFailures{}
	if networkRetirement {
		receivingNetwork = publicationReceivingNetwork(t, f)
		current := f.current
		if retirement == "decoded-network" {
			original := current
			current = func() (network.RuntimeView, error) { return sourceACKGate.observeResult(original) }
		}
		f.current = func() (network.RuntimeView, error) { return observedNetwork.current(current) }
		f.authority = networkAdmissionAuthority(f.current, func() error { return nil })
	}
	operationLimit, routeLimit := 45*time.Second, 40*time.Second
	if retirement == "elapsed" {
		operationLimit, routeLimit = 460*time.Second, 450*time.Second
	}
	ctx, cancel := context.WithTimeout(t.Context(), operationLimit)
	defer cancel()
	publisherCaller, cancelPublisher := context.WithCancel(ctx)
	defer cancelPublisher()
	var operation *executionruntime.Operation
	workerLost := false
	networkLost := false
	checkCleanup := func(err error) bool {
		if err == nil {
			return true
		}
		if resultLost {
			return publicationDeliveryExpiryOnly(err)
		}
		originalLoss := retirement == "caller" && publisherCaller.Err() == context.Canceled ||
			retirement == "worker" && workerLost && operation != nil && operation.Context().Err() == context.Canceled
		if originalLoss {
			return publicationACKCancellationOnly(err)
		}
		return networkLost && publicationACKNetworkLossOnly(err, observedNetwork)
	}
	plan, target := installedPublisherInstance(t, f.profile.NetworkID)
	storeRoot := t.TempDir()
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: storeRoot, Network: f.profile.NetworkID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	gate := sourceACKGate
	gate.root = storeRoot
	checkReceiver := func(err error) bool {
		return (networkLost || resultLost) && checkCleanup(err) || (workerLost || networkLost) && transport.IsPeerRetirementCause(err)
	}
	servers := installedPublisherReceiversChecked(t, receivingNetwork, reservations, certificates, store, checkReceiver, func() (network.RuntimeView, error) {
		if retirement == "decoded-network" {
			return receivingNetwork.current()
		}
		return gate.observe(receivingNetwork.current)
	})
	defer gate.release()
	holder := routePermissionStock(t, f, admission.AllocationPublisher, [3]uint32{2, 12, 2})
	var attempt stock.Attempt
	installedPublisherBatches(t, f, func(command any) localAdmissionReply {
		request := command.(holderCommand)
		var result localAdmissionReply
		var err error
		if request.Operation == "begin" {
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
	principal := [32]byte{32}
	local, err := executionruntime.New(execution.Config{ID: [32]byte{31}, Grants: []execution.Grant{{Principal: principal, Surface: execution.Administration}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := local.Close(); !checkCleanup(err) {
			t.Error(err)
		}
	})
	invocation, err := local.LaunchPublisher(ctx, principal, []byte("genuine delayed Descriptor ACK snapshot\n"))
	if err != nil {
		t.Fatal(err)
	}
	operation, err = invocation.BeginOperation(ctx)
	if err != nil {
		t.Fatal(errors.Join(err, invocation.Close()))
	}
	checkInvocation := func(err error) bool {
		if retirement != "worker" || !workerLost {
			return checkCleanup(err)
		}
		physical, completed := invocation.CleanupResult()
		return err != nil && completed && checkCleanup(physical) && !invocation.CompletedCurrent()
	}
	t.Cleanup(func() {
		operation.Close()
		if err := invocation.Close(); !checkInvocation(err) {
			t.Error(err)
		}
	})
	root := t.TempDir()
	routePlan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "introduction"), SourceInteriorRoot: filepath.Join(root, "source"), ResponderInteriorRoot: filepath.Join(root, "responder"), HostingRoot: routeProcessBudget(t), Domain: 4, Deadline: time.Now().Add(routeLimit).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	owner, err := startPublicationHolder(publisherCaller, plan, routePlan, networkAdmissionAuthority(f.current, func() error { return nil }), holder, operation)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		gate.release()
		if err := owner.close(); !checkCleanup(err) {
			t.Error(err)
		}
	})
	expected, err := publication.TargetLink(f.profile.NetworkID, target)
	if err != nil {
		t.Fatal(err)
	}
	if retirement == "elapsed" {
		runInstalledPublisherElapsedPairs(t, ctx, f, owner, gate, store, target, expected)
		if err := owner.withdraw(ctx); err != nil {
			t.Fatal("elapsed Publisher withdrawal", err)
		}
		if err := owner.close(); err != nil {
			t.Fatal("elapsed Publisher physical join", err)
		}
		operation.Close()
		if err := invocation.Close(); err != nil || !invocation.CompletedCurrent() {
			t.Fatal("elapsed original snapshot worker join", err)
		}
		for _, server := range servers {
			if err := server.Close(); err != nil {
				t.Fatal("elapsed receiving physical join", err)
			}
		}
		return
	}
	var previous reachability.Descriptor
	var predecessor reachability.Descriptor
	var acceptedNonce [32]byte
	originalContext, err := operation.AdministrationContext()
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"initial", "replacement"} {
		t.Run(phase, func(t *testing.T) {
			gate.arm(t)
			defer gate.release()
			finished := make(chan error, 1)
			go func() {
				call := owner.publish
				if phase == "replacement" {
					call = owner.refresh
				}
				link, err := call(ctx)
				if err == nil && link != expected {
					err = errors.New("actual ACK returned foreign Link")
				}
				finished <- err
			}()
			joined := false
			defer func() {
				gate.release()
				if !joined {
					<-finished
				}
			}()
			select {
			case <-gate.entered:
			case err := <-finished:
				joined = true
				t.Fatal("publication completed before committed-record gate", err)
			case <-ctx.Done():
				t.Fatal("actual Store commit not observed", ctx.Err())
			}
			raw, err := gate.record()
			if err != nil || len(raw) < 2 {
				t.Fatal("committed public record absent", err)
			}
			proof, err := reachability.Verify(raw[2:], target, f.profile.NetworkID, f.profile.Digest, time.Now())
			if err != nil {
				t.Fatal("actual committed Descriptor invalid", err)
			}
			link, err := owner.link(ctx)
			if phase == "initial" {
				if err == nil || link != "" {
					t.Fatal("unacknowledged initial pair supplied Link")
				}
			} else {
				if err != nil || link != expected {
					t.Fatal("pending replacement lost original Link", err)
				}
				if proof.Introduction.Revision != previous.Introduction.Revision+1 || proof.Introduction.Slot == previous.Introduction.Slot || proof.Introduction.RecipientKey == previous.Introduction.RecipientKey || proof.PublicationDigest != previous.PublicationDigest {
					t.Fatal("replacement lacks independent slot/key or exact Publication")
				}
			}
			select {
			case err := <-finished:
				joined = true
				t.Fatal("held Store response completed", err)
			default:
			}
			retiring := retirement != "" && retirement != "delivery-close" && phase == "replacement"
			if retiring {
				if retirement == "caller" {
					cancelPublisher()
				} else if networkRetirement {
					if publisherCaller.Err() != nil || operation.CheckPublisher() != nil {
						t.Fatal("original local authority ended before signed Network conflict")
					}
					if err := f.profileConflict(t); err == nil || err.Error() != "closed profile has a durable conflict" {
						t.Fatal("signed conflicting profile did not establish durable refusal", err)
					}
					if _, err := f.current(); err == nil || err.Error() != "closed profile is unavailable" {
						t.Fatal("genuine State remained current after durable profile conflict", err)
					}
					if publisherCaller.Err() != nil {
						t.Fatal("caller ended before signed Network-loss observation")
					}
					networkLost = true
					if view, err := receivingNetwork.current(); err != nil || view.Profile().Digest != f.profile.Digest {
						t.Fatal("Publisher conflict revoked independent receiving authority", err)
					}
					if _, completed := invocation.CleanupResult(); completed {
						t.Fatal("Network loss bypassed original operation join")
					}
				} else if retirement == "worker" {
					stopInstalledExecutionWorker(t, ctx, "publisher")
					select {
					case <-operation.Context().Done():
					case <-ctx.Done():
						t.Fatal("original worker loss retained live Publisher operation")
					}
					if publisherCaller.Err() != nil || operation.CheckPublisher() == nil {
						t.Fatal("worker loss did not revoke the original operation independently of caller")
					}
					if _, err := f.current(); err != nil {
						t.Fatal("Network authority lost before worker-loss observation", err)
					}
					workerLost = true
					if _, completed := invocation.CleanupResult(); completed {
						t.Fatal("worker loss bypassed original operation join")
					}
				} else if err := owner.withdraw(ctx); err != nil {
					t.Fatal("actual pending withdrawal did not join", err)
				}
				if link, err := owner.link(ctx); err == nil || link != "" {
					t.Fatal("retirement left accepting Link before held response release")
				}
			}
			gate.release()
			select {
			case err := <-finished:
				joined = true
				if (err == nil) == retiring {
					t.Fatal("actual ACK/retirement result disagreed", err)
				}
				if retiring && retirement == "decoded-network" {
					var acknowledged *prefix.DescriptorAcknowledgedFailure
					if !errors.As(err, &acknowledged) || acknowledged.Unwrap() == nil {
						t.Fatal("Network-loss refusal lacks original decoded successful Store ACK", err)
					}
				}
			case <-ctx.Done():
				t.Fatal("ACK exchange did not join", ctx.Err())
			}
			accepted, outcome, err := store.Lookup(target, f.profile.Digest, time.Now())
			if err != nil || outcome != reachability.Accepted || !bytes.Equal(accepted, raw[2:]) {
				t.Fatal("ACK differs from actual Store record", err)
			}
			if networkLost {
				if view, err := receivingNetwork.current(); err != nil || view.Profile().Digest != f.profile.Digest {
					t.Fatal("receiving authority ended before original exchange join", err)
				}
			}
			link, linkErr := owner.link(ctx)
			if retiring {
				if linkErr == nil || link != "" {
					t.Fatal("late held response revived retired Publisher")
				}
			} else if linkErr != nil || link != expected {
				t.Fatal("actual ACK lacks current Link", linkErr)
			}
			predecessor, previous = previous, proof
			if retirement == "worker" && phase == "initial" {
				t.Run("accepted-history", func(t *testing.T) {
					acceptedNonce = installedPublisherAcceptedNonce(t, f, proof)
				})
			}
		})
	}
	if retirement == "delivery-close" && !t.Failed() {
		resultLost = true
		t.Run("retained-replay", func(t *testing.T) {
			installedPublisherFailedDelivery(t, f, predecessor, previous, deliveryGate)
		})
		if link, err := owner.link(ctx); err != nil || link != expected {
			t.Fatal("predecessor delivery failure retired current accepting pair", err)
		}
	}
	withdrawErr := owner.withdraw(ctx)
	if networkLost {
		observedNetwork.mu.Lock()
		t.Log("actual retained State refusal count", len(observedNetwork.causes))
		observedNetwork.mu.Unlock()
		var inspect func(error)
		inspect = func(err error) {
			if err == nil {
				return
			}
			t.Logf("withdrawal cause type=%T observed=%t", err, observedNetwork.contains(err))
			if many, ok := err.(interface{ Unwrap() []error }); ok {
				for _, child := range many.Unwrap() {
					inspect(child)
				}
			} else if one, ok := err.(interface{ Unwrap() error }); ok {
				inspect(one.Unwrap())
			}
		}
		inspect(withdrawErr)
	}
	if !checkCleanup(withdrawErr) || (retirement == "caller" || workerLost || networkLost || resultLost) && withdrawErr == nil {
		t.Fatal("owning withdrawal lost or changed original retirement result", withdrawErr)
	}
	if resultLost {
		var emitted *introduction.DeliveryResultFailure
		if !errors.As(withdrawErr, &emitted) || emitted.Unwrap() == nil || !errors.Is(emitted, introduction.ErrDeliveryInterrupted) {
			t.Fatal("owning join lacks original successful Publisher RESULT before failed CLOSE", withdrawErr)
		}
	}
	closeErr := owner.close()
	if !checkCleanup(closeErr) || (retirement == "caller" || workerLost || networkLost || resultLost) && closeErr == nil {
		t.Fatal("owning close lost or changed original retirement result", closeErr)
	}
	if repeated := owner.close(); !checkCleanup(repeated) || (repeated == nil) != (closeErr == nil) || repeated != nil && repeated.Error() != closeErr.Error() {
		t.Fatal("repeated close changed original terminal result", repeated)
	}
	operation.Close()
	if err := invocation.Close(); !checkInvocation(err) || retirement == "caller" && err == nil {
		t.Fatal("original snapshot worker did not join", err)
	} else if workerLost {
		t.Log("retained original worker-loss result separately from joined cleanup", err)
	}
	if (retirement == "caller" || workerLost || networkLost) && invocation.CompletedCurrent() {
		t.Fatal("failed Publisher cleanup supplied completed-current provenance")
	}
	if workerLost {
		t.Run("retained-history", func(t *testing.T) {
			if acceptedNonce == [32]byte{} || originalContext.Check(operation) == nil {
				t.Fatal("worker loss did not revoke the exact operation after genuine acceptance")
			}
			if originalContext.Context().Err() != nil {
				// Retained failed borrower cleanup revokes the local generation.
				// Its exact failed terminal report must still retain private history.
				select {
				case <-originalContext.Done():
				case <-ctx.Done():
					t.Fatal("original failed context did not report joined cleanup", ctx.Err())
				}
				if failure, completed := originalContext.Completion(); !completed || failure == nil {
					t.Fatal("worker-loss context supplied successful retirement", failure)
				}
			} else {
				select {
				case <-originalContext.Done():
					t.Fatal("Job retirement completed the still-live Administration context")
				default:
				}
			}
			// Original Publisher Close has joined every writer and opening user;
			// the original context is either live or has failed terminal cleanup.
			// Neither allows its watcher to clear entries. Read-only reflection
			// observes that private owner
			// without a production diagnostic API or a fabricated acceptance.
			publisher := reflect.ValueOf(owner.owner).Elem()
			if publisher.FieldByName("initialDeliveries").Len() != 0 {
				t.Fatal("original delivery users remain after Publisher physical join")
			}
			history := publisher.FieldByName("history").Elem()
			entry := history.FieldByName("entries").MapIndex(reflect.ValueOf(acceptedNonce))
			if !entry.IsValid() || entry.IsNil() || !entry.Elem().FieldByName("accepted").Bool() {
				t.Fatal("genuinely accepted nonce lost after original worker and Publisher join")
			}
			// Independent Store verification supplies the original signed
			// registration end. Neither the short capsule deadline nor the
			// later unacknowledged replacement may select replay retention.
			retentionEnd := predecessor.Introduction.NotAfter.Add(60 * time.Second)
			if !entry.Elem().FieldByName("retainUntil").Equal(reflect.ValueOf(retentionEnd)) {
				t.Fatal("accepted nonce retention differs from original registration expiry+60s")
			}
		})
	}
	for _, server := range servers {
		err := server.Close()
		if err != nil && !((networkLost || resultLost) && checkCleanup(err)) && (!(workerLost || networkLost) || !transport.IsPeerRetirementCause(err)) {
			t.Fatal("actual receiving join", err)
		} else if err != nil {
			t.Log("retained failed receiver after original authority loss", err)
		}
	}
}

// This separately selected profile keeps real clocks unchanged. Its longer
// qualification bound grants no additional Stock, Registration lifetime or retry.
func TestInstalledPublicationElapsedBothCarriers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	if time.Until(time.Now().UTC().Truncate(time.Hour).Add(time.Hour)) < 16*time.Minute {
		t.Fatal("invalid environment: elapsed profile requires sixteen minutes in the original token hour")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) { runInstalledPublisherACK(t, carrier, "elapsed") })
	}
}

func runInstalledPublisherElapsedPairs(t *testing.T, ctx context.Context, f *networkAdmissionFixture, owner publicationHandle, gate *publicationACKGate, store *reachability.Store, target [32]byte, expected string) {
	var first, replacement reachability.Descriptor
	var started, initialCommit, initialACKLower, switchLower, switchUpper time.Time
	waitUntil := func(test *testing.T, end time.Time) {
		test.Helper()
		timer := time.NewTimer(time.Until(end))
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			test.Fatal("original elapsed Publication bound ended", ctx.Err())
		}
	}
	readHeldProof := func(test *testing.T) reachability.Descriptor {
		test.Helper()
		raw, err := gate.record()
		if err != nil || len(raw) < 2 {
			test.Fatal("elapsed actual committed record absent", err)
		}
		proof, err := reachability.Verify(raw[2:], target, f.profile.NetworkID, f.profile.Digest, time.Now())
		if err != nil {
			test.Fatal("elapsed independently verified Descriptor", err)
		}
		return proof
	}
	if !t.Run("initial-delayed-ack", func(t *testing.T) {
		gate.arm(t)
		defer gate.release()
		finished := make(chan error, 1)
		started = time.Now()
		go func() {
			link, err := owner.publish(ctx)
			if err == nil && link != expected {
				err = errors.New("elapsed initial ACK returned foreign Link")
			}
			finished <- err
		}()
		select {
		case <-gate.entered:
			initialCommit = time.Now()
		case err := <-finished:
			t.Fatal("elapsed publication completed before real Store gate", err)
		case <-ctx.Done():
			t.Fatal("elapsed initial commit absent", ctx.Err())
		}
		first = readHeldProof(t)
		if link, err := owner.link(ctx); err == nil || link != "" {
			t.Fatal("elapsed pending initial supplied Link")
		}
		waitUntil(t, initialCommit.Add(12*time.Second))
		initialACKLower = time.Now()
		gate.release()
		select {
		case err := <-finished:
			if err != nil {
				t.Fatal("elapsed genuine initial ACK failed", err)
			}
		case <-ctx.Done():
			t.Fatal("elapsed initial exchange did not join", ctx.Err())
		}
	}) {
		t.Fatal("elapsed initial pair failed")
	}
	if !t.Run("creation-refresh", func(t *testing.T) {
		gate.arm(t)
		defer gate.release()
		// REGISTER creation lies between the Publish call and independently
		// observed initial Store commit. A refresh based on the delayed ACK
		// cannot reach this gate within that original creation interval +5s.
		latest := initialCommit.Add(305 * time.Second)
		timer := time.NewTimer(time.Until(latest))
		defer timer.Stop()
		select {
		case <-gate.entered:
		case <-timer.C:
			t.Fatal("automatic refresh missed original registration creation+300s")
		case <-ctx.Done():
			t.Fatal("elapsed refresh interrupted", ctx.Err())
		}
		observed := time.Now()
		if observed.Before(started.Add(300*time.Second)) || !observed.Before(initialACKLower.Add(300*time.Second)) {
			t.Fatal("automatic refresh did not use original registration creation")
		}
		t.Log("unchanged-clock automatic refresh elapsed", observed.Sub(started))
		replacement = readHeldProof(t)
		if replacement.Introduction.Revision != first.Introduction.Revision+1 || replacement.Introduction.Slot == first.Introduction.Slot || replacement.Introduction.RecipientKey == first.Introduction.RecipientKey || replacement.PublicationDigest != first.PublicationDigest {
			t.Fatal("elapsed automatic replacement changed identity or reused recipient")
		}
		if link, err := owner.link(ctx); err != nil || link != expected {
			t.Fatal("elapsed pending replacement lost original Link", err)
		}
		installedPublisherSubmitNonce(t, f, replacement, false)
		installedPublisherAcceptedNonce(t, f, first)
		switchLower = time.Now()
		gate.release()
		// A fresh independently admitted Source must complete real delivery
		// using the replacement. Its success bounds the actual pair switch.
		installedPublisherAcceptedNonce(t, f, replacement)
		switchUpper = time.Now()
		if _, err := owner.owner.Refresh(ctx); err == nil || err.Error() != "Publisher predecessor still retained" {
			t.Fatal("elapsed overlap acquired a third recipient", err)
		}
		raw, outcome, err := store.Lookup(target, f.profile.Digest, time.Now())
		if err != nil || outcome != reachability.Accepted {
			t.Fatal("elapsed replacement lacks real Store proof", err)
		}
		if link, err := owner.publish(ctx); err != nil || link != expected {
			t.Fatal("elapsed exact current retry failed", err)
		}
		again, outcome, err := store.Lookup(target, f.profile.Digest, time.Now())
		if err != nil || outcome != reachability.Accepted || !bytes.Equal(raw, again) {
			t.Fatal("elapsed exact retry changed signed recipient bytes", err)
		}
	}) {
		t.Fatal("elapsed automatic refresh failed")
	}
	if !t.Run("bounded-overlap", func(t *testing.T) {
		// The first original proof remains signed-valid well beyond overlap.
		// Real delivery at +45s and refusal after the observed switch+60s
		// isolate pair acceptance from Descriptor or capsule expiry.
		waitUntil(t, switchLower.Add(45*time.Second))
		installedPublisherAcceptedNonce(t, f, first)
		waitUntil(t, switchUpper.Add(61*time.Second))
		if err := first.Current(time.Now()); err != nil {
			t.Fatal("original signed Descriptor expired before overlap oracle", err)
		}
		installedPublisherSubmitNonce(t, f, first, false)
		installedPublisherAcceptedNonce(t, f, replacement)
		t.Log("unchanged-clock predecessor refusal after switch upper bound", time.Since(switchUpper))
	}) {
		t.Fatal("elapsed predecessor overlap failed")
	}
}

// Deliberate original capsule expiry is a failed operation. Every joined cause
// must retain that deadline, the original interrupted child or native peer cause;
// raw EOF, fabricated acknowledgement and foreign/mixed failures still refuse.
func publicationDeliveryExpiryOnly(err error) bool {
	if err == context.Canceled || err == context.DeadlineExceeded || err == os.ErrDeadlineExceeded || err == prefix.ErrIntroductionExpired || err == introduction.ErrDeliveryInterrupted || err == introduction.ErrWithdrawalUnavailable || transport.IsPeerRetirementCause(err) {
		return true
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		children := many.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !publicationDeliveryExpiryOnly(child) {
				return false
			}
		}
		return true
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return publicationDeliveryExpiryOnly(one.Unwrap())
	}
	return false
}

func publicationACKNetworkLossOnly(err error, observed *publicationACKNetworkFailures) bool {
	if err == context.Canceled || observed.contains(err) || transport.IsPeerRetirementCause(err) {
		return true
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		children := many.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !publicationACKNetworkLossOnly(child, observed) {
				return false
			}
		}
		return true
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return publicationACKNetworkLossOnly(one.Unwrap(), observed)
	}
	return false
}

// Every retained leaf must be the original deliberate cancellation. A physical
// error, EOF or unrelated refusal remains a failing native result.
func publicationACKCancellationOnly(err error) bool {
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		children := many.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !publicationACKCancellationOnly(child) {
				return false
			}
		}
		return true
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return publicationACKCancellationOnly(one.Unwrap())
	}
	return err == context.Canceled
}
