package credential

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/openpcc/ohttp"
)

func TestClientObtainsOnlyExactMembershipGrantThroughOHTTP(t *testing.T) {
	now := time.Unix(2_000_500_000, 0).UTC()
	issuerPublic, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorPublic, initiatorPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorCertificate := credentialCertificate(t, initiatorPrivate, 2)
	request := Request{NetworkID: credentialID(1), Digest: credentialID(2), IntroductionNodeID: credentialID(3),
		AttachmentID: credentialID(4), ClientKeyDigest: credentialID(5), Epoch: 6, NotAfter: now.Add(10 * time.Second)}
	var authorized Request
	issuer, err := NewIssuer(IssuerConfig{NetworkID: request.NetworkID, NodeID: credentialID(7), IdentityKey: issuerPrivate,
		GrantSigner: authorityPrivate, InitiatorNodeID: credentialID(8), InitiatorPublicKey: publicIdentifier(initiatorPublic),
		CurrentDuty: func() (StateDuty, bool) {
			return StateDuty{NetworkID: request.NetworkID, Digest: request.Digest, IssuerNodeID: credentialID(7),
				IssuerPublicKey: publicIdentifier(issuerPublic), InitiatorNodeID: credentialID(8), InitiatorPublicKey: publicIdentifier(initiatorPublic),
				GrantAuthorityID: sha256.Sum256(authorityPublic), Epoch: request.Epoch, NotAfter: now.Add(time.Minute)}, true
		}, Clock: func() time.Time { return now },
		Authorize: func(got Request, at time.Time) bool { authorized = got; return at.Equal(now) && got == request }})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(issuer.Handler())
	server.TLS, err = issuer.TLSConfig(credentialCertificate(t, issuerPrivate, 1))
	if err != nil {
		t.Fatal(err)
	}
	server.StartTLS()
	defer server.Close()
	var issuerKey [32]byte
	copy(issuerKey[:], issuerPublic)
	httpClient, err := HTTPClient(issuerKey, initiatorCertificate)
	if err != nil {
		t.Fatal(err)
	}
	defer httpClient.CloseIdleConnections()
	client, err := OpenClient(ClientConfig{NetworkID: request.NetworkID, IssuerPublic: issuerKey, Profile: issuer.Profile(),
		GrantAuthority: authorityPublic, Exchange: issuerExchange(httpClient, server.URL), At: now, Deadline: now.Add(15 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := client.Issue(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := route.VerifyTransitGrant(raw, authorityPublic)
	if err != nil || authorized != request || grant.NetworkID != request.NetworkID || grant.Digest != request.Digest ||
		grant.Epoch != request.Epoch || grant.TransitNodeID != request.IntroductionNodeID || grant.AttachmentID != request.AttachmentID ||
		grant.ClientKeyDigest != request.ClientKeyDigest || grant.TransitRole != route.IntroductionRole || !grant.NotAfter.Equal(request.NotAfter) {
		t.Fatalf("issued Grant = %+v, authorised = %+v, %v", grant, authorized, err)
	}
	if grant.IssuerID != sha256.Sum256(authorityPublic) || grant.GrantID == [32]byte{} {
		t.Fatalf("issued Grant authority/id = (%x, %x)", grant.IssuerID, grant.GrantID)
	}
}

func TestIssuerRejectsCallerWithoutSelectedInitiatorCertificate(t *testing.T) {
	now := time.Unix(2_000_500_000, 0).UTC()
	issuerPublic, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := NewIssuer(IssuerConfig{NetworkID: credentialID(21), NodeID: credentialID(22), IdentityKey: issuerPrivate,
		GrantSigner: authorityPrivate, InitiatorNodeID: credentialID(23), InitiatorPublicKey: publicIdentifier(initiatorPublic),
		CurrentDuty: func() (StateDuty, bool) {
			return StateDuty{NetworkID: credentialID(21), Digest: credentialID(24), IssuerNodeID: credentialID(22),
				IssuerPublicKey: publicIdentifier(issuerPublic), InitiatorNodeID: credentialID(23), InitiatorPublicKey: publicIdentifier(initiatorPublic),
				GrantAuthorityID: sha256.Sum256(authorityPublic), Epoch: 25, NotAfter: now.Add(time.Minute)}, true
		}, Clock: func() time.Time { return now }, Authorize: func(Request, time.Time) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(issuer.Handler())
	server.TLS, err = issuer.TLSConfig(credentialCertificate(t, issuerPrivate, 3))
	if err != nil {
		t.Fatal(err)
	}
	server.StartTLS()
	defer server.Close()
	request, err := http.NewRequest(http.MethodPost, server.URL, bytes.NewReader([]byte{1}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.Client().Do(request); err == nil {
		t.Fatal("issuer accepted a direct caller without the selected Initiator certificate")
	}
}

func TestCredentialRequestHasClosedTargetFreeGrammar(t *testing.T) {
	request := Request{NetworkID: credentialID(11), Digest: credentialID(12), IntroductionNodeID: credentialID(13),
		AttachmentID: credentialID(14), ClientKeyDigest: credentialID(15), Epoch: 16, NotAfter: time.Unix(2_000_500_010, 0).UTC()}
	raw, err := encodeRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 4+32*5+8+8 {
		t.Fatalf("request length = %d", len(raw))
	}
	if _, err := decodeRequest(append(raw, []byte("target-or-name")...)); err == nil {
		t.Fatal("credential request accepted a trailing Target or Name")
	}
	changed := append([]byte(nil), raw...)
	changed[4+32*2] ^= 1
	if got, err := decodeRequest(changed); err != nil || got.IntroductionNodeID == request.IntroductionNodeID {
		t.Fatalf("closed request mutation = %+v, %v", got, err)
	}
}

func issuerExchange(client *http.Client, origin string) Exchange {
	return func(ctx context.Context, envelope []byte) ([]byte, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, origin, bytes.NewReader(envelope))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", ohttp.RequestMediaType)
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != ohttp.ResponseMediaType {
			return nil, errors.New("issuer response is unavailable")
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, maximumEnvelopeSize+1))
		if err != nil || len(body) == 0 || len(body) > maximumEnvelopeSize {
			return nil, errors.New("issuer response envelope is invalid")
		}
		return body, nil
	}
}

func credentialID(marker byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = marker
	}
	return result
}

func publicIdentifier(public ed25519.PublicKey) [32]byte {
	var result [32]byte
	copy(result[:], public)
	return result
}

func credentialCertificate(t *testing.T, private ed25519.PrivateKey, serial int64) tls.Certificate {
	t.Helper()
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: time.Unix(1, 0).UTC(), NotAfter: time.Unix(2_100_000_000, 0).UTC(),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, private.Public(), private)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private, Leaf: leaf}
}
