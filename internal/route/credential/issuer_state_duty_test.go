package credential

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestIssuerRejectsChangedOrUnavailableDutyBeforeLedgerAccess(t *testing.T) {
	now := time.Unix(2_000_800_000, 0).UTC()
	issuerPublic, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorPublic, initiatorPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	request := Request{RequestID: credentialID(90), NetworkID: credentialID(91), Digest: credentialID(92),
		TransitNodeID: credentialID(93), AttachmentID: credentialID(94), ClientKeyDigest: credentialID(95),
		Epoch: 96, TransitRole: route.IntroductionRole, NotAfter: now.Add(10 * time.Second)}
	root := filepath.Join(t.TempDir(), "issuer-root")
	var dutyMutation func(*StateDuty)
	issuer := openTestRootIssuer(t, root, request.NetworkID, credentialID(97), issuerPrivate,
		credentialID(98), publicIdentifier(initiatorPublic), now.Add(time.Minute), 8, func() time.Time { return now },
		func(profile Profile, profileDigest [32]byte) (StateDuty, bool) {
			duty := StateDuty{Generation: testStateGeneration, NetworkID: request.NetworkID, Digest: request.Digest,
				IssuerNodeID: credentialID(97), IssuerPublicKey: publicIdentifier(issuerPublic),
				InitiatorNodeID: credentialID(98), InitiatorPublicKey: publicIdentifier(initiatorPublic),
				GrantSignerPublicKey: profile.GrantSignerPublicKey, ProfileDigest: profileDigest,
				Epoch: request.Epoch, NotAfter: now.Add(time.Minute), Fresh: true}
			if dutyMutation != nil {
				dutyMutation(&duty)
			}
			return duty, true
		})
	defer func() { _ = issuer.Close() }()

	server := httptest.NewUnstartedServer(issuer.Handler())
	server.TLS, err = issuer.TLSConfig(credentialCertificate(t, issuerPrivate, 20))
	if err != nil {
		t.Fatal(err)
	}
	server.StartTLS()
	defer server.Close()
	httpClient, err := HTTPClient(publicIdentifier(issuerPublic), credentialCertificate(t, initiatorPrivate, 21))
	if err != nil {
		t.Fatal(err)
	}
	defer httpClient.CloseIdleConnections()
	issue := func(input Request) Result {
		t.Helper()
		client, openErr := OpenClient(ClientConfig{NetworkID: input.NetworkID, IssuerPublic: publicIdentifier(issuerPublic),
			Profile: issuer.Profile(), Exchange: issuerExchange(httpClient, server.URL), At: now, Deadline: now.Add(15 * time.Second)})
		if openErr != nil {
			t.Fatal(openErr)
		}
		result, issueErr := client.Issue(context.Background(), input)
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		return result
	}

	find, reserve, withdraw := issuer.find, issuer.reserve, issuer.withdraw
	findCalls, reserveCalls, withdrawCalls := 0, 0, 0
	issuer.find = func(scope issuerScope, requestID, requestDigest [32]byte) ([32]byte, bool, error) {
		findCalls++
		return find(scope, requestID, requestDigest)
	}
	issuer.reserve = func(scope issuerScope, requestID, requestDigest, grantID [32]byte) ([32]byte, bool, error) {
		reserveCalls++
		return reserve(scope, requestID, requestDigest, grantID)
	}
	issuer.withdraw = func(scope issuerScope) error {
		withdrawCalls++
		return withdraw(scope)
	}

	cases := []struct {
		name          string
		mutateDuty    func(*StateDuty)
		mutateRequest func(*Request)
	}{
		{name: "generation", mutateDuty: func(duty *StateDuty) {
			duty.Generation = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
		{name: "digest", mutateDuty: func(duty *StateDuty) { duty.Digest = credentialID(100) },
			mutateRequest: func(input *Request) { input.Digest = credentialID(100) }},
		{name: "epoch", mutateDuty: func(duty *StateDuty) { duty.Epoch = 101 },
			mutateRequest: func(input *Request) { input.Epoch = 101 }},
		{name: "deadline", mutateDuty: func(duty *StateDuty) { duty.NotAfter = now.Add(2 * time.Minute) }},
		{name: "stale", mutateDuty: func(duty *StateDuty) { duty.Fresh = false }},
		{name: "conflicting", mutateDuty: func(duty *StateDuty) { duty.Conflicting = true }},
	}
	for index, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			dutyMutation = testCase.mutateDuty
			input := request
			input.RequestID[0] = byte(index + 101)
			input.AttachmentID[0] = byte(index + 111)
			if testCase.mutateRequest != nil {
				testCase.mutateRequest(&input)
			}
			before := issuerRootBytes(t, root)
			if result := issue(input); result.Outcome != Unavailable || len(result.Grant) != 0 {
				t.Fatalf("changed duty outcome = %q, %d bytes", result.Outcome, len(result.Grant))
			}
			if after := issuerRootBytes(t, root); string(after) != string(before) {
				t.Fatal("changed duty mutated the predecessor issuer root")
			}
			if findCalls != 0 || reserveCalls != 0 || withdrawCalls != 0 {
				t.Fatalf("changed duty touched ledger: find=%d reserve=%d withdraw=%d", findCalls, reserveCalls, withdrawCalls)
			}
		})
	}
}
