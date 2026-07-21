package replication

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"

	model "ardents/internal/content/catalog"
	datapayload "ardents/internal/content/payload"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/transfer"
)

func TestReplicaCapacityQueryRequiresBoundEncryptedBlobMetadata(t *testing.T) {
	valid := model.Blob{ID: "cid", CID: "cid", Hash: "hash", Cipher: datapayload.AES256GCMCipher, Size: 10, Encrypted: true}
	if !validReplicaBlobMetadata(valid) {
		t.Fatal("valid encrypted blob metadata was rejected")
	}
	for name, mutate := range map[string]func(*model.Blob){
		"plaintext":          func(blob *model.Blob) { blob.Encrypted = false },
		"wrong cid":          func(blob *model.Blob) { blob.CID = "other" },
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
	source, err := identityprincipal.FromPublicKey(encodedKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signControl(actionReserveOffer, "operation-1", source, "target-1", encodedKey, map[string]string{"cid": "cid-1"}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyControl(raw, "target-1"); err != nil {
		t.Fatalf("verify signed control: %v", err)
	}

	for name, mutate := range map[string]func(*controlWire){
		"action":    func(wire *controlWire) { wire.Action = actionCommitRequest },
		"operation": func(wire *controlWire) { wire.OperationID = "operation-2" },
		"source":    func(wire *controlWire) { wire.Source = "p_forged" },
		"target":    func(wire *controlWire) { wire.Target = "target-2" },
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
	source, err := identityprincipal.FromPublicKey(encodedKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signControl(actionReserveOffer, "operation-1", source, "target-1", encodedKey, struct{}{}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	service := New(Config{LocalNodeID: "target-2"})
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
	source, err := identityprincipal.FromPublicKey(encodedKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signControl(actionReserveOffer, "operation-1", source, "target-1", encodedKey, struct{}{}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyControl(raw, "target-2"); err == nil {
		t.Fatal("replica control for another target was accepted")
	}
}
