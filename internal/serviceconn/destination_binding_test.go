package serviceconn_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func TestNameOriginConnectionClosesWhenTargetBindingChanges(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	client, publisher, publication := connectedEndpoints(t, fixture)
	record := namelease.Record{
		Name: "alice", Generation: 1, Revision: 2,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: "name-authority", Target: fixture.first.Target,
		LeaseExpiresAt: fixture.now.Add(time.Hour).Unix(), GraceExpiresAt: fixture.now.Add(2 * time.Hour).Unix(),
	}
	binding, _, err := namelease.ResolveBinding(record, fixture.now.Unix(), nil)
	if err != nil {
		t.Fatal(err)
	}
	updates := make(chan serviceconn.DestinationBinding, 1)
	clientRoute, publisherRoute := net.Pipe()
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	t.Cleanup(func() {
		_ = clientApplication.Close()
		_ = publisherApplication.Close()
	})

	type outcome struct {
		result serviceconn.Result
		err    error
	}
	results := make(chan outcome, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() {
		result, runErr := publisher.Do(ctx, serviceconn.Request{Action: "accept",
			Principal: fixture.publisherPrincipal, Session: session(publisher, fixture.publisherPrincipal, fixture.now),
			Route: publisherRoute, Application: publisherEndpoint, BytesEachDirection: 1, At: fixture.now})
		results <- outcome{result, runErr}
	}()
	go func() {
		result, runErr := client.Do(ctx, serviceconn.Request{Action: "connect",
			Principal: fixture.clientPrincipal, Session: session(client, fixture.clientPrincipal, fixture.now),
			Target: fixture.first.Target, Publication: publication, Route: clientRoute,
			Application: clientEndpoint, BytesEachDirection: 1, At: fixture.now,
			NameBinding: serviceBinding(binding), NameUpdates: updates})
		results <- outcome{result, runErr}
	}()
	replacement := record
	replacement.Revision++
	replacement.Target = [32]byte{99}
	replacementBinding, _, err := namelease.ResolveBinding(replacement, fixture.now.Add(time.Second).Unix(), nil)
	if err != nil {
		t.Fatal(err)
	}
	updates <- serviceBinding(replacementBinding)

	select {
	case completed := <-results:
		if completed.err == nil || completed.result.Class != "abrupt connection loss" {
			t.Fatalf("changed Name binding result=%+v err=%v", completed.result, completed.err)
		}
	case <-time.After(time.Second):
		t.Fatal("changed Name binding did not close the live connection")
	}
}

func serviceBinding(value namelease.Binding) serviceconn.DestinationBinding {
	return serviceconn.DestinationBinding{Name: value.Name, Generation: value.Generation, Revision: value.Revision,
		Authority: value.Authority, Target: value.Target, ParentName: value.ParentName,
		ParentGeneration: value.ParentGeneration, RecordDigest: value.RecordDigest, Commitment: value.Commitment}
}
