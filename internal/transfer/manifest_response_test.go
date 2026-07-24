package transfer

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	chunking "ardents/internal/content"
	model "ardents/internal/content/catalog"
	"ardents/internal/discovery"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"

	"github.com/stretchr/testify/require"
)

func TestManifestResponseBindsCompleteManifestToTrustedSource(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	encodedKey := base64.StdEncoding.EncodeToString(publicKey)
	principal, err := identityprincipal.FromPublicKey(encodedKey)
	require.NoError(t, err)
	owner, err := identityprincipal.Parse(principal)
	require.NoError(t, err)
	disc := discovery.New("")
	require.NoError(t, publishTransferTestNode(disc, principal, encodedKey, privateKey))
	trust := discovery.NewTrustEvaluator(transferTrustRegistry(t, encodedKey))
	plan, err := chunking.Plan([]string{"chunk-1", "chunk-2"}, chunking.ManifestSpec{
		Owner: owner, MediaType: "application/octet-stream", KeyID: "key-1", TotalPlaintextBytes: 2,
	})
	require.NoError(t, err)
	request := blobFetchRequest{RequestID: "manifest-request", Requester: "requester", ResourceID: plan.Root.ID, ResourceKind: "manifest", Owner: owner.String()}
	wire, err := marshalManifestResponse(principal, privateKey, request, plan.Root, nil)
	require.NoError(t, err)
	cfg := ExchangeConfig{Discovery: disc, Trust: trust}
	received, source, err := acceptManifestResponse(cfg, owner, plan.Root.ID, request.Requester, request.RequestID, wire)
	require.NoError(t, err)
	require.Equal(t, principal, source)
	require.Equal(t, plan.Root.ID, received.ID)
	require.Equal(t, plan.Root.Refs, received.Refs)
	require.NoError(t, chunking.ValidateManifest(received))
	otherOwner, err := identityprincipal.FromEd25519PublicKey(bytes.Repeat([]byte{0x42}, ed25519.PublicKeySize))
	require.NoError(t, err)
	_, _, err = acceptManifestResponse(cfg, otherOwner, plan.Root.ID, request.Requester, request.RequestID, wire)
	require.ErrorContains(t, err, "does not match request")

	var tampered blobFetchResponse
	require.NoError(t, json.Unmarshal(wire, &tampered))
	tampered.Manifest.Refs[0].ID = "attacker-selected-chunk"
	wire, err = json.Marshal(tampered)
	require.NoError(t, err)
	_, _, err = acceptManifestResponse(cfg, owner, plan.Root.ID, request.Requester, request.RequestID, wire)
	require.ErrorContains(t, err, "signature")
}

func TestManifestFromWireRejectsUntypedOwner(t *testing.T) {
	_, err := manifestFromWire(manifestWire{ID: "manifest", Kind: "blob-set", Owner: "owner"})
	require.ErrorContains(t, err, "owner is invalid")
}

func TestManifestRequestWithoutOwnerFailsClosed(t *testing.T) {
	payload, err := json.Marshal(blobFetchRequest{
		RequestID: "request", ResourceID: "root", ResourceKind: "manifest", Requester: "requester",
	})
	require.NoError(t, err)
	_, err = decodeBlobRequest(payload)
	require.ErrorContains(t, err, "owner is invalid")
}

func TestManifestCandidateErrorDoesNotCancelLaterHonestSuccess(t *testing.T) {
	firstPublic, firstPrivate, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	secondPublic, secondPrivate, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	firstEncoded := base64.StdEncoding.EncodeToString(firstPublic)
	secondEncoded := base64.StdEncoding.EncodeToString(secondPublic)
	firstID, err := identityprincipal.FromPublicKey(firstEncoded)
	require.NoError(t, err)
	secondID, err := identityprincipal.FromPublicKey(secondEncoded)
	require.NoError(t, err)
	owner, err := identityprincipal.Parse(secondID)
	require.NoError(t, err)
	disc := discovery.New("")
	require.NoError(t, publishTransferTestNode(disc, firstID, firstEncoded, firstPrivate))
	require.NoError(t, publishTransferTestNode(disc, secondID, secondEncoded, secondPrivate))
	registry, err := identitytrust.NewRegistry([]identitytrust.Entry{
		{Principal: firstID, PublicKey: firstPublic, Purposes: []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish}},
		{Principal: secondID, PublicKey: secondPublic, Purposes: []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish}},
	})
	require.NoError(t, err)
	plan, err := chunking.Plan([]string{"chunk-honest"}, chunking.ManifestSpec{
		Owner: owner, MediaType: "application/octet-stream", KeyID: "key-1", TotalPlaintextBytes: 1,
	})
	require.NoError(t, err)
	request := blobFetchRequest{
		RequestID: "manifest-race", Requester: "requester", ResourceID: plan.Root.ID,
		ResourceKind: "manifest", Owner: owner.String(),
	}
	rejection, err := marshalManifestResponse(firstID, firstPrivate, request, model.Manifest{}, errors.New("candidate rejection"))
	require.NoError(t, err)
	success, err := marshalManifestResponse(secondID, secondPrivate, request, plan.Root, nil)
	require.NoError(t, err)
	var spoofedHonestSource blobFetchResponse
	require.NoError(t, json.Unmarshal(success, &spoofedHonestSource))
	spoofedHonestSource.Signature = base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	spoofedHonestWire, err := json.Marshal(spoofedHonestSource)
	require.NoError(t, err)
	responses := make(chan []byte, 3)
	responses <- rejection
	responses <- spoofedHonestWire
	responses <- success
	history := newFetchTestData()
	transfer, err := history.Start(Record{
		ID: request.RequestID, Kind: "manifest_fetch", ResourceOwner: owner.String(),
		ResourceID: plan.Root.ID, Direction: "inbound", State: "pending",
	})
	require.NoError(t, err)

	received, err := awaitManifestResponse(context.Background(), ExchangeConfig{
		Discovery: disc, Trust: discovery.NewTrustEvaluator(registry), History: history,
	}, transfer.ID, owner, plan.Root.ID, request.Requester, request.RequestID, responses,
		trustedFetchCandidates(ExchangeConfig{
			Discovery: disc, Trust: discovery.NewTrustEvaluator(registry),
		}, request.Requester))
	require.NoError(t, err)
	require.Equal(t, plan.Root.ID, received.ID)
	require.Equal(t, secondID, history.transfers[transfer.ID].Peer)
}
