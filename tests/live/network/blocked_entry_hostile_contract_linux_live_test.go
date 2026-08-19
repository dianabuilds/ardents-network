//go:build linux && live

package network_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/planfile"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func runBlockedHostileContract(t *testing.T) {
	cell := os.Getenv("ARDENTS_HOSTILE_CELL")
	parts := strings.Split(cell, "/")
	if len(parts) != 4 {
		t.Fatal("hostile contract cell is invalid")
	}
	terminal := "bridge-local-denial"
	var before, after, receipt string
	if parts[1] == "G6-substitution" {
		before, after, receipt = exerciseBlockedBindingSubstitution(t, parts[2])
	} else if parts[1] == "G7-forbidden-path" {
		terminal = "bridge-attempt-exhausted"
		if parts[2] == "deadline-exposure-reset" {
			terminal = "bridge-deadline-exceeded"
		}
		before, after, receipt = exerciseBlockedForbiddenPath(t, parts[2], terminal)
	} else {
		t.Fatal("unsupported hostile contract group")
	}
	result := blockedHostileContractResult{Kind: "hostile-contract", Cell: cell, Terminal: terminal,
		Before: before, After: after, Receipt: receipt}
	raw, _ := json.Marshal(result)
	fmt.Println(string(raw))
}

type hostileServiceFixture struct {
	now                                               time.Time
	network, client, publisher, administrator, broker [32]byte
	authorityPrivate, introductionPrivate             ed25519.PrivateKey
	authorityPublic, introductionPublic               ed25519.PublicKey
	instancePrivate                                   ed25519.PrivateKey
	credential                                        serviceconn.Credential
	publication                                       []byte
}

func newHostileServiceFixture(t *testing.T) hostileServiceFixture {
	t.Helper()
	value := hostileServiceFixture{now: time.Now().UTC(), network: [32]byte{1}, client: [32]byte{2},
		publisher: [32]byte{3}, administrator: [32]byte{4}, broker: [32]byte{5}}
	value.authorityPrivate = ed25519.NewKeyFromSeed(bytesOf(0x31, ed25519.SeedSize))
	value.authorityPublic = value.authorityPrivate.Public().(ed25519.PublicKey)
	value.introductionPrivate = ed25519.NewKeyFromSeed(bytesOf(0x32, ed25519.SeedSize))
	value.introductionPublic = value.introductionPrivate.Public().(ed25519.PublicKey)
	value.instancePrivate = ed25519.NewKeyFromSeed(bytesOf(0x33, ed25519.SeedSize))
	var authority, instance [32]byte
	copy(authority[:], value.authorityPublic)
	copy(instance[:], value.instancePrivate.Public().(ed25519.PublicKey))
	credential, err := (serviceconn.Credential{AuthorityPublic: authority, InstancePublic: instance,
		Generation: 1, NotBefore: value.now.Add(-time.Minute).Unix(), NotAfter: value.now.Add(time.Minute).Unix(),
		NetworkID: value.network, Capabilities: 3}).Issue(value.authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	value.credential = credential
	publisher, err := serviceconn.New(serviceconn.Setup{NetworkID: value.network, BrokerID: value.broker,
		AuthorityPublic: value.authorityPublic, IntroductionPublic: value.introductionPublic,
		ConnectionPrincipal: value.publisher, AdministrationPrincipal: value.administrator})
	if err != nil {
		t.Fatal(err)
	}
	session := hostileAdmit(t, publisher, "administration", value.administrator, value.now)
	result, err := publisher.Do(context.Background(), serviceconn.Request{Action: "publish",
		Principal: value.administrator, Session: session, Credential: credential,
		InstancePrivate: value.instancePrivate, IntroductionAcknowledgement: hostileAcknowledgement(value), At: value.now})
	if err != nil || result.Class != "published" {
		t.Fatalf("publish hostile fixture: %+v %v", result, err)
	}
	value.publication = result.Publication
	return value
}

type hostileEndpoint interface {
	Do(context.Context, serviceconn.Request) (serviceconn.Result, error)
}

func hostileAdmit(t *testing.T, endpoint hostileEndpoint, surface string, principal [32]byte, at time.Time) [32]byte {
	t.Helper()
	result, err := endpoint.Do(context.Background(), serviceconn.Request{Action: "admit", Surface: surface,
		Principal: principal, At: at})
	if err != nil || result.Class != "authorized" {
		t.Fatalf("admit hostile fixture: %+v %v", result, err)
	}
	return result.Session
}

func hostileAcknowledgement(value hostileServiceFixture) []byte {
	body := make([]byte, 149)
	copy(body[:4], "ASIA")
	body[4] = 1
	copy(body[5:37], value.credential.Target[:])
	binary.BigEndian.PutUint64(body[37:45], value.credential.Generation)
	binary.BigEndian.PutUint64(body[45:53], uint64(value.credential.NotAfter))
	copy(body[53:85], value.credential.NetworkID[:])
	copy(body[85:117], value.broker[:])
	body[117] = 1
	return append(body, ed25519.Sign(value.introductionPrivate,
		append([]byte("ardents-h3-introduction-ack-v1\x00"), body...))...)
}

func exerciseBlockedBindingSubstitution(t *testing.T, variant string) (string, string, string) {
	fixture := newHostileServiceFixture(t)
	client, err := serviceconn.New(serviceconn.Setup{NetworkID: fixture.network, BrokerID: [32]byte{6},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.client})
	if err != nil {
		t.Fatal(err)
	}
	session := hostileAdmit(t, client, "connection", fixture.client, fixture.now)
	publication := append([]byte(nil), fixture.publication...)
	target := fixture.credential.Target
	binding := serviceconn.Recovery{NetworkID: fixture.network, CandidateView: [32]byte{7},
		IsolationContext: [32]byte{8}, DestinationBinding: [32]byte{9}, RouteProfile: "h3-route-tracer-v1",
		WorkSafetyNotAfter: fixture.now.Add(40 * time.Second).Unix(),
		WorkSafetyMaximum:  fixture.now.Add(50 * time.Second).Unix(),
		NoNewRecoveryAfter: fixture.now.Add(30 * time.Second).Unix()}
	before := hostileBindingDigest(publication, target, binding)
	wantClass := "service target authentication failure"
	switch variant {
	case "target":
		target[0]++
	case "instance-generation":
		publication[42]++
	case "isolation-context":
		binding.IsolationContext = [32]byte{}
		wantClass = "local authorization or policy denial"
	case "route-generation":
		binding.CandidateView = [32]byte{}
		wantClass = "local authorization or policy denial"
	case "attachment":
		binding.DestinationBinding = [32]byte{}
		wantClass = "local authorization or policy denial"
	case "application-canary":
		publication[len(publication)-1]++
	default:
		t.Fatalf("unsupported G6 variant %q", variant)
	}
	route, routePeer := net.Pipe()
	application, applicationPeer := net.Pipe()
	defer routePeer.Close()
	defer applicationPeer.Close()
	result, runErr := client.Do(context.Background(), serviceconn.Request{Action: "connect", Principal: fixture.client,
		Session: session, Target: target, Publication: publication, RecoveryBinding: binding, Route: route,
		Application: application, BytesEachDirection: 1, At: fixture.now,
		OpenAttachment: func(context.Context, serviceconn.Recovery) (net.Conn, error) {
			return nil, errors.New("unexpected reattachment")
		}})
	if runErr == nil || result.Class != wantClass {
		t.Fatalf("G6 %s accepted substitution: %+v %v", variant, result, runErr)
	}
	after := hostileBindingDigest(publication, target, binding)
	return before, after, result.Class + ":" + runErr.Error()
}

func hostileBindingDigest(publication []byte, target [32]byte, binding serviceconn.Recovery) string {
	raw, _ := json.Marshal(binding)
	value := append(append(append([]byte(nil), publication...), target[:]...), raw...)
	return hex.EncodeToString(sha256Sum(value))
}

func exerciseBlockedForbiddenPath(t *testing.T, variant, terminal string) (string, string, string) {
	challenge, ok := finalForbiddenPathChallenge(variant)
	if !ok {
		t.Fatalf("unsupported G7 variant %q", variant)
	}
	prepareBlockedState(t, "bridge-network", "bridge-network")
	prepareBlockedState(t, "local-roles", "local-roles")
	runBlockedCommand(t, "/usr/local/bin/ardents-bridge", "import", "/run/secure/import.json")
	timeline := startBlockedTimeline(t)
	transition, err := os.ReadFile("/run/secure/transition.bin")
	if err != nil {
		t.Fatal(err)
	}
	transition = stampBlockedTransition(t, transition, timeline)
	var entry struct {
		RouteManifestDigest string `json:"route_manifest_digest"`
	}
	if err := planfile.Decode("/run/secure/entry.json", 32<<10, &entry); err != nil {
		t.Fatal(err)
	}
	var manifest [32]byte
	if err := planfile.FixedHex(entry.RouteManifestDigest, manifest[:]); err != nil {
		t.Fatal(err)
	}
	owner, closeOwner := openBlockedBridgeOwner(t, "/run/secure/import.json", time.Now)
	defer func() {
		if err := closeOwner(); err != nil {
			t.Error(err)
		}
	}()
	identity, candidate, contactErr := owner.Contact()
	if contactErr != nil || identity == ([32]byte{}) || len(candidate) == 0 {
		t.Fatalf("read manifest-bound G7 contact: %v", contactErr)
	}
	deadline := time.Now().Add(time.Minute)
	if terminal == "bridge-deadline-exceeded" {
		deadline = time.Now().Add(100 * time.Millisecond)
	}
	calls := 0
	_, cleanup, acquireErr := owner.Acquire(context.Background(), transition, manifest, deadline,
		func(_ context.Context, gotIdentity [32]byte, gotCandidate []byte, contactDeadline time.Time) (net.Conn, func() error, bool, error) {
			calls++
			if gotIdentity != identity || !bytes.Equal(gotCandidate, candidate) || contactDeadline.After(deadline) {
				t.Fatal("Bridge left the committed candidate or extended its deadline")
			}
			return nil, func() error { return nil }, true, errors.New("manifest-bound candidate refused")
		})
	if cleanup != nil {
		_ = cleanup()
	}
	if acquireErr == nil || !strings.Contains(acquireErr.Error(), terminal) || calls == 0 || calls > 2 {
		t.Fatalf("G7 %s calls=%d terminal=%v", variant, calls, acquireErr)
	}
	evidence, evidenceErr := owner.Evidence()
	if evidenceErr != nil || evidence.ContactStarts != uint8(calls) || evidence.DeadlineOffset == 0 {
		t.Fatalf("G7 durable evidence=%+v err=%v", evidence, evidenceErr)
	}
	receipt, _ := json.Marshal(finalForbiddenPathReceipt{Schema: "ardents-h3-g7-receipt-v2", Variant: variant,
		Source: challenge.Source, Component: challenge.Component, Calls: uint16(calls),
		ContactStarts: uint16(evidence.ContactStarts), Terminal: terminal, DeadlineOffset: evidence.DeadlineOffset})
	return hostileBindingDigest(receipt, identity, serviceconn.Recovery{}),
		hostileBindingDigest(candidate, identity, serviceconn.Recovery{}), string(receipt)
}

func bytesOf(value byte, count int) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}

func sha256Sum(value []byte) []byte {
	digest := sha256.Sum256(value)
	return digest[:]
}
