package replication

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	model "ardents/internal/content/catalog"
	datapayload "ardents/internal/content/payload"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/replication/placement"
	"ardents/internal/transfer"
)

func TestReplicaCapacityQueryRequiresBoundEncryptedBlobMetadata(t *testing.T) {
	valid := model.Blob{Reference: replicationTestReference(t, "cid"), Hash: "hash", Cipher: datapayload.AES256GCMCipher, Size: 10, Encrypted: true}
	if !validReplicaBlobMetadata(valid) {
		t.Fatal("valid encrypted blob metadata was rejected")
	}
	for name, mutate := range map[string]func(*model.Blob){
		"plaintext":          func(blob *model.Blob) { blob.Encrypted = false },
		"missing reference":  func(blob *model.Blob) { blob.Reference = model.ContentReference{} },
		"missing hash":       func(blob *model.Blob) { blob.Hash = "" },
		"missing cipher":     func(blob *model.Blob) { blob.Cipher = "" },
		"unsupported cipher": func(blob *model.Blob) { blob.Cipher = "custom" },
		"empty size":         func(blob *model.Blob) { blob.Size = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if validReplicaBlobMetadata(candidate) {
				t.Fatal("invalid capacity query blob metadata was accepted")
			}
		})
	}
}

func TestReplicaControlSignatureBindsRoutingAndBody(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encodedKey := base64.StdEncoding.EncodeToString(publicKey)
	sourceText, err := identityprincipal.FromPublicKey(encodedKey)
	if err != nil {
		t.Fatal(err)
	}
	source, err := identityprincipal.Parse(sourceText)
	if err != nil {
		t.Fatal(err)
	}
	target := replicationTestPrincipal("target-1")
	raw, err := signControl(actionReserveOffer, "operation-1", source, target, encodedKey, map[string]string{"cid": "cid-1"}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyControl(raw, target); err != nil {
		t.Fatalf("verify signed control: %v", err)
	}

	for name, mutate := range map[string]func(*controlWire){
		"action":    func(wire *controlWire) { wire.Action = actionCommitRequest },
		"operation": func(wire *controlWire) { wire.OperationID = "operation-2" },
		"source":    func(wire *controlWire) { wire.Source = replicationTestPrincipal("forged") },
		"target":    func(wire *controlWire) { wire.Target = replicationTestPrincipal("target-2") },
		"body":      func(wire *controlWire) { wire.Body = json.RawMessage(`{"cid":"cid-2"}`) },
	} {
		t.Run(name, func(t *testing.T) {
			var wire controlWire
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatal(err)
			}
			mutate(&wire)
			tampered, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := verifyControl(tampered, wire.Target); err == nil {
				t.Fatal("tampered replica control was accepted")
			}
		})
	}
}

func TestReplicaControlForAnotherTargetIsSilentlyIgnored(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encodedKey := base64.StdEncoding.EncodeToString(publicKey)
	sourceText, err := identityprincipal.FromPublicKey(encodedKey)
	if err != nil {
		t.Fatal(err)
	}
	source, err := identityprincipal.Parse(sourceText)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signControl(actionReserveOffer, "operation-1", source, replicationTestPrincipal("target-1"), encodedKey, struct{}{}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	service := New(Config{LocalNodePrincipal: replicationTestPrincipal("target-2")})
	err = service.handle(context.Background(), transfer.ReplicaControlMessage{
		OperationID: "operation-1", Action: actionReserveOffer, Payload: raw,
	})
	if err != nil {
		t.Fatalf("control for another target must be ignored: %v", err)
	}
}

func TestReplicaControlRejectsWrongLocalTarget(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encodedKey := base64.StdEncoding.EncodeToString(publicKey)
	sourceText, err := identityprincipal.FromPublicKey(encodedKey)
	if err != nil {
		t.Fatal(err)
	}
	source, err := identityprincipal.Parse(sourceText)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signControl(actionReserveOffer, "operation-1", source, replicationTestPrincipal("target-1"), encodedKey, struct{}{}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyControl(raw, replicationTestPrincipal("target-2")); err == nil {
		t.Fatal("replica control for another target was accepted")
	}
}

func TestReplicaControlBodyRejectsObsoleteCommitmentTargetFields(t *testing.T) {
	body := healthQueryBody{Commitment: placement.Commitment{
		OperationID: "operation", IntentVersion: 1, ContentReference: replicationTestReference(t, "cid"),
		TargetNode: replicationTestPrincipal("target"), Size: 1, State: placement.CommitmentActive,
		LeaseStartsAt: time.Now().UTC(), LastObservedAt: time.Now().UTC(), LeaseExpiresAt: time.Now().UTC().Add(time.Hour),
	}, RequestedLeaseExpiresAt: time.Now().UTC().Add(time.Hour)}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, oldField := range []string{"peer_id", "PeerID"} {
		t.Run(oldField, func(t *testing.T) {
			obsolete := strings.Replace(string(raw), "target_node", oldField, 1)
			var decoded healthQueryBody
			if err := decodeControlBody([]byte(obsolete), &decoded); err == nil {
				t.Fatal("obsolete replica target field was accepted")
			}
		})
	}
	for _, oldField := range []string{"blob_id", "BlobID", "cid", "CID"} {
		t.Run(oldField, func(t *testing.T) {
			obsolete := strings.Replace(string(raw), "content_reference", oldField, 1)
			var decoded healthQueryBody
			if err := decodeControlBody([]byte(obsolete), &decoded); err == nil {
				t.Fatal("obsolete replica content identity field was accepted")
			}
		})
	}
	malformed := strings.Replace(string(raw), body.Commitment.TargetNode.String(), "not-a-principal", 1)
	var decoded healthQueryBody
	if err := decodeControlBody([]byte(malformed), &decoded); err == nil {
		t.Fatal("malformed replica target Principal was accepted")
	}
}
