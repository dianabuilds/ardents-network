package transfer

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	model "ardents/internal/content/catalog"
	"ardents/internal/content/payload"
	"ardents/internal/discovery"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"

	"github.com/stretchr/testify/require"
)

type fetchTestData struct {
	DataExchange
	History
	blobs     map[string]model.Blob
	transfers map[string]Record
}

func newFetchTestData() *fetchTestData {
	return &fetchTestData{blobs: map[string]model.Blob{}, transfers: map[string]Record{}}
}

func (d *fetchTestData) GetBlob(id string) (model.Blob, bool) {
	blob, ok := d.blobs[id]
	return blob, ok
}

func (d *fetchTestData) StoreBlob(blob model.Blob, raw []byte) (model.Blob, error) {
	hash, blobCID, err := payload.DeriveIdentity(raw)
	if err != nil {
		return model.Blob{}, err
	}
	if err := payload.ApplyDerivedIdentity(&blob, hash, blobCID); err != nil {
		return model.Blob{}, err
	}
	d.blobs[blob.Reference.String()] = blob
	return blob, nil
}

func (d *fetchTestData) ObserveBlobSource(_ string, source model.BlobSourceRecord) (model.BlobSourceRecord, error) {
	return source, nil
}

func (d *fetchTestData) Start(record Record) (Record, error) {
	d.transfers[record.ID] = record
	return record, nil
}

func (d *fetchTestData) Fail(id, peer, reason string) (Record, error) {
	record := d.transfers[id]
	record.State, record.Peer, record.Reason = "failed", peer, reason
	d.transfers[id] = record
	return record, nil
}

func TestAcceptBlobResponseRejectsUnsignedSpoofedSource(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "generate key: %v", err)

	sourcePublicKey := base64.StdEncoding.EncodeToString(publicKey)
	sourcePrincipal, err := identityprincipal.FromPublicKey(sourcePublicKey)
	require.NoErrorf(t, err, "principal from public key: %v", err)

	disc := discovery.New("")
	{
		err := publishTransferTestNode(disc, sourcePrincipal, sourcePublicKey, privateKey)
		require.NoErrorf(t, err, "publish node: %v", err)
	}

	trust := discovery.NewTrustEvaluator(transferTrustRegistry(t, sourcePublicKey))
	store := newFetchTestData()

	requestID := "req-1"
	requester := "p_requester"
	blobID := transferTestContentReference(t, "blob-1").String()
	forged, err := json.Marshal(blobFetchResponse{
		RequestID: requestID,
		Requester: requester,
		Status:    blobFetchStatusOK,
		Blob: &model.Blob{
			Reference: transferTestContentReference(t, "blob-1"),
			MediaType: "application/octet-stream",
			Size:      int64(len("forged payload")),
		},
		Payload: base64.StdEncoding.EncodeToString([]byte("forged payload")),
		Source:  sourcePrincipal,
	})
	require.NoErrorf(t, err, "marshal forged response: %v", err)

	cfg := ExchangeConfig{
		Discovery: disc,
		Trust:     trust,
		Data:      store,
	}
	{
		_, _, _, err := acceptBlobResponse(cfg, blobID, requester, requestID, forged)
		require.Error(t, err, "expected unsigned spoofed response to be rejected")
	}
	{

		_, ok := store.GetBlob(blobID)
		require.False(t, ok, "expected spoofed response to keep blob unavailable locally")
	}

}

func TestBlobFetchWireRejectsObsoleteContentIdentityFields(t *testing.T) {
	reference := transferTestContentReference(t, "wire-reference")
	request := []byte(`{"request_id":"request","blob_id":"` + reference.String() + `","requester":"requester","public_key":"key","signature":"signature"}`)
	_, err := decodeBlobRequest(request)
	require.ErrorContains(t, err, "unknown field")

	response := blobFetchResponse{
		RequestID: "request", Requester: "requester", Status: blobFetchStatusOK,
		Blob: &model.Blob{Reference: reference}, Source: "source",
	}
	raw, err := json.Marshal(response)
	require.NoError(t, err)
	for _, mutate := range []func(string) string{
		func(value string) string {
			return strings.Replace(value, `"requester"`, `"blob_id":"`+reference.String()+`","requester"`, 1)
		},
		func(value string) string { return strings.Replace(value, `"reference"`, `"id"`, 1) },
		func(value string) string { return strings.Replace(value, `"reference"`, `"cid"`, 1) },
	} {
		_, err := decodeBlobResponse([]byte(mutate(string(raw))), reference.String(), "requester", "request")
		require.Error(t, err)
	}
}

func TestAcceptBlobResponseReturnsSignedTerminalError(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "generate key: %v", err)

	sourcePublicKey := base64.StdEncoding.EncodeToString(publicKey)
	sourcePrincipal, err := identityprincipal.FromPublicKey(sourcePublicKey)
	require.NoErrorf(t, err, "principal from public key: %v", err)

	disc := discovery.New("")
	{
		err := publishTransferTestNode(disc, sourcePrincipal, sourcePublicKey, privateKey)
		require.NoErrorf(t, err, "publish node: %v", err)
	}

	trust := discovery.NewTrustEvaluator(transferTrustRegistry(t, sourcePublicKey))
	store := newFetchTestData()

	requestID := "req-err"
	requester := "p_requester"
	blobID := transferTestContentReference(t, "blob-err").String()
	wire, err := marshalBlobErrorResponse(sourcePrincipal, privateKey, blobFetchRequest{
		RequestID:  requestID,
		ResourceID: blobID,
		Requester:  requester,
	}, errors.New("plaintext blob re-serve is not allowed"))
	require.NoErrorf(t, err, "marshal error response: %v", err)

	cfg := ExchangeConfig{
		Discovery: disc,
		Trust:     trust,
		Data:      store,
	}
	{
		_, _, _, err := acceptBlobResponse(cfg, blobID, requester, requestID, wire)
		require.Falsef(t, err == nil || err.
			Error() !=
			"plaintext blob re-serve is not allowed", "error = %v, want signed terminal denial", err)
	}
	{

		_, ok := store.GetBlob(blobID)
		require.False(t, ok, "expected denied response to keep blob unavailable locally")
	}

}

func TestAcceptBlobResponseRejectsSignedMismatchedContentIdentity(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "generate key: %v", err)

	sourcePublicKey := base64.StdEncoding.EncodeToString(publicKey)
	sourcePrincipal, err := identityprincipal.FromPublicKey(sourcePublicKey)
	require.NoErrorf(t, err, "principal from public key: %v", err)

	disc := discovery.New("")
	{
		err := publishTransferTestNode(disc, sourcePrincipal, sourcePublicKey, privateKey)
		require.NoErrorf(t, err, "publish node: %v", err)
	}

	trust := discovery.NewTrustEvaluator(transferTrustRegistry(t, sourcePublicKey))
	store := newFetchTestData()

	requestID := "req-2"
	requester := "p_requester"
	blobID := transferTestContentReference(t, "blob.logical").String()
	wire, err := marshalBlobResponse(sourcePrincipal, privateKey, blobFetchRequest{
		RequestID:  requestID,
		ResourceID: blobID,
		Requester:  requester,
	}, model.Blob{
		Reference: transferTestContentReference(t, "blob.logical"),
		MediaType: "application/octet-stream",
	}, []byte("payload bound to another cid"))
	require.NoErrorf(t, err, "marshal signed response: %v", err)

	cfg := ExchangeConfig{
		Discovery: disc,
		Trust:     trust,
		Data:      store,
	}
	{
		_, _, _, err := acceptBlobResponse(cfg, blobID, requester, requestID, wire)
		require.Error(t, err, "expected mismatched content identity to be rejected")
	}
	{

		_, ok := store.GetBlob(blobID)
		require.False(t, ok, "expected mismatched response to keep blob unavailable locally")
	}

}

func transferTrustRegistry(t *testing.T, encodedPublic string) *identitytrust.Registry {
	t.Helper()
	public, err := base64.StdEncoding.DecodeString(encodedPublic)
	require.NoError(t, err)
	principalID, err := identityprincipal.FromEd25519PublicKey(ed25519.PublicKey(public))
	require.NoError(t, err)
	registry, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: principalID.String(), PublicKey: ed25519.PublicKey(public),
		Purposes: []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish},
	}})
	require.NoError(t, err)
	return registry
}

func TestAwaitBlobFetchResponseReturnsCandidateRejectionInsteadOfTimeout(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "generate key: %v", err)

	sourcePublicKey := base64.StdEncoding.EncodeToString(publicKey)
	sourcePrincipal, err := identityprincipal.FromPublicKey(sourcePublicKey)
	require.NoErrorf(t, err, "principal from public key: %v", err)

	disc := discovery.New("")
	{
		err := publishTransferTestNode(disc, sourcePrincipal, sourcePublicKey, privateKey)
		require.NoErrorf(t, err, "publish node: %v", err)
	}

	store := newFetchTestData()

	requestID := "req-timeout"
	requester := "p_requester"
	blobID := transferTestContentReference(t, "blob-timeout").String()
	wire, err := marshalBlobResponse(sourcePrincipal, privateKey, blobFetchRequest{
		RequestID:  requestID,
		ResourceID: blobID,
		Requester:  requester,
	}, model.Blob{
		Reference: transferTestContentReference(t, "blob-timeout"),
		MediaType: "application/octet-stream",
	}, []byte("network payload"))
	require.NoErrorf(t, err, "marshal response: %v", err)

	responses := make(chan []byte, 1)
	responses <- wire
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	transfer, err := store.Start(Record{
		ID: "xfer-timeout", Kind: "blob_fetch", ResourceID: blobID, Direction: "inbound", State: "pending",
	})
	require.NoError(t, err)

	_, err = awaitBlobFetchResponse(ctx, ExchangeConfig{
		Discovery: disc,
		Trust:     discovery.NewTrustEvaluator(nil),
		Data:      store,
		History:   store,
	}, transfer.ID, blobID, requester, requestID, responses)
	require.Falsef(t, err == nil || err.
		Error() !=
		"remote source is not trusted", "error = %v, want explicit remote trust rejection", err)

}
