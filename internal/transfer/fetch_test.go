package transfer

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	model "ardents/internal/content/catalog"
	"ardents/internal/content/payload"
	"ardents/internal/discovery"
	identityprincipal "ardents/internal/identity/principal"

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
	d.blobs[blob.ID] = blob
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

	trust := discovery.NewTrustEvaluator()
	trust.Trust(sourcePublicKey)
	store := newFetchTestData()

	requestID := "req-1"
	requester := "p_requester"
	blobID := "blob-1"
	forged, err := json.Marshal(blobFetchResponse{
		RequestID: requestID,
		Requester: requester,
		BlobID:    blobID,
		Status:    blobFetchStatusOK,
		Blob: model.Blob{
			ID:        blobID,
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

	trust := discovery.NewTrustEvaluator()
	trust.Trust(sourcePublicKey)
	store := newFetchTestData()

	requestID := "req-err"
	requester := "p_requester"
	blobID := "blob-err"
	wire, err := marshalBlobErrorResponse(sourcePrincipal, privateKey, blobFetchRequest{
		RequestID: requestID,
		BlobID:    blobID,
		Requester: requester,
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

	trust := discovery.NewTrustEvaluator()
	trust.Trust(sourcePublicKey)
	store := newFetchTestData()

	requestID := "req-2"
	requester := "p_requester"
	blobID := "blob.logical"
	wire, err := marshalBlobResponse(sourcePrincipal, privateKey, blobFetchRequest{
		RequestID: requestID,
		BlobID:    blobID,
		Requester: requester,
	}, model.Blob{
		ID:        blobID,
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
	blobID := "blob-timeout"
	wire, err := marshalBlobResponse(sourcePrincipal, privateKey, blobFetchRequest{
		RequestID: requestID,
		BlobID:    blobID,
		Requester: requester,
	}, model.Blob{
		ID:        blobID,
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
		Trust:     discovery.NewTrustEvaluator(),
		Data:      store,
		History:   store,
	}, transfer.ID, blobID, requester, requestID, responses)
	require.Falsef(t, err == nil || err.
		Error() !=
		"remote source is not trusted", "error = %v, want explicit remote trust rejection", err)

}
