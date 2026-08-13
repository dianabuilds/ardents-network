package serviceconn_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func TestLocalGrantsKeepConnectionAdministrationAndCustodySeparate(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	publisher := newPublisher(t, fixture)

	connection, err := publisher.Do(context.Background(), serviceconn.Request{
		Action: "admit", Surface: "connection", Principal: fixture.publisherPrincipal, At: fixture.now,
	})
	if err != nil || connection.Class != "authorized" {
		t.Fatalf("admit connection: result=%+v err=%v", connection, err)
	}
	if result, err := publisher.Do(context.Background(), serviceconn.Request{
		Action: "publish", Principal: fixture.publisherPrincipal, Session: connection.Session,
		Credential: fixture.first, InstancePrivate: fixture.firstPrivate,
		IntroductionAcknowledgement: [32]byte{1}, At: fixture.now,
	}); err == nil || result.Class != "local authorization or policy denial" {
		t.Fatalf("connection grant administered service: result=%+v err=%v", result, err)
	}

	administration := admit(t, publisher, "administration", fixture.administrationPrincipal, fixture.now)
	if result, err := publisher.Do(context.Background(), serviceconn.Request{
		Action: "publish", Principal: fixture.administrationPrincipal, Session: administration,
		Credential: fixture.first, InstancePrivate: fixture.firstPrivate, At: fixture.now,
	}); err == nil || result.Class != "service unavailable" {
		t.Fatalf("publication succeeded before Introduction acknowledgement: result=%+v err=%v", result, err)
	}

	administration = admit(t, publisher, "administration", fixture.administrationPrincipal, fixture.now)
	published, err := publisher.Do(context.Background(), serviceconn.Request{
		Action: "publish", Principal: fixture.administrationPrincipal, Session: administration,
		Credential: fixture.first, InstancePrivate: fixture.firstPrivate,
		IntroductionAcknowledgement: [32]byte{1}, At: fixture.now,
	})
	if err != nil || published.Class != "published" || len(published.Publication) == 0 {
		t.Fatalf("publish current Instance: result=%+v err=%v", published, err)
	}
	if bytes.Contains(published.Publication, fixture.firstPrivate) || bytes.Contains(published.Publication, fixture.authorityPrivate) {
		t.Fatal("public publication exported private authority or Instance material")
	}

	if result, err := publisher.Do(context.Background(), serviceconn.Request{
		Action: "unpublish", Principal: fixture.administrationPrincipal, Session: administration, At: fixture.now,
	}); err == nil || result.Class != "local authorization or policy denial" {
		t.Fatalf("one-use administration session replayed: result=%+v err=%v", result, err)
	}
	if result, err := publisher.Do(context.Background(), serviceconn.Request{
		Action: "accept", Principal: fixture.hostilePrincipal, Session: connection.Session, At: fixture.now,
	}); err == nil || result.Class != "local authorization or policy denial" {
		t.Fatalf("stolen session accepted for sibling: result=%+v err=%v", result, err)
	}

	restarted := newPublisher(t, fixture)
	if result, err := restarted.Do(context.Background(), serviceconn.Request{
		Action: "accept", Principal: fixture.publisherPrincipal, Session: connection.Session, At: fixture.now,
	}); err == nil || result.Class != "local authorization or policy denial" {
		t.Fatalf("session survived broker restart: result=%+v err=%v", result, err)
	}
}

func TestPublicationRejectsWrongPossessionValidityScopeAndGeneration(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	publisher := newPublisher(t, fixture)

	cases := []struct {
		name       string
		credential serviceconn.Credential
		private    ed25519.PrivateKey
		at         time.Time
	}{
		{"wrong Instance key", fixture.first, fixture.secondPrivate, fixture.now},
		{"not yet valid", fixture.first, fixture.firstPrivate, fixture.now.Add(-time.Hour)},
		{"expired", fixture.first, fixture.firstPrivate, fixture.now.Add(time.Hour)},
		{"wrong network", alterCredential(t, fixture, func(value *serviceconn.Credential) { value.NetworkID[0]++ }), fixture.firstPrivate, fixture.now},
		{"wrong capability", alteredUnsigned(fixture.first, func(value *serviceconn.Credential) { value.Capabilities = 0 }), fixture.firstPrivate, fixture.now},
	}
	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			session := admit(t, publisher, "administration", fixture.administrationPrincipal, fixture.now)
			result, err := publisher.Do(context.Background(), serviceconn.Request{
				Action: "publish", Principal: fixture.administrationPrincipal, Session: session,
				Credential: test.credential, InstancePrivate: test.private,
				IntroductionAcknowledgement: [32]byte{1}, At: test.at,
			})
			if err == nil || result.Class != "service target authentication failure" {
				t.Fatalf("invalid publication accepted: result=%+v err=%v", result, err)
			}
		})
	}

	publish(t, publisher, fixture, fixture.first, fixture.firstPrivate)
	publish(t, publisher, fixture, fixture.second, fixture.secondPrivate)
	if fixture.first.Target != fixture.second.Target || fixture.first.InstancePublic == fixture.second.InstancePublic {
		t.Fatal("fixture did not preserve Target while changing Instance Key")
	}

	staleSession := admit(t, publisher, "administration", fixture.administrationPrincipal, fixture.now)
	if result, err := publisher.Do(context.Background(), serviceconn.Request{
		Action: "publish", Principal: fixture.administrationPrincipal, Session: staleSession,
		Credential: fixture.first, InstancePrivate: fixture.firstPrivate,
		IntroductionAcknowledgement: [32]byte{2}, At: fixture.now,
	}); err == nil || result.Class != "service target authentication failure" {
		t.Fatalf("lower generation republished: result=%+v err=%v", result, err)
	}
}

func TestExactTargetServiceConnectionCarriesOpaqueBytesBothDirections(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	publisher := newPublisher(t, fixture)
	publication := publish(t, publisher, fixture, fixture.first, fixture.firstPrivate)
	client, err := serviceconn.New(serviceconn.Setup{
		NetworkID: fixture.networkID, BrokerID: [32]byte{8}, AuthorityPublic: fixture.authorityPublic,
		ConnectionPrincipal: fixture.clientPrincipal,
	})
	if err != nil {
		t.Fatal(err)
	}
	clientSession := admit(t, client, "connection", fixture.clientPrincipal, fixture.now)
	publisherSession := admit(t, publisher, "connection", fixture.publisherPrincipal, fixture.now)

	clientRoute, publisherRoute := net.Pipe()
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer clientApplication.Close()
	defer publisherApplication.Close()

	type outcome struct {
		result serviceconn.Result
		err    error
	}
	outcomes := make(chan outcome, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() {
		result, runErr := publisher.Do(ctx, serviceconn.Request{
			Action: "accept", Principal: fixture.publisherPrincipal, Session: publisherSession,
			Route: publisherRoute, Application: publisherEndpoint, BytesEachDirection: 64 << 10, At: fixture.now,
		})
		outcomes <- outcome{result, runErr}
	}()
	go func() {
		result, runErr := client.Do(ctx, serviceconn.Request{
			Action: "connect", Principal: fixture.clientPrincipal, Session: clientSession,
			Target: fixture.first.Target, Publication: publication, Route: clientRoute,
			Application: clientEndpoint, BytesEachDirection: 64 << 10, At: fixture.now,
		})
		outcomes <- outcome{result, runErr}
	}()

	clientBytes := seededBytes(64<<10, 17)
	publisherBytes := seededBytes(64<<10, 91)
	assertExchange(t, clientApplication, publisherApplication, clientBytes, publisherBytes)
	for range 2 {
		completed := <-outcomes
		if completed.err != nil || completed.result.Class != "clean service connection close" ||
			completed.result.AuthenticatedTarget != fixture.first.Target || completed.result.Generation != 1 ||
			completed.result.AcceptedBytes != 64<<10 || completed.result.ReceivedBytes != 64<<10 {
			t.Fatalf("Service Connection failed: result=%+v err=%v", completed.result, completed.err)
		}
	}

	wrongClient, _ := serviceconn.New(serviceconn.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{9},
		AuthorityPublic: fixture.authorityPublic, ConnectionPrincipal: fixture.clientPrincipal})
	wrongSession := admit(t, wrongClient, "connection", fixture.clientPrincipal, fixture.now)
	result, err := wrongClient.Do(context.Background(), serviceconn.Request{
		Action: "connect", Principal: fixture.clientPrincipal, Session: wrongSession,
		Target: [32]byte{99}, Publication: publication, At: fixture.now,
	})
	if err == nil || result.Class != "service target authentication failure" || strings.Contains(result.Reason, "route") {
		t.Fatalf("wrong Target returned dishonest result: result=%+v err=%v", result, err)
	}
}

type fixture struct {
	now                                            time.Time
	networkID, clientPrincipal, publisherPrincipal [32]byte
	administrationPrincipal, hostilePrincipal      [32]byte
	authorityPublic, firstPublic, secondPublic     ed25519.PublicKey
	authorityPrivate, firstPrivate, secondPrivate  ed25519.PrivateKey
	first, second                                  serviceconn.Credential
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	value := fixture{now: time.Unix(2_000_000_000, 0), networkID: [32]byte{1}, clientPrincipal: [32]byte{2},
		publisherPrincipal: [32]byte{3}, administrationPrincipal: [32]byte{4}, hostilePrincipal: [32]byte{5}}
	var err error
	value.authorityPublic, value.authorityPrivate, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	value.firstPublic, value.firstPrivate, _ = ed25519.GenerateKey(rand.Reader)
	value.secondPublic, value.secondPrivate, _ = ed25519.GenerateKey(rand.Reader)
	value.first = issue(t, value, value.firstPublic, 1, 3)
	value.second = issue(t, value, value.secondPublic, 2, 3)
	return value
}

func issue(t *testing.T, fixture fixture, public ed25519.PublicKey, generation uint64, capabilities uint32) serviceconn.Credential {
	t.Helper()
	var authority, instance [32]byte
	copy(authority[:], fixture.authorityPublic)
	copy(instance[:], public)
	credential, err := serviceconn.IssueCredential(fixture.authorityPrivate, serviceconn.Credential{
		AuthorityPublic: authority, InstancePublic: instance, Generation: generation,
		NotBefore: fixture.now.Add(-time.Minute).Unix(), NotAfter: fixture.now.Add(time.Minute).Unix(),
		NetworkID: fixture.networkID, Capabilities: capabilities,
	})
	if err != nil {
		t.Fatal(err)
	}
	return credential
}

func alterCredential(t *testing.T, fixture fixture, change func(*serviceconn.Credential)) serviceconn.Credential {
	t.Helper()
	value := fixture.first
	change(&value)
	credential, err := serviceconn.IssueCredential(fixture.authorityPrivate, value)
	if err != nil {
		t.Fatal(err)
	}
	return credential
}

func alteredUnsigned(value serviceconn.Credential, change func(*serviceconn.Credential)) serviceconn.Credential {
	change(&value)
	return value
}

func newPublisher(t *testing.T, fixture fixture) *serviceconn.Endpoint {
	t.Helper()
	endpoint, err := serviceconn.New(serviceconn.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{7},
		AuthorityPublic: fixture.authorityPublic, ConnectionPrincipal: fixture.publisherPrincipal,
		AdministrationPrincipal: fixture.administrationPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	return endpoint
}

func admit(t *testing.T, endpoint *serviceconn.Endpoint, surface string, principal [32]byte, at time.Time) [32]byte {
	t.Helper()
	result, err := endpoint.Do(context.Background(), serviceconn.Request{Action: "admit", Surface: surface, Principal: principal, At: at})
	if err != nil || result.Class != "authorized" || result.Session == [32]byte{} {
		t.Fatalf("admit %s: result=%+v err=%v", surface, result, err)
	}
	return result.Session
}

func publish(t *testing.T, endpoint *serviceconn.Endpoint, fixture fixture, credential serviceconn.Credential, private ed25519.PrivateKey) []byte {
	t.Helper()
	session := admit(t, endpoint, "administration", fixture.administrationPrincipal, fixture.now)
	result, err := endpoint.Do(context.Background(), serviceconn.Request{Action: "publish",
		Principal: fixture.administrationPrincipal, Session: session, Credential: credential,
		InstancePrivate: private, IntroductionAcknowledgement: [32]byte{byte(credential.Generation)}, At: fixture.now})
	if err != nil || result.Class != "published" {
		t.Fatalf("publish generation %d: result=%+v err=%v", credential.Generation, result, err)
	}
	return result.Publication
}

func seededBytes(length, seed int) []byte {
	value := make([]byte, length)
	for index := range value {
		value[index] = byte((index*131 + seed) % 251)
	}
	return value
}

func assertExchange(t *testing.T, client, publisher net.Conn, clientBytes, publisherBytes []byte) {
	t.Helper()
	type result struct {
		got []byte
		err error
	}
	results := make(chan result, 2)
	var writers sync.WaitGroup
	writers.Add(2)
	go func() {
		defer writers.Done()
		_, err := client.Write(clientBytes)
		if err != nil {
			results <- result{err: err}
		}
	}()
	go func() {
		defer writers.Done()
		_, err := publisher.Write(publisherBytes)
		if err != nil {
			results <- result{err: err}
		}
	}()
	go func() {
		got := make([]byte, len(publisherBytes))
		_, err := netReadFull(client, got)
		results <- result{got, err}
	}()
	go func() {
		got := make([]byte, len(clientBytes))
		_, err := netReadFull(publisher, got)
		results <- result{got, err}
	}()
	writers.Wait()
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("Application exchange failed: %v %v", first.err, second.err)
	}
	if !(bytes.Equal(first.got, publisherBytes) && bytes.Equal(second.got, clientBytes) ||
		bytes.Equal(second.got, publisherBytes) && bytes.Equal(first.got, clientBytes)) {
		t.Fatal("Application bytes changed length or order")
	}
}

func netReadFull(connection net.Conn, destination []byte) (int, error) {
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
