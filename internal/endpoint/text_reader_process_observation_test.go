//go:build linux

package endpoint

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

type textReaderProcessInput struct {
	Snapshot state.Snapshot
	View     state.ClosedRouteView
	Target   [32]byte
	Expected []byte
}

type textReaderObservationEvent struct {
	Phase                    string    `json:"phase"`
	ResponseDescriptorSHA256 string    `json:"responseDescriptorSHA256,omitempty"`
	ResponseRevision         uint64    `json:"responseRevision,omitempty"`
	HolderSHA256             string    `json:"holderSHA256,omitempty"`
	PermissionIDSHA256       string    `json:"permissionIDSHA256,omitempty"`
	IssuanceBatches          uint8     `json:"issuanceBatches,omitempty"`
	Reserved                 [3]uint32 `json:"reserved,omitempty"`
	DescriptorFloorRevision  uint64    `json:"descriptorFloorRevision,omitempty"`
}

type textReaderControl struct {
	Command    string `json:"command"`
	Permission []byte `json:"permission,omitempty"`
}

type textReaderLiveBoundaryReceipt struct {
	Carrier                  string                   `json:"carrier"`
	InputSHA256              string                   `json:"inputSHA256"`
	ExchangeSHA256           string                   `json:"exchangeSHA256"`
	Boundaries               []textReaderLiveBoundary `json:"boundaries"`
	ResponseDescriptorSHA256 string                   `json:"responseDescriptorSHA256"`
	ResponseRevision         uint64                   `json:"responseRevision"`
}

type textReaderLiveBoundary struct {
	Phase                        string   `json:"phase"`
	ObservedOwners               []string `json:"observedOwners"`
	UnobservedReceivingOwners    []string `json:"unobservedReceivingOwners"`
	UnobservedAdmissionTLSOwners []string `json:"unobservedAdmissionTLSOwners"`
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
	closed := false
	defer func() {
		if !closed {
			_ = owner.Close()
			_ = endpoint.Close()
		}
	}()
	request, digest, err := owner.requestTextPermission([3]uint32{64, 64, 0})
	if err != nil {
		t.Fatal(err)
	}
	protocol := textReaderProtocol(t)
	defer protocol.Close()
	encoder, decoder := json.NewEncoder(protocol), json.NewDecoder(os.Stdin)
	if err := encoder.Encode(request); err != nil {
		t.Fatal(err)
	}
	var control textReaderControl
	for {
		if err := decoder.Decode(&control); err != nil || control.Command != "permission" {
			t.Fatal("reader controller did not provide permission")
		}
		if err := owner.importTextPermission(digest, control.Permission); err != nil {
			if err := encoder.Encode(textReaderObservationEvent{Phase: "permission-refused"}); err != nil {
				t.Fatal(err)
			}
			continue
		}
		break
	}
	if err := encoder.Encode(textReaderObservationEvent{Phase: "permission-imported"}); err != nil {
		t.Fatal(err)
	}
	// The opened prefix retains this wrapper. Keep it inactive while opening,
	// then pause the actual resolution flight's State reread before selection.
	paused := &textPausedResolutionState{textSourceStateFixture: source, entered: make(chan struct{}), release: make(chan struct{})}
	endpoint.closedState = paused
	if _, err := owner.openTextPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	for {
		if err := decoder.Decode(&control); err != nil {
			t.Fatal(err)
		}
		switch control.Command {
		case "lookup":
			textReaderLookupObservation(t, owner, input, encoder, decoder, paused, true)
		case "repeat-lookup":
			textReaderLookupObservation(t, owner, input, encoder, decoder, paused, false)
		case "stop":
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			if err := endpoint.Close(); err != nil {
				t.Fatal(err)
			}
			closed = true
			return
		default:
			t.Fatal("reader controller sent unknown command")
		}
	}
}

func textReaderProtocol(t *testing.T) *os.File {
	t.Helper()
	fd, err := strconv.Atoi(os.Getenv("ARDENTS_TEXT_READER_PROTOCOL_FD"))
	if err != nil || fd < 3 {
		t.Fatal("reader child did not receive a private observation pipe")
	}
	protocol := os.NewFile(uintptr(fd), "reader-observation")
	if protocol == nil {
		t.Fatal("reader child observation pipe is unavailable")
	}
	return protocol
}

func textReaderLookupObservation(t *testing.T, owner *textContext, input textReaderProcessInput, encoder *json.Encoder, decoder *json.Decoder, paused *textPausedResolutionState, first bool) {
	t.Helper()
	var verified reachability.Verified
	if first {
		paused.active.Store(true)
		type lookupResult struct {
			verified reachability.Verified
			err      error
		}
		result := make(chan lookupResult, 1)
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(paused.release) }) }
		joined := false
		defer func() {
			release()
			if !joined {
				select {
				case outcome := <-result:
					if outcome.err != nil {
						t.Error(outcome.err)
					}
				case <-time.After(5 * time.Second):
					t.Error("reader lookup did not join")
				}
			}
		}()
		go func() {
			outcome, err := owner.lookupTextDescriptor(t.Context(), input.Target)
			result <- lookupResult{verified: outcome, err: err}
		}()
		select {
		case <-paused.entered:
		case <-time.After(5 * time.Second):
			t.Fatal("reader lookup did not reach active boundary")
		}
		if err := encoder.Encode(textReaderObservationEvent{Phase: "protected-lookup-active"}); err != nil {
			t.Fatal(err)
		}
		var control textReaderControl
		if err := decoder.Decode(&control); err != nil || control.Command != "release-lookup" {
			t.Fatal("reader controller did not release lookup")
		}
		release()
		outcome := <-result
		if outcome.err != nil {
			t.Fatal(outcome.err)
		}
		verified, joined = outcome.verified, true
	} else {
		outcome, err := owner.lookupTextDescriptor(t.Context(), input.Target)
		if err != nil {
			t.Fatal(err)
		}
		verified = outcome
	}
	expected, err := reachability.VerifyPrivate(input.Expected, input.Target, input.View.Profile.NetworkID, input.View.Profile.Digest, time.Now().UTC())
	if err != nil || !reflect.DeepEqual(verified, expected) {
		t.Fatalf("reader lookup result did not retain exact Store Descriptor: %#v / %v", verified, err)
	}
	responseDigest := sha256.Sum256(input.Expected)
	owner.mu.Lock()
	permission := owner.permission
	if permission == nil || permission.accepted == (credential.Permission{}) || permission.batches == 0 {
		owner.mu.Unlock()
		t.Fatal("reader lookup lost its actual permission allocation")
	}
	holderDigest, permissionIDDigest := sha256.Sum256(permission.accepted.HolderKey[:]), sha256.Sum256(permission.accepted.PermissionID[:])
	batches := permission.batches
	reserved := permission.reserved
	floor := owner.descriptorFloors[input.Target]
	owner.mu.Unlock()
	if err := encoder.Encode(textReaderObservationEvent{Phase: "response-received-before-close", ResponseDescriptorSHA256: hex.EncodeToString(responseDigest[:]), ResponseRevision: verified.Descriptor.Private.Revision, HolderSHA256: hex.EncodeToString(holderDigest[:]), PermissionIDSHA256: hex.EncodeToString(permissionIDDigest[:]), IssuanceBatches: batches, Reserved: reserved, DescriptorFloorRevision: floor.revision}); err != nil {
		t.Fatal(err)
	}
	// This is the observer's known Store proof, accepted only after the real
	// lookup above returned the identical verified Descriptor. The child never
	// performs a direct exchange to manufacture this evidence.
	if err := encoder.Encode(input.Expected); err != nil {
		t.Fatal(err)
	}
}

func observeTextIndependentReader(t *testing.T, source *textSourceStateFixture, target [32]byte, expected []byte, output, carrier string) {
	t.Helper()
	source.mu.Lock()
	input := textReaderProcessInput{Snapshot: source.snapshot, View: source.view, Target: target, Expected: append([]byte(nil), expected...)}
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
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	protocolRead, protocolWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer protocolRead.Close()
	defer protocolWrite.Close()
	command.ExtraFiles = []*os.File{protocolWrite}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "ARDENTS_TEXT_READER_CHILD=" + path, "ARDENTS_TEXT_READER_PROTOCOL_FD=3"}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	if err := protocolWrite.Close(); err != nil {
		t.Fatal(err)
	}
	joined := false
	defer func() {
		if !joined {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	reader := bufio.NewReader(protocolRead)
	decoder, encoder := json.NewDecoder(reader), json.NewEncoder(stdin)
	var request []byte
	if err := decoder.Decode(&request); err != nil {
		t.Fatalf("reader permission request: %v", err)
	}
	permission := source.issueRawPermission(t, request, sha256.Sum256(request))
	if err := encoder.Encode(textReaderControl{Command: "permission", Permission: permission}); err != nil {
		t.Fatal(err)
	}
	var event textReaderObservationEvent
	if err := decoder.Decode(&event); err != nil || event.Phase != "permission-imported" {
		t.Fatalf("reader permission import event: %#v / %v", event, err)
	}
	if err := encoder.Encode(textReaderControl{Command: "lookup"}); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&event); err != nil || event.Phase != "protected-lookup-active" {
		t.Fatalf("reader active lookup event: %#v / %v", event, err)
	}
	if err := encoder.Encode(textReaderControl{Command: "release-lookup"}); err != nil {
		t.Fatal(err)
	}
	responseDigest := sha256.Sum256(expected)
	expectedResponse, err := reachability.VerifyPrivate(expected, target, input.View.Profile.NetworkID, input.View.Profile.Digest, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&event); err != nil || event.Phase != "response-received-before-close" || event.ResponseDescriptorSHA256 != hex.EncodeToString(responseDigest[:]) || event.ResponseRevision != expectedResponse.Descriptor.Private.Revision {
		t.Fatalf("reader response event: %#v / %v", event, err)
	}
	var proof []byte
	if err := decoder.Decode(&proof); err != nil {
		t.Fatalf("reader lookup result: %v", err)
	}
	// The child returned the observer's known Store proof only after its actual
	// lookup yielded the same verified Descriptor. Preserve the same-run
	// comparison bytes without exporting any child secret.
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
	inputBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	exchangeBytes, err := os.ReadFile(filepath.Join(output, "reader-exchange.json"))
	if err != nil {
		t.Fatal(err)
	}
	inputDigest, exchangeDigest := sha256.Sum256(inputBytes), sha256.Sum256(exchangeBytes)
	boundaries := []textReaderLiveBoundary{
		{Phase: "permission-imported", ObservedOwners: []string{"reader permission-import owner"}, UnobservedReceivingOwners: []string{"Resolution receiving owner"}, UnobservedAdmissionTLSOwners: []string{"Resolution admission owner", "Resolution role TLS owner"}},
		{Phase: "protected-lookup-active", ObservedOwners: []string{"reader textResolutionFlight with retained Source prefix before Resolution recipient selection"}, UnobservedReceivingOwners: []string{"Resolution receiving owner"}, UnobservedAdmissionTLSOwners: []string{"Resolution admission owner", "Resolution role TLS owner"}},
		{Phase: "response-received-before-close", ObservedOwners: []string{"reader textResolutionFlight verified response owner"}, UnobservedReceivingOwners: []string{"Resolution receiving owner"}, UnobservedAdmissionTLSOwners: []string{"Resolution admission owner", "Resolution role TLS owner"}},
	}
	receiptBytes, err := json.Marshal(textReaderLiveBoundaryReceipt{
		Carrier:                  carrier,
		InputSHA256:              hex.EncodeToString(inputDigest[:]),
		ExchangeSHA256:           hex.EncodeToString(exchangeDigest[:]),
		Boundaries:               boundaries,
		ResponseDescriptorSHA256: event.ResponseDescriptorSHA256,
		ResponseRevision:         event.ResponseRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(output, "reader-live-boundaries.json")
	if err := os.WriteFile(receiptPath, receiptBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	persisted, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt textReaderLiveBoundaryReceipt
	if err := json.Unmarshal(persisted, &receipt); err != nil || receipt.Carrier != carrier || receipt.InputSHA256 != hex.EncodeToString(inputDigest[:]) || receipt.ExchangeSHA256 != hex.EncodeToString(exchangeDigest[:]) || receipt.ResponseDescriptorSHA256 != event.ResponseDescriptorSHA256 || receipt.ResponseRevision != event.ResponseRevision || !reflect.DeepEqual(receipt.Boundaries, boundaries) {
		t.Fatalf("reader boundary receipt = %#v / %v", receipt, err)
	}
	if err := encoder.Encode(textReaderControl{Command: "stop"}); err != nil {
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
