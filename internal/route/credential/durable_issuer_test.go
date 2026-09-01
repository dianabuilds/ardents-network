package credential

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestIssuerReconcilesOneDurableBudgetUnitAcrossRestart(t *testing.T) {
	now := time.Unix(2_000_600_000, 0).UTC()
	issuerPublic, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorPublic, initiatorPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	request := Request{RequestID: credentialID(40), NetworkID: credentialID(41), Digest: credentialID(42),
		TransitNodeID: credentialID(43), AttachmentID: credentialID(44), ClientKeyDigest: credentialID(45),
		Epoch: 46, TransitRole: route.IntroductionRole, NotAfter: now.Add(10 * time.Second)}
	withdrawn := false
	generation := testStateGeneration
	root := t.TempDir()
	initiatorCertificate := credentialCertificate(t, initiatorPrivate, 10)

	start := func() (*Issuer, *httptest.Server, *http.Client) {
		t.Helper()
		issuer := openTestRootIssuer(t, root, request.NetworkID, credentialID(47), issuerPrivate,
			credentialID(48), publicIdentifier(initiatorPublic), now.Add(time.Minute), 2, func() time.Time { return now },
			func(profile Profile, profileDigest [32]byte) (StateDuty, bool) {
				return StateDuty{Generation: generation, NetworkID: request.NetworkID, Digest: request.Digest, IssuerNodeID: credentialID(47),
					IssuerPublicKey: publicIdentifier(issuerPublic), InitiatorNodeID: credentialID(48),
					InitiatorPublicKey: publicIdentifier(initiatorPublic), GrantSignerPublicKey: profile.GrantSignerPublicKey,
					ProfileDigest: profileDigest, Epoch: request.Epoch, NotAfter: now.Add(time.Minute), Fresh: true, Withdrawn: withdrawn}, true
			})
		profile := issuer.Profile()
		grantPublic := ed25519.PublicKey(profile.GrantSignerPublicKey[:])
		if profile.GrantSignerID != sha256.Sum256(grantPublic) {
			t.Fatal("issuer profile did not bind the purpose-scoped Grant signer")
		}
		server := httptest.NewUnstartedServer(issuer.Handler())
		var openErr error
		server.TLS, openErr = issuer.TLSConfig(credentialCertificate(t, issuerPrivate, 11))
		if openErr != nil {
			t.Fatal(openErr)
		}
		server.StartTLS()
		var issuerKey [32]byte
		copy(issuerKey[:], issuerPublic)
		httpClient, openErr := HTTPClient(issuerKey, initiatorCertificate)
		if openErr != nil {
			t.Fatal(openErr)
		}
		return issuer, server, httpClient
	}
	issue := func(issuer *Issuer, server *httptest.Server, httpClient *http.Client, input Request) Result {
		t.Helper()
		var issuerKey [32]byte
		copy(issuerKey[:], issuerPublic)
		client, openErr := OpenClient(ClientConfig{NetworkID: input.NetworkID, IssuerPublic: issuerKey, Profile: issuer.Profile(),
			Exchange: issuerExchange(httpClient, server.URL), At: now, Deadline: now.Add(15 * time.Second)})
		if openErr != nil {
			t.Fatal(openErr)
		}
		result, issueErr := client.Issue(context.Background(), input)
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		return result
	}

	issuer, server, httpClient := start()
	openedProfile := issuer.Profile()
	grantPublic := ed25519.PublicKey(openedProfile.GrantSignerPublicKey[:])
	firstProfile, err := EncodeProfile(issuer.Profile())
	if err != nil {
		t.Fatal(err)
	}
	first := issue(issuer, server, httpClient, request)
	server.Close()
	httpClient.CloseIdleConnections()
	if err := issuer.Close(); err != nil {
		t.Fatal(err)
	}
	if first.Outcome != Issued || len(first.Grant) == 0 {
		t.Fatalf("first outcome = %q, %d bytes", first.Outcome, len(first.Grant))
	}
	firstGrant, err := route.VerifyTransitGrant(first.Grant, grantPublic)
	if err != nil || firstGrant.IssuerID != sha256.Sum256(grantPublic) {
		t.Fatalf("first purpose-scoped Grant = %+v, %v", firstGrant, err)
	}

	issuer, server, httpClient = start()
	defer server.Close()
	defer httpClient.CloseIdleConnections()
	defer func() {
		if err := issuer.Close(); err != nil {
			t.Error(err)
		}
	}()
	restartedProfile, err := EncodeProfile(issuer.Profile())
	if err != nil {
		t.Fatal(err)
	}
	if string(restartedProfile) != string(firstProfile) {
		t.Fatal("issuer restart changed its State-authenticated OHTTP profile")
	}
	reconciled := issue(issuer, server, httpClient, request)
	if reconciled.Outcome != Issued || string(reconciled.Grant) != string(first.Grant) {
		t.Fatalf("reconciled outcome changed: %q, same Grant = %t", reconciled.Outcome, string(reconciled.Grant) == string(first.Grant))
	}

	beforeSuccessor := issuerRootBytes(t, root)
	generation = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
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
	successor := request
	successor.RequestID = credentialID(56)
	successor.AttachmentID = credentialID(57)
	if result := issue(issuer, server, httpClient, successor); result.Outcome != Unavailable || len(result.Grant) != 0 {
		t.Fatalf("generation-only State successor outcome = %q, %d bytes", result.Outcome, len(result.Grant))
	}
	if afterSuccessor := issuerRootBytes(t, root); string(afterSuccessor) != string(beforeSuccessor) {
		t.Fatal("generation-only State successor mutated the issuer root")
	}
	if findCalls != 0 || reserveCalls != 0 || withdrawCalls != 0 {
		t.Fatalf("generation-only State successor touched ledger: find=%d reserve=%d withdraw=%d", findCalls, reserveCalls, withdrawCalls)
	}
	issuer.find, issuer.reserve, issuer.withdraw = find, reserve, withdraw
	generation = testStateGeneration

	fresh := request
	fresh.RequestID = credentialID(49)
	fresh.AttachmentID = credentialID(50)
	fresh.TransitNodeID = credentialID(53)
	fresh.TransitRole = route.ResponderRole
	responder := issue(issuer, server, httpClient, fresh)
	responderGrant, verifyErr := route.VerifyTransitGrant(responder.Grant, grantPublic)
	if responder.Outcome != Issued || verifyErr != nil || responderGrant.TransitRole != route.ResponderRole ||
		responderGrant.TransitNodeID != fresh.TransitNodeID || string(responder.Grant) == string(first.Grant) {
		t.Fatalf("shared-budget Responder outcome = %q, Grant = %+v, err = %v", responder.Outcome, responderGrant, verifyErr)
	}

	exhausted := fresh
	exhausted.RequestID = credentialID(54)
	exhausted.AttachmentID = credentialID(55)
	if result := issue(issuer, server, httpClient, exhausted); result.Outcome != Exhausted || len(result.Grant) != 0 {
		t.Fatalf("fresh request after budget = %q, %d bytes", result.Outcome, len(result.Grant))
	}

	conflict := request
	conflict.TransitRole = route.ResponderRole
	if result := issue(issuer, server, httpClient, conflict); result.Outcome != Unavailable || len(result.Grant) != 0 {
		t.Fatalf("Request ID conflict = %q, %d bytes", result.Outcome, len(result.Grant))
	}

	withdrawn = true
	withdrawnRequest := exhausted
	withdrawnRequest.RequestID = credentialID(52)
	if result := issue(issuer, server, httpClient, withdrawnRequest); result.Outcome != Withdrawn || len(result.Grant) != 0 {
		t.Fatalf("withdrawn duty = %q, %d bytes", result.Outcome, len(result.Grant))
	}
}

func issuerRootBytes(t *testing.T, root string) []byte {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var result []byte
	for _, entry := range entries {
		if entry.Name() == issuerRootLockName {
			continue
		}
		if entry.IsDir() {
			t.Fatalf("issuer root unexpectedly contains directory %q", entry.Name())
		}
		raw, readErr := os.ReadFile(filepath.Join(root, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		result = append(result, entry.Name()...)
		result = append(result, 0)
		result = append(result, raw...)
		result = append(result, 0)
	}
	return result
}

func TestCredentialOutcomePlaintextHasOneFixedSize(t *testing.T) {
	grant := make([]byte, 332)
	for index := range grant {
		grant[index] = byte(index + 1)
	}
	for _, result := range []Result{{Outcome: Issued, Grant: grant}, {Outcome: Exhausted}, {Outcome: Withdrawn}, {Outcome: Unavailable}} {
		raw, err := encodeResponse(result)
		if err != nil {
			t.Fatal(err)
		}
		fixed, err := pad(raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(fixed) != messageSize {
			t.Fatalf("%q fixed response length = %d", result.Outcome, len(fixed))
		}
	}
}
