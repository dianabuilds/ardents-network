package forwarding

import (
	"crypto/tls"
	"encoding/hex"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

type ClosedForwardingProfile struct {
	Certificate        tls.Certificate
	AdmissionTraffic   resource.HostingTraffic
	TerminationTraffic resource.HostingTraffic
}

type forwardingFixtureConfig struct {
	Current              func() (state.NodeDuty, error)
	CurrentClosedRoute   func() (state.ClosedRouteView, error)
	CurrentClosedProfile func() (state.ClosedProfileView, bool)
	ClosedForwarding     ClosedForwardingProfile
	CarrierRelayEndpoint string
	now                  func() time.Time
}

func (config forwardingFixtureConfig) source() authority.Source {
	return authority.Source{CurrentRoute: config.CurrentClosedRoute, CurrentProfile: config.CurrentClosedProfile}
}

func forwardingDependencies(config forwardingFixtureConfig, host hosting.Host) dependencies {
	source := config.source()
	return dependencies{current: config.Current, authority: source, relayEndpoint: config.CarrierRelayEndpoint,
		literalEndpoint: literalForwardingFixtureEndpoint,
		verify: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
			return hosting.AdmissionVerifier(source, config.now, receiver, host, config.ClosedForwarding.AdmissionTraffic, config.ClosedForwarding.TerminationTraffic)
		},
		replenish: func(receiver route.ClosedRoleReceiver, spends *spending.Ledger) route.ClosedForwardingReplenisher {
			return hosting.Replenisher(source, config.now, receiver, host, spends, config.ClosedForwarding.AdmissionTraffic, config.ClosedForwarding.TerminationTraffic)
		},
	}
}

func closedRouteReceiver(config forwardingFixtureConfig, snapshot state.NodeDuty, purpose ardp.Purpose, now time.Time) (route.ClosedRoleReceiver, bool) {
	return config.source().Receiver(snapshot, purpose, now)
}

func literalForwardingFixtureEndpoint(endpoint string) bool {
	host, port, err := net.SplitHostPort(endpoint)
	number, portErr := strconv.Atoi(port)
	return err == nil && net.ParseIP(host) != nil && portErr == nil && number >= 1 && number <= 65535
}

type closedBootstrapFixture struct {
	now      time.Time
	config   forwardingFixtureConfig
	snapshot state.NodeDuty
	receiver route.ClosedRoleReceiver
	peer     [32]byte
	open     route.ClosedOpen
	view     state.ClosedRouteView
}

// Projection fixture only: accepted State parsing and its signature/record
// joins have their own tests. This exercises the Node's additional pre-dial
// adjacency, role transition and family exclusions.
func newClosedBootstrapFixture(t *testing.T) *closedBootstrapFixture {
	t.Helper()
	fixture := &closedBootstrapFixture{now: time.Unix(1_800_000_000, 0).UTC()}
	generation := [32]byte{1}
	profile := state.ClosedProfileView{NetworkID: [32]byte{2}, StateGeneration: generation, StateDigest: [32]byte{3}, Digest: [32]byte{4},
		Epoch: 5, IssuerNodeID: [32]byte{13}, IssuerDutyGeneration: 13, NotBefore: fixture.now, NotAfter: fixture.now.Add(time.Hour)}
	fixture.view = state.ClosedRouteView{Profile: profile, NodeCount: 3}
	for index := 0; index < 3; index++ {
		fixture.view.Nodes[index] = state.ClosedRouteNodeView{NodeID: [32]byte{byte(11 + index)}, RecordDigest: [32]byte{byte(21 + index)},
			RoleDomain: 1, Subrole: uint8(index + 1), DutyGeneration: uint64(11 + index)}
	}
	fixture.view.Nodes[2].RoleDomain, fixture.view.Nodes[2].Subrole = 2, 6
	fixture.snapshot = state.NodeDuty{Generation: hex.EncodeToString(generation[:]), NetworkID: profile.NetworkID, Epoch: profile.Epoch, Digest: profile.StateDigest,
		EpochValidFrom: fixture.now, ValidUntil: profile.NotAfter, Profile: carrier.ClosedRouteProfile, Fresh: true, NodeID: fixture.view.Nodes[1].NodeID,
		RecordGeneration: 12, RecordValidUntil: profile.NotAfter, DeclaredFamily: "bootstrap-interior", CandidateCount: 2}
	for index, role := range []state.ClosedRouteNodeView{fixture.view.Nodes[0], fixture.view.Nodes[2]} {
		fixture.snapshot.Candidates[index] = state.NodeDutyCandidate{NodeID: role.NodeID, RecordDigest: role.RecordDigest, PublicKey: [32]byte{byte(31 + index)},
			FamilyID: [32]byte{byte(41 + index)}, Endpoint: "127.0.0.1:41000", CarrierProfile: string(carrier.ClosedCarrierTCP), ValidFrom: fixture.now,
			ValidUntil: profile.NotAfter, AssignmentNotAfter: profile.NotAfter}
	}
	fixture.config = forwardingFixtureConfig{CurrentClosedRoute: func() (state.ClosedRouteView, error) { return fixture.view, nil }}
	var available bool
	fixture.receiver, available = closedRouteReceiver(fixture.config, fixture.snapshot, ardp.PurposeForwarding, fixture.now)
	if !available {
		t.Fatal("fixture receiver unavailable")
	}
	fixture.peer = fixture.snapshot.Candidates[0].PublicKey
	fixture.open = route.ClosedOpen{NextNodeID: profile.IssuerNodeID, NextDutyGeneration: profile.IssuerDutyGeneration,
		Purpose: ardp.PurposeIssuer, Deadline: fixture.now.Add(time.Second)}
	return fixture
}
