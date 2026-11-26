//go:build linux && document_worker_installed

package endpoint

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/publication"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

// InitializeInstalledServiceInstance drives the compiled command to prepare and accept a signed Credential.
func InitializeInstalledServiceInstance(t *testing.T, program string, credential *syscall.Credential, base string, network [32]byte) {
	t.Helper()
	before := time.Now().UTC().Truncate(time.Second)
	raw := RunStartupCommand(t, program, credential, "instance-request-prepared", "publication", "instance-initialize",
		filepath.Join(base, "instance"), hex.EncodeToString(network[:]), before.Format(time.RFC3339), before.Add(time.Hour).Format(time.RFC3339))
	var result struct{ Request []byte }
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	request, err := publication.ParseServiceRequest(result.Request)
	if err != nil {
		t.Fatal(err)
	}
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	response, _ := processfixture.SignedInstanceResponse(authority, network, request.InstancePublic, request.Commitment, 1, request.NotBefore, request.NotAfter)
	path := filepath.Join(base, "bootstrap", "credential-response")
	WriteStartupFile(t, path, response, 0644)
	RunStartupCommand(t, program, credential, "instance-response-accepted", "publication", "instance-accept", filepath.Join(base, "instance"), path)
}
