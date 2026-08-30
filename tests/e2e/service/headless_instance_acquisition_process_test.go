package service_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func TestHeadlessServiceInstanceAcquisitionIsAtMostOnceAcrossProcesses(t *testing.T) {
	binary := buildProductCommand(t, "ardents")
	directory := t.TempDir()
	rootPath := filepath.Join(directory, "instance-root")
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	network := [32]byte{41}
	now := time.Now().UTC().Truncate(time.Second)
	requestPath := filepath.Join(directory, "instance-request.bin")
	planPath := filepath.Join(directory, "instance-plan.json")
	plan, err := json.Marshal(map[string]string{
		"schema":       "ardents-service-instance-initialize-v1",
		"root":         rootPath,
		"network_id":   hex.EncodeToString(network[:]),
		"not_before":   now.Format(time.RFC3339),
		"not_after":    now.Add(time.Hour).Format(time.RFC3339),
		"request_file": requestPath,
	})
	if err != nil || os.WriteFile(planPath, plan, 0o600) != nil {
		t.Fatal("write Service Instance process plan")
	}

	initialized := runCommand(t, t.Context(), directory, binary,
		"service-instance", "initialize", "--config", planPath)
	var initialization struct {
		Schema  string `json:"schema"`
		Request []byte `json:"request"`
	}
	if err := json.Unmarshal(initialized, &initialization); err != nil ||
		initialization.Schema != "ardents-service-instance-request-v1" {
		t.Fatalf("initialization receipt = %+v / %v", initialization, err)
	}
	request, err := os.ReadFile(requestPath)
	if err != nil || string(request) != string(initialization.Request) {
		t.Fatalf("persisted public request differs: %v", err)
	}
	requestView, err := instance.ParseRequest(request)
	if err != nil || requestView.NetworkID != network {
		t.Fatalf("public request = %+v / %v", requestView, err)
	}

	_, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	response := serviceInstanceResponse(t, request, requestView, authorityPrivate, 1)
	responsePath := filepath.Join(directory, "instance-response.bin")
	if err := os.WriteFile(responsePath, response, 0o600); err != nil {
		t.Fatal(err)
	}
	assertServiceInstanceAccepted(t, directory, binary, rootPath, responsePath)
	assertServiceInstanceAccepted(t, directory, binary, rootPath, responsePath)

	conflicting := serviceInstanceResponse(t, request, requestView, authorityPrivate, 2)
	conflictingPath := filepath.Join(directory, "conflicting-response.bin")
	if err := os.WriteFile(conflictingPath, conflicting, 0o600); err != nil {
		t.Fatal(err)
	}
	assertServiceInstanceUnavailable(t, directory, binary, rootPath, conflictingPath)
	assertServiceInstanceUnavailable(t, directory, binary, rootPath, responsePath)
}

func serviceInstanceResponse(t *testing.T, request []byte, view instance.RequestView,
	authority ed25519.PrivateKey, generation uint64) []byte {
	t.Helper()
	credential, err := (publication.Credential{
		InstancePublic:         view.InstancePublic,
		IntroductionHPKEPublic: view.IntroductionPublic,
		Generation:             generation,
		NotBefore:              view.NotBefore,
		NotAfter:               view.NotAfter,
		NetworkID:              view.NetworkID,
		Capabilities:           publication.CapabilityPublish | publication.CapabilityConnect,
	}).Issue(authority)
	if err != nil {
		t.Fatal(err)
	}
	response, err := instance.BuildResponse(request, credential)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func assertServiceInstanceAccepted(t *testing.T, directory, binary, rootPath, responsePath string) {
	t.Helper()
	output := runCommand(t, t.Context(), directory, binary,
		"service-instance", "accept", "--root", rootPath, "--response", responsePath)
	var receipt struct {
		Schema     string `json:"schema"`
		State      string `json:"state"`
		Generation uint64 `json:"generation"`
	}
	if err := json.Unmarshal(output, &receipt); err != nil ||
		receipt.Schema != "ardents-service-instance-acceptance-v1" ||
		receipt.State != "accepted" || receipt.Generation != 1 {
		t.Fatalf("acceptance receipt = %+v / %v", receipt, err)
	}
}

func assertServiceInstanceUnavailable(t *testing.T, directory, binary, rootPath, responsePath string) {
	t.Helper()
	command := exec.Command(binary, "service-instance", "accept", "--root", rootPath, "--response", responsePath)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("unavailable Service Instance accepted a response: %s", output)
	}
}
