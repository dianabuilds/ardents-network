package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/instance"
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
	plan := map[string]any{"schema": "ardents-service-instance-initialize-v1",
		"root": rootPath, "network_id": hex.EncodeToString(network[:]),
		"not_before": now.Format(time.RFC3339), "not_after": now.Add(time.Hour).Format(time.RFC3339)}
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
}
