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
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/openpcc/ohttp"
)

func TestClientObtainsOnlyExactRoleScopedGrantThroughOHTTP(t *testing.T) {
	now := time.Unix(2_000_500_000, 0).UTC()
	issuerPublic, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorPublic, initiatorPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorCertificate := credentialCertificate(t, initiatorPrivate, 2)
	request := Request{RequestID: credentialID(9), NetworkID: credentialID(1), Digest: credentialID(2), TransitNodeID: credentialID(3),
		AttachmentID: credentialID(4), ClientKeyDigest: credentialID(5), Epoch: 6, TransitRole: route.ResponderRole,
		NotAfter: now.Add(10 * time.Second)}
	issuer := openTestRootIssuer(t, filepath.Join(t.TempDir(), "issuer-root"), request.NetworkID, credentialID(7), issuerPrivate,
		credentialID(8), publicIdentifier(initiatorPublic), now.Add(time.Minute), 4, func() time.Time { return now },
		func(profile Profile, profileDigest [32]byte) (StateDuty, bool) {
			return StateDuty{NetworkID: request.NetworkID, Digest: request.Digest, IssuerNodeID: credentialID(7),
				IssuerPublicKey: publicIdentifier(issuerPublic), InitiatorNodeID: credentialID(8), InitiatorPublicKey: publicIdentifier(initiatorPublic),
				GrantSignerPublicKey: profile.GrantSignerPublicKey, ProfileDigest: profileDigest,
				Epoch: request.Epoch, NotAfter: now.Add(time.Minute)}, true
		})
	defer func() { _ = issuer.Close() }()
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
		Exchange: issuerExchange(httpClient, server.URL), At: now, Deadline: now.Add(15 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Issue(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != Issued {
		t.Fatalf("issuance outcome = %q", result.Outcome)
	}
	issuerProfile := issuer.Profile()
	grantPublic := ed25519.PublicKey(issuerProfile.GrantSignerPublicKey[:])
	grant, err := route.VerifyTransitGrant(result.Grant, grantPublic)
	if err != nil || grant.NetworkID != request.NetworkID || grant.Digest != request.Digest ||
		grant.Epoch != request.Epoch || grant.TransitNodeID != request.TransitNodeID || grant.AttachmentID != request.AttachmentID ||
		grant.ClientKeyDigest != request.ClientKeyDigest || grant.TransitRole != request.TransitRole || !grant.NotAfter.Equal(request.NotAfter) {
		t.Fatalf("issued Grant = %+v, %v", grant, err)
	}
	if grant.IssuerID != sha256.Sum256(grantPublic) || grant.GrantID == [32]byte{} {
		t.Fatalf("issued Grant authority/id = (%x, %x)", grant.IssuerID, grant.GrantID)
	}
}

func TestIssuerRejectsCallerWithoutSelectedInitiatorCertificate(t *testing.T) {
	now := time.Unix(2_000_500_000, 0).UTC()
	issuerPublic, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	issuer := openTestRootIssuer(t, filepath.Join(t.TempDir(), "issuer-root"), credentialID(21), credentialID(22), issuerPrivate,
		credentialID(23), publicIdentifier(initiatorPublic), now.Add(time.Minute), 4, func() time.Time { return now },
		func(profile Profile, profileDigest [32]byte) (StateDuty, bool) {
			return StateDuty{NetworkID: credentialID(21), Digest: credentialID(24), IssuerNodeID: credentialID(22),
				IssuerPublicKey: publicIdentifier(issuerPublic), InitiatorNodeID: credentialID(23), InitiatorPublicKey: publicIdentifier(initiatorPublic),
				GrantSignerPublicKey: profile.GrantSignerPublicKey, ProfileDigest: profileDigest,
				Epoch: 25, NotAfter: now.Add(time.Minute)}, true
		})
	defer func() { _ = issuer.Close() }()
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
	request := Request{RequestID: credentialID(10), NetworkID: credentialID(11), Digest: credentialID(12), TransitNodeID: credentialID(13),
		AttachmentID: credentialID(14), ClientKeyDigest: credentialID(15), Epoch: 16, TransitRole: route.IntroductionRole,
		NotAfter: time.Unix(2_000_500_010, 0).UTC()}
	raw, err := encodeRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 4+32*6+8+1+8 {
		t.Fatalf("request length = %d", len(raw))
	}
	if _, err := decodeRequest(append(raw, []byte("target-or-name")...)); err == nil {
		t.Fatal("credential request accepted a trailing Target or Name")
	}
	changed := append([]byte(nil), raw...)
	changed[4+32*3] ^= 1
	if got, err := decodeRequest(changed); err != nil || got.TransitNodeID == request.TransitNodeID {
		t.Fatalf("closed request mutation = %+v, %v", got, err)
	}
	changed = append([]byte(nil), raw...)
	changed[4+32*6+8] = route.RendezvousRole
	if _, err := decodeRequest(changed); err == nil {
		t.Fatal("credential request accepted an unsupported transit role")
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
