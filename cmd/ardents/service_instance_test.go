package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func TestServiceInstanceInitializePublishesOnlyStableRequest(t *testing.T) {
	directory := t.TempDir()
	rootPath := filepath.Join(directory, "instance-root")
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	network := [32]byte{91}
	now := time.Now().UTC().Truncate(time.Second)
	planPath := filepath.Join(directory, "instance-plan.json")
	requestPath := filepath.Join(directory, "instance-request.bin")
	plan := map[string]any{"schema": "ardents-service-instance-initialize-v1",
		"root": rootPath, "network_id": hex.EncodeToString(network[:]),
		"not_before": now.Format(time.RFC3339), "not_after": now.Add(time.Hour).Format(time.RFC3339),
		"request_file": requestPath}
	raw, err := json.Marshal(plan)
	if err != nil || os.WriteFile(planPath, raw, 0o600) != nil {
		t.Fatal("write Service Instance initialization plan")
	}
	initialize := func() []byte {
		t.Helper()
		var output bytes.Buffer
		if err := run(context.Background(), []string{"service-instance", "initialize", "--config", planPath}, &output); err != nil {
			t.Fatal(err)
		}
		return output.Bytes()
	}
	first, second := initialize(), initialize()
	if !bytes.Equal(first, second) || bytes.Contains(first, []byte("private")) {
		t.Fatalf("Service Instance receipt is unstable or discloses a private field: %s", first)
	}
	var receipt struct {
		Schema  string `json:"schema"`
		Request []byte `json:"request"`
	}
	if err := json.Unmarshal(first, &receipt); err != nil {
		t.Fatal(err)
	}
	view, err := instance.ParseRequest(receipt.Request)
	if err != nil || receipt.Schema != "ardents-service-instance-request-v1" || view.NetworkID != network {
		t.Fatalf("Service Instance receipt = %+v, view = %+v, err = %v", receipt, view, err)
	}
	persistedRequest, err := os.ReadFile(requestPath)
	if err != nil || !bytes.Equal(persistedRequest, receipt.Request) {
		t.Fatalf("persisted public request differs: %v", err)
	}
	_, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := (publication.Credential{InstancePublic: view.InstancePublic,
		IntroductionHPKEPublic: view.IntroductionPublic, Generation: 1, NotBefore: view.NotBefore,
		NotAfter: view.NotAfter, NetworkID: view.NetworkID,
		Capabilities: publication.CapabilityPublish | publication.CapabilityConnect}).Issue(authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	response, err := instance.BuildResponse(receipt.Request, credential)
	if err != nil {
		t.Fatal(err)
	}
	responsePath := filepath.Join(directory, "instance-response.bin")
	if err := os.WriteFile(responsePath, response, 0o600); err != nil {
		t.Fatal(err)
	}
	var accepted bytes.Buffer
	if err := run(context.Background(), []string{"service-instance", "accept", "--root", rootPath,
		"--response", responsePath}, &accepted); err != nil {
		t.Fatalf("accept Service Credential response: %v", err)
	}
	var result struct {
		Schema     string `json:"schema"`
		State      string `json:"state"`
		Generation uint64 `json:"generation"`
	}
	if err := json.Unmarshal(accepted.Bytes(), &result); err != nil || result.Schema != "ardents-service-instance-acceptance-v1" ||
		result.State != "accepted" || result.Generation != 1 || bytes.Contains(accepted.Bytes(), []byte("private")) {
		t.Fatalf("acceptance receipt = %+v / %v", result, err)
	}
}
