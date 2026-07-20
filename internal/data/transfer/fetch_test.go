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

	appdata "ardents/internal/data"
	discovery "ardents/internal/discovery"
	identityapi "ardents/internal/identity/api"
	publication "ardents/internal/publication"

	"github.com/stretchr/testify/require"
)

func TestAcceptBlobResponseRejectsUnsignedSpoofedSource(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "generate key: %v", err)

	sourcePublicKey := base64.StdEncoding.EncodeToString(publicKey)
	sourcePrincipal, err := identityapi.PrincipalFromPublicKey(sourcePublicKey)
	require.NoErrorf(t, err, "principal from public key: %v", err)

	disc := discovery.New("")
	{
		err := publication.PublishLocalNode(disc, identityapi.Summary{
			Principal: sourcePrincipal,
			Device:    "d_source",
			PublicKey: sourcePublicKey,
		}, privateKey, []string{"tcp://source"})
		require.NoErrorf(t, err, "publish node: %v", err)
	}

	trust := discovery.NewTrustEvaluator()
	trust.Trust(sourcePublicKey)
	store := appdata.NewInDir(t.TempDir())
	{
		err := store.Load()
		require.NoErrorf(t, err, "load data store: %v", err)
	}

	requestID := "req-1"
	requester := "p_requester"
	blobID := "blob-1"
	forged, err := json.Marshal(blobFetchResponse{
		RequestID: requestID,
		Requester: requester,
		BlobID:    blobID,
		Status:    blobFetchStatusOK,
		Blob: appdata.Blob{
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
	sourcePrincipal, err := identityapi.PrincipalFromPublicKey(sourcePublicKey)
	require.NoErrorf(t, err, "principal from public key: %v", err)

	disc := discovery.New("")
	{
		err := publication.PublishLocalNode(disc, identityapi.Summary{
			Principal: sourcePrincipal,
			Device:    "d_source",
			PublicKey: sourcePublicKey,
		}, privateKey, []string{"tcp://source"})
		require.NoErrorf(t, err, "publish node: %v", err)
	}

	trust := discovery.NewTrustEvaluator()
	trust.Trust(sourcePublicKey)
	store := appdata.NewInDir(t.TempDir())
	{
		err := store.Load()
		require.NoErrorf(t, err, "load data store: %v", err)
	}

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
	sourcePrincipal, err := identityapi.PrincipalFromPublicKey(sourcePublicKey)
	require.NoErrorf(t, err, "principal from public key: %v", err)

	disc := discovery.New("")
	{
		err := publication.PublishLocalNode(disc, identityapi.Summary{
			Principal: sourcePrincipal,
			Device:    "d_source",
			PublicKey: sourcePublicKey,
		}, privateKey, []string{"tcp://source"})
		require.NoErrorf(t, err, "publish node: %v", err)
	}

	trust := discovery.NewTrustEvaluator()
	trust.Trust(sourcePublicKey)
	store := appdata.NewInDir(t.TempDir())
	{
		err := store.Load()
		require.NoErrorf(t, err, "load data store: %v", err)
	}

	requestID := "req-2"
	requester := "p_requester"
	blobID := "blob.logical"
	wire, err := marshalBlobResponse(sourcePrincipal, privateKey, blobFetchRequest{
		RequestID: requestID,
		BlobID:    blobID,
		Requester: requester,
	}, appdata.Blob{
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
	sourcePrincipal, err := identityapi.PrincipalFromPublicKey(sourcePublicKey)
	require.NoErrorf(t, err, "principal from public key: %v", err)

	disc := discovery.New("")
	{
		err := publication.PublishLocalNode(disc, identityapi.Summary{
			Principal: sourcePrincipal,
			Device:    "d_source",
			PublicKey: sourcePublicKey,
		}, privateKey, []string{"tcp://source"})
		require.NoErrorf(t, err, "publish node: %v", err)
	}

	store := appdata.NewInDir(t.TempDir())
	{
		err := store.Load()
		require.NoErrorf(t, err, "load data store: %v", err)
	}

	requestID := "req-timeout"
	requester := "p_requester"
	blobID := "blob-timeout"
	wire, err := marshalBlobResponse(sourcePrincipal, privateKey, blobFetchRequest{
		RequestID: requestID,
		BlobID:    blobID,
		Requester: requester,
	}, appdata.Blob{
		ID:        blobID,
		MediaType: "application/octet-stream",
	}, []byte("network payload"))
	require.NoErrorf(t, err, "marshal response: %v", err)

	responses := make(chan []byte, 1)
	responses <- wire
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	transfer, err := store.StartTransfer(appdata.TransferRecord{
		ID: "xfer-timeout", Kind: "blob_fetch", ResourceID: blobID, Direction: "inbound", State: "pending",
	})
	require.NoError(t, err)

	_, err = awaitBlobFetchResponse(ctx, ExchangeConfig{
		Discovery: disc,
		Trust:     discovery.NewTrustEvaluator(),
		Data:      store,
	}, transfer.ID, blobID, requester, requestID, responses)
	require.Falsef(t, err == nil || err.
		Error() !=
		"remote source is not trusted", "error = %v, want explicit remote trust rejection", err)

}
