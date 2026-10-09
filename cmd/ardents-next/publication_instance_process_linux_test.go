package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPublicationCompiledInstancePublicExchange(t *testing.T) {
	root := filepath.Join(t.TempDir(), "host-instance")
	network := hex.EncodeToString(bytes.Repeat([]byte{51}, 32))
	before := time.Now().UTC().Truncate(time.Second)
	command := compiledCommand(t)
	run := func(success bool, args ...string) []byte {
		t.Helper()
		out, err := exec.Command(command, append([]string{"publication"}, args...)...).CombinedOutput()
		if (err == nil) != success {
			t.Fatalf("public exchange exit: %v", err)
		}
		if bytes.Contains(out, []byte("instance_private")) {
			t.Fatal("private key field exported")
		}
		return out
	}
	prepared := run(true, "instance-initialize", root, network, before.Format(time.RFC3339), before.Add(time.Hour).Format(time.RFC3339))
	var reply struct {
		Outcome string `json:"outcome"`
		Request []byte `json:"request"`
	}
	if err := json.Unmarshal(prepared, &reply); err != nil || reply.Outcome != "instance-request-prepared" {
		t.Fatal("public request result", err)
	}
	if again := run(true, "instance-request", root); !bytes.Equal(again, prepared) {
		t.Fatal("stored request changed")
	}
	const requestDomain = "ardents-service-instance-request-v3\x00"
	request := reply.Request
	if len(request) != len(requestDomain)+112 || string(request[:len(requestDomain)]) != requestDomain {
		t.Fatal("request grammar changed")
	}
	commitment := sha256.Sum256(request[:len(request)-32])
	if !bytes.Equal(commitment[:], request[len(request)-32:]) {
		t.Fatal("request commitment changed")
	}
	// Independently approved fixture Authority signs public host request bytes.
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{17}, 32))
	public := authority.Public().(ed25519.PublicKey)
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), public...))
	credential := append([]byte{0, 3}, public...)
	credential = append(credential, target[:]...)
	credential = append(credential, request[len(requestDomain)+32:len(requestDomain)+64]...)
	credential = binary.BigEndian.AppendUint64(credential, 7)
	credential = append(credential, request[len(requestDomain)+64:len(requestDomain)+80]...)
	credential = append(credential, request[len(requestDomain):len(requestDomain)+32]...)
	credential = binary.BigEndian.AppendUint32(credential, 3)
	credential = append(credential, ed25519.Sign(authority, credential)...)
	response := append([]byte("ardents-service-instance-response-v3\x00"), commitment[:]...)
	response = append(response, credential...)
	responsePath := filepath.Join(t.TempDir(), "public-response")
	if err := os.WriteFile(responsePath, response, 0600); err != nil {
		t.Fatal(err)
	}
	accepted := run(true, "instance-accept", root, responsePath)
	if again := run(true, "instance-accept", root, responsePath); !bytes.Equal(again, accepted) {
		t.Fatal("exact accepted response retry changed")
	}
	var acceptance struct {
		Outcome    string `json:"outcome"`
		Acceptance struct {
			State            string
			Generation       uint64
			CredentialDigest [32]byte
		} `json:"acceptance"`
	}
	if err := json.Unmarshal(accepted, &acceptance); err != nil || acceptance.Outcome != "instance-response-accepted" || acceptance.Acceptance.State != "accepted" || acceptance.Acceptance.Generation != 7 || acceptance.Acceptance.CredentialDigest != sha256.Sum256(credential) {
		t.Fatal("accepted public receipt changed", err)
	}
	response[len(response)-1] ^= 1
	if err := os.WriteFile(responsePath, response, 0600); err != nil {
		t.Fatal(err)
	}
	if out := run(false, "instance-accept", root, responsePath); string(out) != "{\"outcome\":\"instance-unavailable\"}\n" {
		t.Fatal("different response accepted")
	}
	raw, err := os.ReadFile(filepath.Join(root, "instance-root.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer clear(raw)
	var terminal struct {
		Phase   string `json:"phase"`
		Private string `json:"instance_private"`
	}
	if err = json.Unmarshal(raw, &terminal); err != nil || terminal.Phase != "conflicting" || terminal.Private != "" {
		t.Fatal("terminal authority retained", err)
	}
}
