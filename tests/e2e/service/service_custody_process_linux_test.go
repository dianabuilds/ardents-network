//go:build linux

package service_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

func TestServiceCustodyBuiltTerminalAndNewInstanceAcceptance(t *testing.T) {
	next, custody := buildProductCommand(t, "ardents-next"), buildProductCommand(t, "ardents-custody")
	directory := t.TempDir()
	instanceRoot, vault := filepath.Join(directory, "host-instance"), filepath.Join(directory, "independent-Service-vault")
	environment, network, root := [32]byte{81}, [32]byte{82}, [32]byte{83}
	now := time.Now().UTC().Truncate(time.Second)
	prepared := runCommand(t, t.Context(), directory, next, "publication", "instance-initialize", instanceRoot, hex.EncodeToString(network[:]), now.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339))
	repeated := runCommand(t, t.Context(), directory, next, "publication", "instance-request", instanceRoot)
	if !bytes.Equal(prepared, repeated) {
		t.Fatal("host request changed across independent process reopen")
	}
	var input struct {
		Request []byte `json:"request"`
	}
	if err := json.Unmarshal(prepared, &input); err != nil {
		t.Fatal(err)
	}
	view, err := publication.ParseServiceRequest(input.Request)
	if err != nil {
		t.Fatal(err)
	}
	requestPath, responsePath := filepath.Join(directory, "request.bin"), filepath.Join(directory, "response.bin")
	if err := os.WriteFile(requestPath, input.Request, 0o600); err != nil {
		t.Fatal(err)
	}
	password := "actual Service terminal password"
	createdTerminal := runInteractiveProductCommand(t, directory, custody, []interactiveProductInput{
		{prompt: "vault-create password:", value: password, secret: true}, {prompt: "vault-create-confirm password:", value: password, secret: true},
	}, "create-service-authority", "-vault-root", vault, "-environment-commitment", hex.EncodeToString(environment[:]), "-network-commitment", hex.EncodeToString(network[:]), "-root-commitment", hex.EncodeToString(root[:]))
	var created struct {
		RecordID string `json:"record_id"`
		ID       string `json:"id_commitment"`
		Public   string `json:"authority_public"`
		Target   string `json:"target"`
	}
	if err := json.Unmarshal(interactiveProductJSON(t, createdTerminal), &created); err != nil {
		t.Fatal(err)
	}
	public, err := hex.DecodeString(created.Public)
	if err != nil || len(public) != 32 {
		t.Fatal("Authority receipt", err)
	}
	digest := sha256.Sum256(input.Request)
	arguments := []string{"issue-service-credential", "-vault-root", vault, "-record", created.RecordID, "-request", requestPath, "-response", responsePath,
		"-environment-commitment", hex.EncodeToString(environment[:]), "-network-commitment", hex.EncodeToString(network[:]), "-root-commitment", hex.EncodeToString(root[:]), "-kind", "service", "-id-commitment", created.ID}
	inputs := []interactiveProductInput{{prompt: "service-request SHA-256", value: hex.EncodeToString(digest[:])}, {prompt: "vault-unlock password:", value: password, secret: true}}
	issuedTerminal := runInteractiveProductCommand(t, directory, custody, inputs, arguments...)
	response, err := os.ReadFile(responsePath)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := publication.VerifyServiceResponse(response, view)
	delegation := credential.Delegation()
	if err != nil || delegation.Generation != 1 || !bytes.Equal(delegation.Authority[:], public) {
		t.Fatal("public verifier refused actual Custody", err)
	}
	const prefix = "ardents-service-instance-response-v3\x00"
	credentialRaw := response[len(prefix)+32:]
	if !ed25519.Verify(ed25519.PublicKey(public), credentialRaw[:158], credentialRaw[158:]) {
		t.Fatal("independent byte signature oracle refused")
	}
	accepted := runCommand(t, t.Context(), directory, next, "publication", "instance-accept", instanceRoot, responsePath)
	if !bytes.Contains(accepted, []byte("instance-response-accepted")) {
		t.Fatal("genuine host did not accept")
	}
	acceptedAgain := runCommand(t, t.Context(), directory, next, "publication", "instance-accept", instanceRoot, responsePath)
	if !bytes.Equal(accepted, acceptedAgain) {
		t.Fatal("host acceptance retry changed")
	}
	retryTerminal := runInteractiveProductCommand(t, directory, custody, inputs, arguments...)
	if !bytes.Equal(interactiveProductJSON(t, issuedTerminal), interactiveProductJSON(t, retryTerminal)) {
		t.Fatal("actual issuer retry changed")
	}
	for _, transcript := range [][]byte{createdTerminal, issuedTerminal, retryTerminal, prepared, accepted} {
		if bytes.Contains(transcript, []byte(password)) {
			t.Fatal("password escaped terminal")
		}
	}
}
