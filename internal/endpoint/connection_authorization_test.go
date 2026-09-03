package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

type connectionWorkCounts struct {
	state, entry, issuer, route int
}

func openConnectionRoute(t *testing.T, network [32]byte, current func() (applicationStateView, error), entry applicationEntry,
	clock func() time.Time,
) *route.Route {
	t.Helper()
	owner, err := route.Open(route.Config{NetworkID: network, Current: func() (route.StateView, error) {
		view, viewErr := current()
		return view, viewErr
	}, Entry: entry, Credentials: func(context.Context, route.CredentialRequest) (route.Credential, error) {
		return route.Credential{}, errors.New("test credential adapter must not run")
	}, Admit: func(ctx context.Context) (func() error, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return func() error { return nil }, nil
	}, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	return owner
}

type countingApplicationState struct{ counts *connectionWorkCounts }

func (view countingApplicationState) Epoch(time.Time, time.Time) (state.ResolutionEpoch, bool) {
	view.counts.state++
	return state.ResolutionEpoch{}, false
}

func (view countingApplicationState) Candidate([32]byte, time.Time, time.Time) (state.ResolutionCandidate, bool) {
	view.counts.state++
	return state.ResolutionCandidate{}, false
}

func (view countingApplicationState) Gateway(time.Time, time.Time) (state.DestinationResolutionGateway, bool) {
	view.counts.state++
	return state.DestinationResolutionGateway{}, false
}

func (view countingApplicationState) CredentialIssuer(time.Time, time.Time) (state.TransitIssuer, bool) {
	view.counts.issuer++
	return state.TransitIssuer{}, false
}

type countingApplicationEntry struct{ counts *connectionWorkCounts }

func (owner countingApplicationEntry) Contact() (entry.Candidate, error) {
	owner.counts.entry++
	return entry.Candidate{}, errors.New("test Entry must not be read")
}

type blockingApplicationEntry struct {
	contact entry.Candidate
	entered chan struct{}
}

func (owner *blockingApplicationEntry) Contact() (entry.Candidate, error) {
	return owner.contact, nil
}

func (owner *blockingApplicationEntry) Acquire(ctx context.Context, _ entry.Attempt, _ entry.CandidateOpener) (net.Conn, func() error, error) {
	select {
	case owner.entered <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	<-ctx.Done()
	return nil, nil, ctx.Err()
}

type blockingConnectionWork struct {
	cancel context.CancelFunc
	result <-chan error
}

func startBlockingConnectionWork(t *testing.T, owner *connectionInterface, entryOwner *blockingApplicationEntry, link string) blockingConnectionWork {
	t.Helper()
	parent, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, openErr := owner.Open(parent, link)
		result <- openErr
	}()
	select {
	case <-entryOwner.entered:
		return blockingConnectionWork{cancel: cancel, result: result}
	case err := <-result:
		cancel()
		t.Fatalf("Connection workload did not become active: %v", err)
		return blockingConnectionWork{}
	}
}

type maintainedConnectionState struct {
	epoch     state.ResolutionEpoch
	gateway   state.DestinationResolutionGateway
	initiator state.ResolutionCandidate
}

func (view maintainedConnectionState) Epoch(time.Time, time.Time) (state.ResolutionEpoch, bool) {
	return view.epoch, true
}

func (view maintainedConnectionState) Candidate(node [32]byte, _ time.Time, _ time.Time) (state.ResolutionCandidate, bool) {
	return view.initiator, node == view.initiator.NodeID
}

func (view maintainedConnectionState) Gateway(time.Time, time.Time) (state.DestinationResolutionGateway, bool) {
	return view.gateway, true
}

func (maintainedConnectionState) CredentialIssuer(time.Time, time.Time) (state.TransitIssuer, bool) {
	return state.TransitIssuer{}, false
}

type maintainedConnectionComposition struct {
	admission *broker.Broker
	endpoint  *endpoint
	owner     *connectionInterface
	entry     *blockingApplicationEntry
	link      string
}

func openMaintainedConnectionComposition(t *testing.T, clock func() time.Time, administration bool) maintainedConnectionComposition {
	t.Helper()
	at := clock().UTC().Truncate(time.Millisecond)
	network, connectionPrincipal, administrationPrincipal := [32]byte{41}, [32]byte{42}, [32]byte{43}
	grants := []broker.Grant{{Principal: connectionPrincipal, Surface: broker.Connection}}
	if administration {
		grants = append(grants, broker.Grant{Principal: administrationPrincipal, Surface: broker.Administration})
	}
	admission, err := broker.New(broker.Config{ID: [32]byte{44}, Clock: clock, Grants: grants})
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := newEndpoint(setup{NetworkID: network, BrokerID: [32]byte{44}, ConnectionPrincipal: connectionPrincipal,
		AdministrationPrincipal: administrationPrincipal, Admission: admission})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = endpoint.Close() })
	floor, link := acceptedConnectionFloor(t, network, at)
	gatewayPublic, gatewayPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), NetworkID: network})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	gatewayNode := [32]byte{45}
	gateway, err := reachability.NewGateway(reachability.GatewayConfig{NetworkID: network, NodeID: gatewayNode,
		IdentityKey: gatewayPrivate, AssignmentNotAfter: at.Add(time.Minute), Store: store, Clock: clock,
		AuthorizeDescriptor: func(reachability.Descriptor, time.Time) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayProfile, err := reachability.EncodeGatewayProfile(gateway.Profile())
	if err != nil {
		t.Fatal(err)
	}
	var gatewayKey [32]byte
	copy(gatewayKey[:], gatewayPublic)
	initiatorFamily := "maintained-connection-initiator"
	contact := entry.Candidate{NodeID: [32]byte{46}, PublicKey: [32]byte{47},
		FamilyID: sha256.Sum256([]byte(initiatorFamily)), Endpoint: "127.0.0.1:1"}
	view := maintainedConnectionState{
		epoch: state.ResolutionEpoch{Generation: "maintained-composition", NetworkID: network, Number: 1, Digest: [32]byte{48}},
		gateway: state.DestinationResolutionGateway{NodeID: gatewayNode, PublicKey: gatewayKey, Family: [32]byte{49},
			Profile: gatewayProfile, AssignmentNotAfter: at.Add(time.Minute)},
		initiator: state.ResolutionCandidate{NodeID: contact.NodeID, PublicKey: contact.PublicKey, Family: initiatorFamily,
			Endpoint: contact.Endpoint, Domain: "initiator", AssignmentNotAfter: at.Add(time.Minute)},
	}
	entryOwner := &blockingApplicationEntry{contact: contact, entered: make(chan struct{}, 1)}
	current := func() (applicationStateView, error) { return view, nil }
	userRoute := openConnectionRoute(t, network, current, entryOwner, clock)
	owner, err := endpoint.openConnectionInterface(connectionInterfaceConfig{Floor: floor,
		Route: userRoute, Principal: connectionPrincipal, BytesEachDirection: 1, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	return maintainedConnectionComposition{admission: admission, endpoint: endpoint, owner: owner, entry: entryOwner, link: link}
}

func (owner countingApplicationEntry) Acquire(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error) {
	owner.counts.route++
	return nil, nil, errors.New("test Route must not be opened")
}

func TestAdmissionCapabilityTTLDoesNotTerminateActiveEndpointWork(t *testing.T) {
	synctest.Test(t, admissionCapabilityTTLDoesNotTerminateActiveEndpointWork)
}

func admissionCapabilityTTLDoesNotTerminateActiveEndpointWork(t *testing.T) {
	// Keep the independent 15-second private-lookup deadline just beyond the
	// capability boundary so this proof isolates the Broker lifecycle timer.
	clock := func() time.Time { return time.Now().Add(time.Millisecond) }
	composition := openMaintainedConnectionComposition(t, clock, false)
	work := startBlockingConnectionWork(t, composition.owner, composition.entry, composition.link)
	if active := composition.admission.Active(); active != 1 {
		t.Fatalf("active Endpoint workload count = %d, want 1", active)
	}
	time.Sleep(15*time.Second + time.Nanosecond)
	synctest.Wait()
	select {
	case err := <-work.result:
		t.Fatalf("pending capability TTL terminated active Endpoint work: %v", err)
	default:
	}
	if active := composition.admission.Active(); active != 1 {
		t.Fatalf("capability TTL changed active Endpoint workload count to %d", active)
	}
	work.cancel()
	if err := <-work.result; err == nil {
		t.Fatal("parent cancellation left active Endpoint work running")
	}
}

func TestConnectionCapacityIsExactWithAndWithoutAdministrationPressure(t *testing.T) {
	for _, administration := range []bool{false, true} {
		for _, workloadCount := range []int{7, 16, 64} {
			name := fmt.Sprintf("connections-%d/administration-%t", workloadCount, administration)
			t.Run(name, func(t *testing.T) {
				composition := openMaintainedConnectionComposition(t, time.Now, administration)
				administrationCapabilities := 0
				if administration {
					administrationCapabilities = 6
					for index := 0; index < administrationCapabilities; index++ {
						if _, err := composition.endpoint.Admit([32]byte{43}, broker.Administration); err != nil {
							t.Fatalf("Administration capability %d: %v", index+1, err)
						}
					}
				}
				workloads := make([]blockingConnectionWork, 0, workloadCount)
				for index := 0; index < workloadCount; index++ {
					workloads = append(workloads, startBlockingConnectionWork(t, composition.owner, composition.entry, composition.link))
				}
				if active := int(composition.admission.Active()); active != workloadCount+administrationCapabilities {
					t.Fatalf("active admission pressure = %d, want %d real workloads + %d Administration capabilities",
						active, workloadCount, administrationCapabilities)
				}
				if workloadCount == 64 {
					extraContext, cancelExtra := context.WithCancel(context.Background())
					extraResult := make(chan error, 1)
					go func() {
						_, openErr := composition.owner.Open(extraContext, composition.link)
						extraResult <- openErr
					}()
					select {
					case <-composition.entry.entered:
						cancelExtra()
						<-extraResult
						t.Fatal("Connection 65 touched Entry instead of failing at the exact admission boundary")
					case err := <-extraResult:
						cancelExtra()
						if err == nil {
							t.Fatal("Connection 65 exceeded the exact capacity")
						}
					}
				}
				for _, work := range workloads {
					work.cancel()
					if err := <-work.result; err == nil {
						t.Fatal("cancelled active Connection workload returned success")
					}
				}
				if active := int(composition.admission.Active()); active != administrationCapabilities {
					t.Fatalf("released workloads left admission pressure %d, want %d", active, administrationCapabilities)
				}
			})
		}
	}
}

func TestConnectionAuthorizationPrecedesStateEntryIssuerAndRouteWork(t *testing.T) {
	at := time.Unix(2_000_600_000, 0).UTC()
	network, principal := [32]byte{1}, [32]byte{2}
	for _, lifecycle := range []string{"revoked", "draining", "closed"} {
		t.Run(lifecycle, func(t *testing.T) {
			admission, err := broker.New(broker.Config{ID: [32]byte{3}, Clock: func() time.Time { return at },
				Grants: []broker.Grant{{Principal: principal, Surface: broker.Connection, PermitDrain: true}}})
			if err != nil {
				t.Fatal(err)
			}
			switch lifecycle {
			case "revoked":
				err = admission.Revoke(principal, broker.Connection)
			case "draining":
				err = admission.DrainUntil(broker.Connection, at.Add(time.Minute))
			case "closed":
				admission.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			endpoint, err := newEndpoint(setup{NetworkID: network, BrokerID: [32]byte{3}, ConnectionPrincipal: principal, Admission: admission})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if closeErr := endpoint.Close(); closeErr != nil {
					t.Errorf("close Endpoint: %v", closeErr)
				}
			})
			floor, link := acceptedConnectionFloor(t, network, at)
			counts := &connectionWorkCounts{}
			current := func() (applicationStateView, error) {
				counts.state++
				return countingApplicationState{counts: counts}, nil
			}
			userRoute := openConnectionRoute(t, network, current, countingApplicationEntry{counts: counts}, func() time.Time { return at })
			owner, err := endpoint.openConnectionInterface(connectionInterfaceConfig{Floor: floor,
				Route: userRoute, Principal: principal, BytesEachDirection: 1, Clock: func() time.Time { return at }})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := owner.Open(context.Background(), link); err == nil {
				t.Fatal("unavailable Connection Grant opened Application work")
			}
			if *counts != (connectionWorkCounts{}) {
				t.Fatalf("%s Grant touched protected work before authorization: %+v", lifecycle, *counts)
			}
		})
	}
}

func acceptedConnectionFloor(t *testing.T, network [32]byte, at time.Time) (*alpha.PersistentFloor, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://service.test")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "connection-authorization", Network: network, Serial: 1,
		Bindings: []alpha.BindingInput{{Link: link, Target: [32]byte{9}}}, NotBefore: at.Add(-time.Second), NotAfter: at.Add(time.Minute)}, private)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(public, raw)
	if err != nil {
		t.Fatal(err)
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: t.TempDir(), Authority: public,
		Cohort: "connection-authorization", Network: network})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := floor.Close(); closeErr != nil {
			t.Errorf("close alpha floor: %v", closeErr)
		}
	})
	if err := floor.Observe(corpus); err != nil {
		t.Fatal(err)
	}
	return floor, link.String()
}
