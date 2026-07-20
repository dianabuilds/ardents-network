package transfer

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"

	appdata "ardents/internal/data"
	"ardents/internal/data/chunking"
	discovery "ardents/internal/discovery"
	identityapi "ardents/internal/identity/api"
	publication "ardents/internal/publication"

	"github.com/stretchr/testify/require"
)

func TestManifestResponseBindsCompleteManifestToTrustedSource(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	encodedKey := base64.StdEncoding.EncodeToString(publicKey)
	principal, err := identityapi.PrincipalFromPublicKey(encodedKey)
	require.NoError(t, err)
	disc := discovery.New("")
	require.NoError(t, publication.PublishLocalNode(disc, identityapi.Summary{
		Principal: principal, Device: "source-device", PublicKey: encodedKey,
	}, privateKey, []string{"tcp://source"}))
	trust := discovery.NewTrustEvaluator()
	trust.Trust(encodedKey)
	target := appdata.NewInDir(t.TempDir())
	require.NoError(t, target.Load())
	plan, err := chunking.Plan([]string{"chunk-1", "chunk-2"}, chunking.ManifestSpec{
		Owner: "owner", MediaType: "application/octet-stream", KeyID: "key-1", TotalPlaintextBytes: 2,
	})
	require.NoError(t, err)
	request := blobFetchRequest{RequestID: "manifest-request", Requester: "requester", BlobID: plan.Root.ID, ResourceKind: "manifest"}
	wire, err := marshalManifestResponse(principal, privateKey, request, plan.Root, nil)
	require.NoError(t, err)
	cfg := ExchangeConfig{Discovery: disc, Trust: trust, Data: target}
	received, source, err := acceptManifestResponse(cfg, plan.Root.ID, request.Requester, request.RequestID, wire)
	require.NoError(t, err)
	require.Equal(t, principal, source)
	require.Equal(t, plan.Root.ID, received.ID)
	require.Equal(t, plan.Root.Refs, received.Refs)
	require.NoError(t, chunking.ValidateManifest(received))

	var tampered blobFetchResponse
	require.NoError(t, json.Unmarshal(wire, &tampered))
	tampered.Manifest.Refs[0].ID = "attacker-selected-chunk"
	wire, err = json.Marshal(tampered)
	require.NoError(t, err)
	_, _, err = acceptManifestResponse(cfg, plan.Root.ID, request.Requester, request.RequestID, wire)
	require.ErrorContains(t, err, "signature")
}
