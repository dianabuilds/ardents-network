//go:build linux

package endpoint

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type textReaderProcessInput struct {
	Snapshot state.Snapshot
	View     state.ClosedRouteView
	Target   [32]byte
}

// Public State/worker qualification are fixtures. The child creates its own
// Endpoint, context, holder key, roots, token stock and real network channels.
func runTextReaderObservationChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var input textReaderProcessInput
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	endpoint, owner, source := textSourceContextFixture(t)
	source.mu.Lock()
	source.snapshot, source.view = input.Snapshot, input.View
	source.mu.Unlock()
	endpoint.network = input.View.Profile.NetworkID
	endpoint.closedTokenRoot = textNetworkPrivateRoot(t)
	request, digest, err := owner.requestTextPermission([3]uint32{64, 64, 0})
	if err != nil {
		t.Fatal(err)
	}
	encoder, decoder := json.NewEncoder(os.Stdout), json.NewDecoder(os.Stdin)
	if err := encoder.Encode(request); err != nil {
		t.Fatal(err)
	}
	var permission []byte
	if err := decoder.Decode(&permission); err != nil {
		t.Fatal(err)
	}
	if err := owner.importTextPermission(digest, permission); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.openTextPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	proof := lookupTextPublishedProof(t, owner, input.Target)
	if err := encoder.Encode(proof); err != nil {
		t.Fatal(err)
	}
	var command string
	if err := decoder.Decode(&command); err != nil || command != "stop" {
		t.Fatal("reader controller ended before stop")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
}

func observeTextIndependentReader(t *testing.T, source *textSourceStateFixture, target [32]byte, expected []byte, output string) {
	t.Helper()
	source.mu.Lock()
	input := textReaderProcessInput{source.snapshot, source.view, target}
	source.mu.Unlock()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(output, "reader-input.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "-test.run=^TestTextPublicationIsolatedRoleObservations$", "-test.timeout=55s")
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "ARDENTS_TEXT_READER_CHILD=" + path}
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	joined := false
	defer func() {
		if !joined {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	reader := bufio.NewReader(stdout)
	decoder, encoder := json.NewDecoder(reader), json.NewEncoder(stdin)
	var request []byte
	if err := decoder.Decode(&request); err != nil {
		t.Fatalf("reader permission request: %v", err)
	}
	permission := source.issueRawPermission(t, request, sha256.Sum256(request))
	if err := encoder.Encode(permission); err != nil {
		t.Fatal(err)
	}
	var proof []byte
	if err := decoder.Decode(&proof); err != nil {
		t.Fatalf("reader lookup result: %v", err)
	}
	// Actual public provisioning and lookup bytes belong to the observer only.
	// They preserve same-run comparison inputs without exporting child secrets.
	exchange, err := json.Marshal(struct{ Request, Permission, Descriptor []byte }{request, permission, proof})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "reader-exchange.json"), exchange, 0600); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(proof, expected) {
		t.Fatal("independent reader got different Descriptor")
	}
	if err := encoder.Encode("stop"); err != nil {
		t.Fatal(err)
	}
	_ = stdin.Close()
	// Include bytes buffered by the JSON decoder before joining StdoutPipe.
	_, readErr := io.Copy(io.Discard, io.MultiReader(decoder.Buffered(), reader))
	err = command.Wait()
	joined = true
	if err != nil {
		t.Fatal(fmt.Errorf("reader process: %w; %s", err, stderr.String()))
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
}
