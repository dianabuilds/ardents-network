package resolution_test

import (
	"context"
	"crypto/sha256"
	"testing"

	nameresolution "github.com/dianabuilds/ardents-network/internal/naming/resolution"
)

type gatewayRoleView struct {
	operation, name, result string
	network, nonce, target  [32]byte
	deadline                int64
	generation, revision    uint64
}

type relayRoleView struct {
	origin, gateway   string
	request, response [32]byte
	requestBytes      uint64
	responseBytes     uint64
	keyID             byte
}

func TestResolutionRolesExposeOnlyTheirObservedFields(t *testing.T) {
	t.Parallel()
	fixture := newResolutionFixture(t)
	contexts := [][32]byte{{1}, {2}}
	var resolverNonces [][32]byte
	for index, isolation := range contexts {
		selection := fixture.admitted(t, fixture.selection, "alice", isolation, byte(index+1))
		resolver, err := nameresolution.Open(fixture.view, selection, fixture.gatewayProfile(), isolation,
			relayTransport(fixture.relayServer))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := resolver.Resolve(context.Background(), "alice", fixture.now); err != nil {
			t.Fatal(err)
		}
		view := resolver.RoleEvidence()
		if view.Operation != "resolve" || view.Name != "alice" || view.Isolation != isolation ||
			view.Network != [32]byte{9} || view.Target != [32]byte{1} ||
			view.Relay != [32]byte{1} || view.Gateway != [32]byte{2} || view.Rendezvous != [32]byte{3} ||
			view.Result != "resolved" || view.Generation != 1 || view.Revision != 1 ||
			view.Nonce == [32]byte{} || view.Deadline != fixture.selection.Deadline.UnixNano() {
			t.Fatalf("resolver role view=%+v", view)
		}
		resolverNonces = append(resolverNonces, view.Nonce)
	}
	if resolverNonces[0] == resolverNonces[1] {
		t.Fatal("resolver reused a nonce across Isolation Contexts")
	}

	gatewayViews, relayViews := fixture.roleEvidence()
	envelopes, _ := fixture.relayEvidence()
	if len(gatewayViews) != 2 || len(relayViews) != 2 || len(envelopes) != 2 {
		t.Fatalf("role view counts=%d/%d envelopes=%d", len(gatewayViews), len(relayViews), len(envelopes))
	}
	for index := range gatewayViews {
		gateway := gatewayViews[index]
		if gateway.operation != "resolve" || gateway.name != "alice" || gateway.network != [32]byte{9} ||
			gateway.nonce != resolverNonces[index] || gateway.target != [32]byte{1} || gateway.result != "resolved" ||
			gateway.generation != 1 || gateway.revision != 1 || gateway.deadline != fixture.selection.Deadline.UnixNano() {
			t.Fatalf("gateway role view %d=%+v", index, gateway)
		}
		relay := relayViews[index]
		if relay.origin == "" || relay.gateway == "" || relay.request != sha256.Sum256(envelopes[index]) ||
			relay.requestBytes != uint64(len(envelopes[index])) || relay.responseBytes == 0 || relay.keyID != envelopes[index][0] {
			t.Fatalf("relay role view %d=%+v", index, relay)
		}
	}
}
