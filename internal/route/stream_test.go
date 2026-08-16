package route_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/localroles"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestBoundedOpaqueStreamCrossesEveryRoutePosition(t *testing.T) {
	runBoundedOpaqueStream(t, false, 8*time.Second, 0, 0, 0)
}

func TestEndpointSecuredAttachmentCrossesRouteWithoutPublisherCredential(t *testing.T) {
	runBoundedOpaqueStream(t, true, 8*time.Second, 0, 0, 0)
}

func TestActiveAttachmentOutlivesItsSetupDeadline(t *testing.T) {
	runBoundedOpaqueStream(t, true, 500*time.Millisecond, 3*time.Second, 0, 750*time.Millisecond)
}

func TestWaitingAttachmentUsesLifetimeBeforeSetupDeadline(t *testing.T) {
	runBoundedOpaqueStream(t, true, 500*time.Millisecond, 3*time.Second, 750*time.Millisecond, 0)
}

func runBoundedOpaqueStream(t *testing.T, endpointSecured bool, setupDeadline, lifetime,
	startDelay, hold time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	identities := make([]testRouteIdentity, 6)
	for index := range identities {
		identities[index] = routeIdentity(t, byte(index+61))
	}
	addresses := make([]string, 5)
	for index := range addresses {
		addresses[index] = unusedAddress(t)
	}
	plan := route.Plan{NetworkID: [32]byte{18}, Generation: "generation-8", Epoch: 8, Digest: [32]byte{17},
		Profile: "h3-route-tracer-v1", ViewRoot: [32]byte{16}, Seed: [32]byte{15}, SelectionAt: time.Now().Unix()}
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	for index, role := range roles {
		plan.Positions = append(plan.Positions, route.Position{Role: role, Domain: role,
			NodeID: [32]byte{byte(index + 21)}, PublicKey: identities[index].public,
			Family: "stream-family-" + role, Endpoint: addresses[index], Capacity: 1})
	}
	clientRoute, clientApplication := net.Pipe()
	publisherRoute, publisherApplication := net.Pipe()
	ready := make(chan route.Evidence, 5)
	done := make(chan route.Evidence, 6)
	start := func(config route.Actor, listener bool) {
		go func() {
			var notify func(route.Evidence)
			if listener {
				notify = func(value route.Evidence) { ready <- value }
			}
			observation, err := route.Run(ctx, config, notify)
			if err != nil {
				observation.Error = err.Error()
			}
			done <- observation
		}()
	}
	for index := 3; index >= 0; index-- {
		nextAddress, nextID, nextPin := addresses[4], [32]byte{90}, identities[4].public
		if index < 3 {
			nextAddress, nextID, nextPin = addresses[index+1], plan.Positions[index+1].NodeID, identities[index+1].public
		}
		upstream := identities[5].public
		if index > 0 {
			upstream = identities[index-1].public
		}
		start(route.Actor{ManifestDigest: [32]byte{99}, NetworkID: plan.NetworkID, EpochDigest: plan.Digest,
			Role: roles[index], NodeID: plan.Positions[index].NodeID, ListenAddress: addresses[index],
			Certificate: identities[index].certificate, UpstreamPin: upstream, NextNodeID: nextID,
			NextAddress: nextAddress, NextPin: nextPin, Deadline: setupDeadline, Lifetime: lifetime,
			MaximumAttachments: 4, AttachmentTarget: 1}, true)
	}
	publisher := route.Actor{Role: "publisher", ManifestDigest: [32]byte{99}, NetworkID: plan.NetworkID,
		EpochDigest: plan.Digest, NodeID: [32]byte{90}, ListenAddress: addresses[4],
		Certificate: identities[4].certificate, UpstreamPin: identities[3].public,
		ServiceCertificate: identities[4].certificate, Stream: publisherRoute, Deadline: setupDeadline, Lifetime: lifetime,
		MaximumAttachments: 4, AttachmentTarget: 1}
	localRoleRoot := t.TempDir()
	client := route.Actor{Role: "client", ManifestDigest: [32]byte{99}, Plan: plan,
		ClientCertificate: identities[5].certificate, PublisherPin: identities[4].public,
		Stream: clientRoute, LocalRoleStateRoot: localRoleRoot, Deadline: setupDeadline, Lifetime: lifetime}
	if endpointSecured {
		publisher.ServiceCertificate = tls.Certificate{}
		publisher.RawAttachment = true
		client.PublisherPin = [32]byte{}
		client.RawAttachment = true
	}
	start(publisher, true)
	for range 5 {
		select {
		case <-ready:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if startDelay > 0 {
		time.Sleep(startDelay)
	}
	start(client, false)
	applicationDeadline := time.Now().Add(5 * time.Second)
	if err := errors.Join(clientApplication.SetDeadline(applicationDeadline),
		publisherApplication.SetDeadline(applicationDeadline)); err != nil {
		t.Fatal(err)
	}
	if hold > 0 {
		time.Sleep(hold)
		roles, err := localroles.Open(localroles.Config{Root: localRoleRoot, Clock: time.Now})
		if err != nil {
			t.Fatal(err)
		}
		position := plan.Positions[1]
		if conflict, err := roles.Conflict(position.NodeID, sha256.Sum256([]byte(position.Family))); err != nil || !conflict {
			t.Fatalf("active Route local duty = %v, %v", conflict, err)
		}
		if err := roles.Close(); err != nil {
			t.Fatal(err)
		}
	}

	repetitions := 1024
	if endpointSecured {
		repetitions = 1 << 20
	}
	clientBytes := bytes.Repeat([]byte{0x3c, 0x00, 0xff, 0x17}, repetitions)
	publisherBytes := bytes.Repeat([]byte{0xa5, 0x42, 0x00, 0x7e}, repetitions)
	exchangeRouteApplications(t, clientApplication, publisherApplication, clientBytes, publisherBytes)
	if err := errors.Join(clientApplication.Close(), publisherApplication.Close()); err != nil {
		t.Fatal(err)
	}
	for range 6 {
		select {
		case observation := <-done:
			if observation.Error != "" || observation.Terminal != "success" || !observation.Cleanup {
				t.Fatalf("stream Route actor failed: %+v", observation)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if err := errors.Join(clientRoute.Close(), publisherRoute.Close()); err != nil {
		t.Fatal(err)
	}
}

func exchangeRouteApplications(t *testing.T, client, publisher net.Conn, clientBytes, publisherBytes []byte) {
	t.Helper()
	type result struct {
		value []byte
		err   error
	}
	results := make(chan result, 2)
	var writes sync.WaitGroup
	writes.Add(2)
	go func() {
		defer writes.Done()
		_, err := client.Write(clientBytes)
		if err != nil {
			t.Error(err)
		}
	}()
	go func() {
		defer writes.Done()
		_, err := publisher.Write(publisherBytes)
		if err != nil {
			t.Error(err)
		}
	}()
	go func() {
		value := make([]byte, len(publisherBytes))
		_, err := readExactly(client, value)
		results <- result{value, err}
	}()
	go func() {
		value := make([]byte, len(clientBytes))
		_, err := readExactly(publisher, value)
		results <- result{value, err}
	}()
	writes.Wait()
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("stream exchange failed: %v %v", first.err, second.err)
	}
	if !(bytes.Equal(first.value, publisherBytes) && bytes.Equal(second.value, clientBytes) ||
		bytes.Equal(second.value, publisherBytes) && bytes.Equal(first.value, clientBytes)) {
		t.Fatal("opaque Route stream changed bytes or order")
	}
}

func readExactly(connection net.Conn, destination []byte) (int, error) {
	total := 0
	for total < len(destination) {
		count, err := connection.Read(destination[total:])
		total += count
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
