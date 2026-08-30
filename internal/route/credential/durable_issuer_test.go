package credential

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
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
	grantPublic, grantPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorPublic, initiatorPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	request := Request{RequestID: credentialID(40), NetworkID: credentialID(41), Digest: credentialID(42),
		IntroductionNodeID: credentialID(43), AttachmentID: credentialID(44), ClientKeyDigest: credentialID(45),
		Epoch: 46, NotAfter: now.Add(10 * time.Second)}
	withdrawn := false
	duty := func() (StateDuty, bool) {
		return StateDuty{NetworkID: request.NetworkID, Digest: request.Digest, IssuerNodeID: credentialID(47),
			IssuerPublicKey: publicIdentifier(issuerPublic), InitiatorNodeID: credentialID(48),
			InitiatorPublicKey: publicIdentifier(initiatorPublic), GrantSignerPublicKey: publicIdentifier(grantPublic),
			Epoch: request.Epoch, NotAfter: now.Add(time.Minute), Withdrawn: withdrawn}, true
	}
	root := t.TempDir()
	initiatorCertificate := credentialCertificate(t, initiatorPrivate, 10)

	start := func(create bool) (*Issuer, *httptest.Server, *http.Client) {
		t.Helper()
		issuer, openErr := NewIssuer(IssuerConfig{NetworkID: request.NetworkID, NodeID: credentialID(47), IdentityKey: issuerPrivate,
			GrantSigner: grantPrivate, InitiatorNodeID: credentialID(48), InitiatorPublicKey: publicIdentifier(initiatorPublic),
			DutyRoot: root, CreateDutyRoot: create, Budget: 1, CurrentDuty: duty, Clock: func() time.Time { return now },
			Authorize: func(Request, time.Time) bool { return true }})
		if openErr != nil {
			t.Fatal(openErr)
		}
		if issuer.Profile().GrantSignerPublicKey != publicIdentifier(grantPublic) ||
			issuer.Profile().GrantSignerID != sha256.Sum256(grantPublic) {
			t.Fatal("issuer profile did not bind the purpose-scoped Grant signer")
		}
		server := httptest.NewUnstartedServer(issuer.Handler())
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

	issuer, server, httpClient := start(true)
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

	issuer, server, httpClient = start(false)
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

	fresh := request
	fresh.RequestID = credentialID(49)
	fresh.AttachmentID = credentialID(50)
	if result := issue(issuer, server, httpClient, fresh); result.Outcome != Exhausted || len(result.Grant) != 0 {
		t.Fatalf("fresh request after budget = %q, %d bytes", result.Outcome, len(result.Grant))
	}

	conflict := request
	conflict.AttachmentID = credentialID(51)
	if result := issue(issuer, server, httpClient, conflict); result.Outcome != Unavailable || len(result.Grant) != 0 {
		t.Fatalf("Request ID conflict = %q, %d bytes", result.Outcome, len(result.Grant))
	}

	withdrawn = true
	withdrawnRequest := fresh
	withdrawnRequest.RequestID = credentialID(52)
	if result := issue(issuer, server, httpClient, withdrawnRequest); result.Outcome != Withdrawn || len(result.Grant) != 0 {
		t.Fatalf("withdrawn duty = %q, %d bytes", result.Outcome, len(result.Grant))
	}
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
