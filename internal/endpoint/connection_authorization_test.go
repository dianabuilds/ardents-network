package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

type connectionWorkCounts struct {
	state, entry, issuer, route int
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

type blockingApplicationEntry struct{ entered chan struct{} }

func (owner blockingApplicationEntry) Contact() (entry.Candidate, error) {
	return entry.Candidate{}, errors.New("blocking Entry has no direct contact")
}

func (owner blockingApplicationEntry) Acquire(ctx context.Context, _ entry.Attempt, _ entry.CandidateOpener) (net.Conn, func() error, error) {
	close(owner.entered)
	<-ctx.Done()
	return nil, nil, ctx.Err()
}

func (owner countingApplicationEntry) Acquire(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error) {
	owner.counts.route++
	return nil, nil, errors.New("test Route must not be opened")
}

func TestActiveConnectionWorkIsCancelledByGrantAndBrokerLifecycle(t *testing.T) {
	for _, terminal := range []string{"revoke", "drain", "close"} {
		t.Run(terminal, func(t *testing.T) {
			network, principal := [32]byte{1}, [32]byte{2}
			admission, err := broker.New(broker.Config{ID: [32]byte{3},
				Grants: []broker.Grant{{Principal: principal, Surface: broker.Connection, PermitDrain: true}}})
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
			link, err := targetlink.Encode(targetlink.Link{Network: network, Target: [32]byte{9}})
			if err != nil {
				t.Fatal(err)
			}
			entered := make(chan struct{})
			result := make(chan error, 1)
			deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
			go func() {
				_, openErr := endpoint.openApplicationConnection(context.Background(), userApplicationConnectionRequest{
					Introduction: userIntroductionRouteRequest{TargetLink: link,
						Introduction: userIntroductionProfile{NetworkID: network, Digest: [32]byte{4}, Epoch: 1,
							Introduction:     transitPeer{NodeID: [32]byte{5}, PublicKey: [32]byte{6}, Endpoint: "127.0.0.1:1"},
							RendezvousNodeID: [32]byte{7}, Reachability: [32]byte{8}, JoinHandle: [32]byte{10}, NotAfter: deadline,
							SubmissionAuthorization: []byte("fixed-test-authorization")},
						Entry: blockingApplicationEntry{entered: entered}, Initiator: transitPeer{NodeID: [32]byte{11}, PublicKey: [32]byte{12}, Endpoint: "127.0.0.1:2"},
						Rendezvous:   transitPeer{NodeID: [32]byte{7}, PublicKey: [32]byte{13}, Endpoint: "127.0.0.1:3"},
						AttachmentID: [32]byte{14}, EndpointHandshake: [32]byte{15}, At: time.Now().UTC()},
					Principal: principal, BytesEachDirection: 1,
				})
				result <- openErr
			}()
			<-entered
			if admission.Active() != 1 {
				t.Fatalf("active Route work session count = %d, want 1", admission.Active())
			}
			switch terminal {
			case "revoke":
				err = admission.Revoke(principal, broker.Connection)
			case "drain":
				err = admission.DrainUntil(broker.Connection, time.Now())
			case "close":
				admission.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := <-result; err == nil {
				t.Fatal("terminal Grant lifecycle left Route acquisition running")
			}
			if admission.Active() != 0 {
				t.Fatalf("terminal Grant lifecycle retained %d sessions", admission.Active())
			}
		})
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
			owner, err := endpoint.openConnectionInterface(connectionInterfaceConfig{Floor: floor,
				Current: func() (applicationStateView, error) {
					counts.state++
					return countingApplicationState{counts: counts}, nil
				}, Entry: countingApplicationEntry{counts: counts}, Principal: principal, BytesEachDirection: 1, Clock: func() time.Time { return at }})
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
