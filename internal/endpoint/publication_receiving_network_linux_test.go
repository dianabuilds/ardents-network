package endpoint_test

import "testing"

func TestPublicationReceivingNetworkSurvivesPublisherConflict(t *testing.T) {
	publisher, _, _ := publicationRouteNetwork(t, "ardents-carrier-tcp-tls-v2")
	receiver := publicationReceivingNetwork(t, publisher)
	before, err := receiver.current()
	if err != nil {
		t.Fatal(err)
	}
	if err := publisher.profileConflict(t); err == nil || err.Error() != "closed profile has a durable conflict" {
		t.Fatalf("Publisher profile conflict = %v", err)
	}
	if _, err := publisher.current(); err == nil || err.Error() != "closed profile is unavailable" {
		t.Fatalf("Publisher authority after conflict = %v", err)
	}
	after, err := receiver.current()
	if err != nil || after.Profile().ProfileBinding != before.Profile().ProfileBinding {
		t.Fatalf("receiving authority after Publisher conflict: %v", err)
	}
	for _, node := range publisher.spec.Nodes {
		if _, err := after.Member(node.NodeID, after.ObservedAt()); err != nil {
			t.Errorf("receiver lost assignment for node %x: %v", node.NodeID, err)
		}
	}
}
