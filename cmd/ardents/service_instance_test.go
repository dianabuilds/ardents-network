package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
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

func TestServiceInstanceBindingOpensSealedIntroductionWithoutExportingRecipient(t *testing.T) {
	now := time.Date(2030, 7, 8, 9, 10, 11, 0, time.UTC)
	network := [32]byte{51}
	root, err := instance.Initialize(instance.InitializeConfig{Root: serviceInstanceFixtureRoot(t), NetworkID: network,
		NotBefore: now, NotAfter: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	request, err := root.Request()
	if err != nil {
		t.Fatal(err)
	}
	view, err := instance.ParseRequest(request)
	if err != nil {
		t.Fatal(err)
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
	response, err := instance.BuildResponse(request, credential)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.Accept(response); err != nil {
		t.Fatal(err)
	}
	binding, err := root.OpenBinding(0)
	if err != nil {
		t.Fatal(err)
	}
	introductionPublic := binding.IntroductionPublic()
	public, err := ecdh.X25519().NewPublicKey(introductionPublic[:])
	if err != nil {
		t.Fatal(err)
	}
	recipient, err := hpke.NewDHKEMPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("one bound Service Introduction")
	sealed, err := route.SealIntroduction(route.SealedIntroduction{NetworkID: network, Digest: [32]byte{52}, Epoch: 7,
		IntroductionNodeID: [32]byte{53}, RendezvousNodeID: [32]byte{54}, Reachability: [32]byte{55},
		NotAfter: now.Add(time.Minute), JoinHandle: [32]byte{56}, EndpointHandshake: [32]byte{57}}, recipient, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := route.OpenSealedIntroductionWith(sealed, binding)
	if err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("purpose-scoped Introduction open = %q / %v", opened, err)
	}
	if err := binding.Withdraw(); err != nil {
		t.Fatal(err)
	}
	if _, err := route.OpenSealedIntroductionWith(sealed, binding); err == nil {
		t.Fatal("withdrawn binding opened a sealed Introduction")
	}
}

func serviceInstanceFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "service-instance-root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}
