//go:build referencec2

package endpoint_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

type opaqueInstanceSigner struct {
	private ed25519.PrivateKey
}

func (signer opaqueInstanceSigner) Public() crypto.PublicKey { return signer.private.Public() }

func (signer opaqueInstanceSigner) Sign(random io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return signer.private.Sign(random, digest, opts)
}

func TestLocalGrantsKeepConnectionAdministrationAndCustodySeparate(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	publisher := newPublisher(t, fixture)

	connection, err := publisher.Admit(fixture.publisherPrincipal, broker.Connection)
	if err != nil || connection == [32]byte{} {
		t.Fatalf("admit connection: session=%x err=%v", connection, err)
	}
	if result, err := publisher.Publish(context.Background(), endpointapi.PublicationRequest{
		Principal: fixture.publisherPrincipal, Capability: connection,
		Credential: fixture.first, InstanceSigner: fixture.firstPrivate,
		IntroductionAcknowledgement: acknowledgement(fixture, fixture.first), At: fixture.now,
	}); err == nil || result.Class != "local authorization or policy denial" {
		t.Fatalf("connection grant administered service: result=%+v err=%v", result, err)
	}

	administration := admit(t, publisher, "administration", fixture.administrationPrincipal, fixture.now)
	if result, err := publisher.Publish(context.Background(), endpointapi.PublicationRequest{
		Principal: fixture.administrationPrincipal, Capability: administration,
		Credential: fixture.first, InstanceSigner: fixture.firstPrivate, At: fixture.now,
	}); err == nil || result.Class != "service unavailable" {
		t.Fatalf("publication succeeded before Introduction acknowledgement: result=%+v err=%v", result, err)
	}

	administration = admit(t, publisher, "administration", fixture.administrationPrincipal, fixture.now)
	published, err := publisher.Publish(context.Background(), endpointapi.PublicationRequest{
		Principal: fixture.administrationPrincipal, Capability: administration,
		Credential: fixture.first, InstanceSigner: opaqueInstanceSigner{private: fixture.firstPrivate},
		IntroductionAcknowledgement: acknowledgement(fixture, fixture.first), At: fixture.now,
	})
	if err != nil || published.Class != "published" || len(published.Record) == 0 {
		t.Fatalf("publish current Instance: result=%+v err=%v", published, err)
	}
	if bytes.Contains(published.Record, fixture.firstPrivate) || bytes.Contains(published.Record, fixture.authorityPrivate) {
		t.Fatal("public publication exported private authority or Instance material")
	}

	if result, err := publisher.Withdraw(context.Background(), endpointapi.WithdrawalRequest{
		Principal: fixture.administrationPrincipal, Capability: administration, At: fixture.now,
	}); err == nil || result.Class != "local authorization or policy denial" {
		t.Fatalf("one-use administration session replayed: result=%+v err=%v", result, err)
	}
	if result, err := publisher.Accept(context.Background(), endpointapi.InboundConnectionRequest{
		Principal: fixture.hostilePrincipal, Capability: connection, At: fixture.now,
	}); err == nil || result.Class != "local authorization or policy denial" {
		t.Fatalf("stolen session accepted for sibling: result=%+v err=%v", result, err)
	}

	restarted := newPublisher(t, fixture)
	if result, err := restarted.Accept(context.Background(), endpointapi.InboundConnectionRequest{
		Principal: fixture.publisherPrincipal, Capability: connection, At: fixture.now,
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
		credential endpointapi.Credential
		private    ed25519.PrivateKey
		at         time.Time
	}{
		{"wrong Instance key", fixture.first, fixture.secondPrivate, fixture.now},
		{"not yet valid", fixture.first, fixture.firstPrivate, fixture.now.Add(-time.Hour)},
		{"expired", fixture.first, fixture.firstPrivate, fixture.now.Add(time.Hour)},
		{"wrong network", alterCredential(t, fixture, func(value *endpointapi.Credential) { value.NetworkID[0]++ }), fixture.firstPrivate, fixture.now},
		{"wrong capability", alteredUnsigned(fixture.first, func(value *endpointapi.Credential) { value.Capabilities = 0 }), fixture.firstPrivate, fixture.now},
	}
	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			session := admit(t, publisher, "administration", fixture.administrationPrincipal, test.at)
			result, err := publisher.Publish(context.Background(), endpointapi.PublicationRequest{
				Principal: fixture.administrationPrincipal, Capability: session,
				Credential: test.credential, InstanceSigner: test.private,
				IntroductionAcknowledgement: acknowledgement(fixture, test.credential), At: test.at,
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
	if result, err := publisher.Publish(context.Background(), endpointapi.PublicationRequest{
		Principal: fixture.administrationPrincipal, Capability: staleSession,
		Credential: fixture.first, InstanceSigner: fixture.firstPrivate,
		IntroductionAcknowledgement: acknowledgement(fixture, fixture.first), At: fixture.now,
	}); err == nil || result.Class != "service target authentication failure" {
		t.Fatalf("lower generation republished: result=%+v err=%v", result, err)
	}
}

func TestSessionExpiresAndGenerationSurvivesEndpointRestart(t *testing.T) {
	fixture := newFixture(t)
	clock := fixture.now
	publisher, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{7},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.publisherPrincipal, AdministrationPrincipal: fixture.administrationPrincipal,
		PublicationRoot: t.TempDir(), Clock: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Close()
	session := admit(t, publisher, "connection", fixture.publisherPrincipal, fixture.now)
	clock = clock.Add(16 * time.Second)
	result, err := publisher.Accept(context.Background(), endpointapi.InboundConnectionRequest{
		Principal: fixture.publisherPrincipal, Capability: session, At: fixture.now})
	if err == nil || result.Class != "local authorization or policy denial" {
		t.Fatalf("expired session remained usable: result=%+v err=%v", result, err)
	}

	state := t.TempDir() + "/generation"
	setup := endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{7},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.publisherPrincipal, AdministrationPrincipal: fixture.administrationPrincipal,
		PublicationRoot: state + "-publication", LegacyGenerationFloor: state}
	first, err := endpointapi.New(setup)
	if err != nil {
		t.Fatal(err)
	}
	publish(t, first, fixture, fixture.first, fixture.firstPrivate)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := endpointapi.New(setup)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	stale := admit(t, restarted, "administration", fixture.administrationPrincipal, fixture.now)
	publishedResult, err := restarted.Publish(context.Background(), endpointapi.PublicationRequest{
		Principal: fixture.administrationPrincipal, Capability: stale, Credential: fixture.first,
		InstanceSigner: fixture.firstPrivate, IntroductionAcknowledgement: acknowledgement(fixture, fixture.first), At: fixture.now})
	if err == nil || publishedResult.Class != "service target authentication failure" {
		t.Fatalf("restart forgot stale generation: result=%+v err=%v", publishedResult, err)
	}
	publish(t, restarted, fixture, fixture.second, fixture.secondPrivate)
}

func TestExactTargetServiceConnectionCarriesOpaqueBytesBothDirections(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	publisher := newPublisher(t, fixture)
	publication := publish(t, publisher, fixture, fixture.first, fixture.firstPrivate)
	client, err := endpointapi.New(endpointapi.Setup{
		NetworkID: fixture.networkID, BrokerID: [32]byte{8}, AuthorityPublic: fixture.authorityPublic,
		IntroductionPublic:  fixture.introductionPublic,
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
		result endpointapi.RuntimeResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() {
		result, runErr := publisher.Accept(ctx, endpointapi.InboundConnectionRequest{
			Principal: fixture.publisherPrincipal, Capability: publisherSession,
			Route: publisherRoute, Application: publisherEndpoint, BytesEachDirection: 64 << 10, At: fixture.now,
		})
		outcomes <- outcome{result, runErr}
	}()
	go func() {
		result, runErr := client.Connect(ctx, endpointapi.OutboundConnectionRequest{
			Principal: fixture.clientPrincipal, Capability: clientSession,
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
		if completed.result.Admission.Session == [32]byte{} || completed.result.Admission.Principal == [32]byte{} ||
			completed.result.Admission.Broker == [32]byte{} || completed.result.Admission.Grant == [32]byte{} ||
			completed.result.Admission.IssuedAt == 0 || completed.result.Admission.ExpiresAt <= completed.result.Admission.IssuedAt {
			t.Fatalf("consumed Broker receipt is invalid: result=%+v", completed.result)
		}
	}

	wrongClient, _ := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{9},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.clientPrincipal})
	wrongSession := admit(t, wrongClient, "connection", fixture.clientPrincipal, fixture.now)
	result, err := wrongClient.Connect(context.Background(), endpointapi.OutboundConnectionRequest{
		Principal: fixture.clientPrincipal, Capability: wrongSession,
		Target: [32]byte{99}, Publication: publication, At: fixture.now,
	})
	if err == nil || result.Class != "service target authentication failure" || strings.Contains(result.Reason, "route") {
		t.Fatalf("wrong Target returned dishonest result: result=%+v err=%v", result, err)
	}
}

type fixture struct {
	now                                                                time.Time
	networkID, clientPrincipal, publisherPrincipal                     [32]byte
	administrationPrincipal, hostilePrincipal                          [32]byte
	authorityPublic, introductionPublic, firstPublic, secondPublic     ed25519.PublicKey
	authorityPrivate, introductionPrivate, firstPrivate, secondPrivate ed25519.PrivateKey
	first, second                                                      endpointapi.Credential
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
	value.introductionPublic, value.introductionPrivate, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	value.firstPublic, value.firstPrivate, _ = ed25519.GenerateKey(rand.Reader)
	value.secondPublic, value.secondPrivate, _ = ed25519.GenerateKey(rand.Reader)
	value.first = issue(t, value, value.firstPublic, 1, 3)
	value.second = issue(t, value, value.secondPublic, 2, 3)
	return value
}

func issue(t *testing.T, fixture fixture, public ed25519.PublicKey, generation uint64, capabilities uint32) endpointapi.Credential {
	t.Helper()
	var authority, instance [32]byte
	copy(authority[:], fixture.authorityPublic)
	copy(instance[:], public)
	credential, err := (endpointapi.Credential{
		AuthorityPublic: authority, InstancePublic: instance, IntroductionHPKEPublic: [32]byte{8}, Generation: generation,
		NotBefore: fixture.now.Add(-time.Minute).Unix(), NotAfter: fixture.now.Add(time.Minute).Unix(),
		NetworkID: fixture.networkID, Capabilities: capabilities,
	}).Issue(fixture.authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	return credential
}

func alterCredential(t *testing.T, fixture fixture, change func(*endpointapi.Credential)) endpointapi.Credential {
	t.Helper()
	value := fixture.first
	change(&value)
	credential, err := value.Issue(fixture.authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	return credential
}

func alteredUnsigned(value endpointapi.Credential, change func(*endpointapi.Credential)) endpointapi.Credential {
	change(&value)
	return value
}

func newPublisher(t *testing.T, fixture fixture) endpointRunner {
	t.Helper()
	endpoint, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{7},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal:     fixture.publisherPrincipal,
		AdministrationPrincipal: fixture.administrationPrincipal, PublicationRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := endpoint.Close(); closeErr != nil {
			t.Errorf("close publisher: %v", closeErr)
		}
	})
	return endpoint
}

func admit(t *testing.T, endpoint endpointRunner, surface string, principal [32]byte, at time.Time) [32]byte {
	t.Helper()
	if at.IsZero() {
		t.Fatal("admit time is absent")
	}
	result, err := endpoint.Admit(principal, broker.Surface(surface))
	if err != nil || result == [32]byte{} {
		t.Fatalf("admit %s: session=%x err=%v", surface, result, err)
	}
	return result
}

func publish(t *testing.T, endpoint endpointRunner, fixture fixture, credential endpointapi.Credential, private ed25519.PrivateKey) []byte {
	t.Helper()
	session := admit(t, endpoint, "administration", fixture.administrationPrincipal, fixture.now)
	result, err := endpoint.Publish(context.Background(), endpointapi.PublicationRequest{
		Principal: fixture.administrationPrincipal, Capability: session, Credential: credential,
		InstanceSigner: private, IntroductionAcknowledgement: acknowledgement(fixture, credential), At: fixture.now})
	if err != nil || result.Class != "published" {
		t.Fatalf("publish generation %d: result=%+v err=%v", credential.Generation, result, err)
	}
	return result.Record
}

func acknowledgement(value fixture, credential endpointapi.Credential) []byte {
	body := make([]byte, 149)
	copy(body[:4], "ARIA")
	body[4] = 1
	copy(body[5:37], credential.Target[:])
	binary.BigEndian.PutUint64(body[37:45], credential.Generation)
	binary.BigEndian.PutUint64(body[45:53], uint64(credential.NotAfter))
	copy(body[53:85], credential.NetworkID[:])
	body[85] = 7
	body[117] = byte(credential.Generation)
	signature := ed25519.Sign(value.introductionPrivate,
		append([]byte("ardents-service-introduction-ack-v1\x00"), body...))
	return append(body, signature...)
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
