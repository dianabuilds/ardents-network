//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/attempts"
	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

type endpointCrashBoundary struct {
	Profile            state.ClosedProfileView `json:"profile"`
	Principal          [32]byte                `json:"principal,omitzero"`
	Capability         [32]byte                `json:"capability,omitzero"`
	PermissionRequest  []byte                  `json:"permission_request"`
	PermissionResponse []byte                  `json:"permission_response"`
	PermissionDigest   [32]byte                `json:"permission_digest,omitzero"`
	Token              []byte                  `json:"token"`
	AttemptProfile     [32]byte                `json:"attempt_profile,omitzero"`
	Receiver           [32]byte                `json:"receiver,omitzero"`
	Nonce              [32]byte                `json:"nonce,omitzero"`
	Duty               uint64                  `json:"duty"`
	Window             time.Time               `json:"window"`
	Observed           time.Time               `json:"observed"`
	AdmissionActive    uint32                  `json:"admission_active"`
	WorkerGrantActive  uint32                  `json:"worker_grant_active"`
	JobLive            bool                    `json:"job_live"`
	StockCount         int                     `json:"stock_count"`
}

// SIGKILL leaves no deferred cleanup path. The successor process reuses the
// same Endpoint identity and durable token root, so rejection cannot be
// attributed to a changed Broker ID or an expired capability. Custody, issuer and stock consumption remain real; accepted State and the
// qualified-worker precondition are explicit fixtures. Installed cgroup
// cleanup is covered by the separate worker lifecycle profile.
func TestTextEndpointCrashDropsVolatileAuthorityAndRetainsSpend(t *testing.T) {
	if root := os.Getenv("ARDENTS_TEXT_ENDPOINT_CRASH_ROOT"); root != "" {
		runEndpointCrashChild(t, root)
		return
	}
	waitNetworkFixtureStart(t)
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	boundary := runAndKillEndpointCrashChild(t, root)
	if boundary.AdmissionActive != 2 || boundary.WorkerGrantActive != 1 || !boundary.JobLive || boundary.StockCount != 1 {
		t.Fatalf("crash precondition missing: admission=%d worker-grant=%d job=%t stock=%d",
			boundary.AdmissionActive, boundary.WorkerGrantActive, boundary.JobLive, boundary.StockCount)
	}

	clock := func() time.Time { return boundary.Observed }
	reopened, err := newEndpoint(setup{NetworkID: boundary.Profile.NetworkID, BrokerID: fixtureID(212),
		ConnectionPrincipal: boundary.Principal, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.closedState = &permissionStateFixture{profile: boundary.Profile}
	reopened.closedTokenRoot = filepath.Join(root, "tokens")
	if active := reopened.admission.Active(); active != 0 {
		t.Fatalf("restart resurrected local admission: %d", active)
	}
	if owner, err := reopened.beginDutyContext(t.Context(), boundary.Capability, boundary.Principal, broker.Connection); err == nil {
		_ = owner.Close()
		t.Fatal("restart accepted a capability from the lost Broker generation")
	}

	capability, err := reopened.Admit(boundary.Principal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := reopened.beginDutyContext(t.Context(), capability, boundary.Principal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	owner.mu.Lock()
	crashPrevious, _ := owner.publication.pair.PreviousLocked()
	fresh := !owner.tokens.PermissionLocked().Present() && owner.job == nil && owner.verifiedJob == nil && owner.sourceSet == nil &&
		owner.source.CurrentLocked() == nil && owner.publication.pair.CurrentLocked() == nil && crashPrevious == nil &&
		introduction.ActiveExchangeCount(&owner.introduction.exchanges) == 0 && introduction.ReplayCount(&owner.introduction.admission) == 0 && owner.descriptorHistory.Cleared()
	owner.mu.Unlock()
	if !fresh {
		t.Fatal("restart populated volatile permission, job, join, or Route state")
	}
	job, _ := attachPermissionJob(t, owner, reopened)
	owner.retireJob(job)
	if err := owner.finishJobCleanup(job, nil); err != nil {
		t.Fatal(err)
	}
	requestRaw, digest, err := owner.tokens.Request([3]uint32{34, 34, 0})
	if err != nil {
		t.Fatal(err)
	}
	request, err := admission.DecodePermissionRequest(requestRaw)
	if err != nil {
		t.Fatal(err)
	}
	oldRequest, err := admission.DecodePermissionRequest(boundary.PermissionRequest)
	if err != nil {
		t.Fatal(err)
	}
	if digest == boundary.PermissionDigest || request.Permission.PermissionID == oldRequest.Permission.PermissionID ||
		request.Permission.HolderKey == oldRequest.Permission.HolderKey {
		t.Fatal("restart reused the lost context permission or holder")
	}
	if err := owner.tokens.Import(boundary.PermissionDigest, boundary.PermissionResponse); err == nil {
		t.Fatal("restart restored a response through the old request digest")
	}
	if err := owner.tokens.Import(digest, boundary.PermissionResponse); err == nil {
		t.Fatal("restart rebound the old response to the fresh holder")
	}
	owner.mu.Lock()
	stock := owner.tokens.PermissionLocked().StockCountForDuty(boundary.Profile.Digest, boundary.Receiver, boundary.Duty, 2)
	owner.mu.Unlock()
	if stock != 0 {
		t.Fatal("restart resurrected token stock")
	}

	journal, err := reopened.tokenJournal()
	if err != nil {
		t.Fatal(err)
	}
	record := attempts.Attempt{Profile: boundary.AttemptProfile, Receiver: boundary.Receiver, Duty: boundary.Duty,
		Window: boundary.Window, Class: 2, Nonce: boundary.Nonce}
	if err := journal.Mark(boundary.Token, record); err == nil || err.Error() != "text token already potentially spent" {
		t.Fatalf("restart must refuse the retained spend, not a changed clock or binding: %v", err)
	}
	receipts := readTokenReceipts(t, reopened.closedTokenRoot, reopened.network)
	if len(receipts) != 1 {
		t.Fatalf("restart changed durable spend journal: %d records", len(receipts))
	}
}

func runAndKillEndpointCrashChild(t *testing.T, root string) endpointCrashBoundary {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "-test.run=^TestTextEndpointCrashDropsVolatileAuthorityAndRetainsSpend$", "-test.timeout=30s")
	command.Env = append(os.Environ(), "ARDENTS_TEXT_ENDPOINT_CRASH_ROOT="+root)
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	joined := false
	defer func() {
		if !joined {
			_ = command.Process.Kill()
			<-done
		}
	}()
	path := filepath.Join(root, "boundary.json")
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	var boundary endpointCrashBoundary
	for boundary.Profile.NetworkID == [32]byte{} {
		raw, readErr := os.ReadFile(path)
		if readErr == nil {
			if err := json.Unmarshal(raw, &boundary); err != nil {
				t.Fatal(err)
			}
			break
		}
		if !errors.Is(readErr, os.ErrNotExist) {
			t.Fatal(readErr)
		}
		select {
		case err := <-done:
			joined = true
			t.Fatalf("child ended before crash boundary: %v / %s", err, output.Bytes())
		case <-ctx.Done():
			t.Fatalf("crash boundary deadline: %s", output.Bytes())
		case <-tick.C:
		}
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	killed := <-done
	joined = true
	var exit *exec.ExitError
	if !errors.As(killed, &exit) {
		t.Fatalf("child was not killed: %v", killed)
	}
	status, ok := exit.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatalf("not SIGKILL: %v", killed)
	}
	return boundary
}

func runEndpointCrashChild(t *testing.T, root string) {
	t.Helper()
	if err := os.Mkdir(filepath.Join(root, "tokens"), 0o700); err != nil {
		t.Fatal(err)
	}
	// The parent reserves enough of the permission hour before launching us.
	// Real Nodes, Custody, blind issuance and the stock consumer reach the crash
	// boundary; only accepted State and worker qualification are fixture seams.
	endpoint, owner, source := startRoleNetwork(t, roleNetworkFixture{
		carrier: carrier.ClosedCarrierTCP, reservedWindow: true,
	})
	endpoint.closedTokenRoot = filepath.Join(root, "tokens")
	profile := source.view.Profile
	principal := fixtureID(211)
	_, workerGrant := attachPermissionJob(t, owner, endpoint)
	if _, err := workerGrant.Admit(fixtureID(240), broker.Connection); err != nil {
		t.Fatal(err)
	}
	requestRaw, digest, err := owner.tokens.Request([3]uint32{34, 34, 0})
	if err != nil {
		t.Fatal(err)
	}
	permissionRaw, err := admission.EncodePermission(owner.tokens.PermissionLocked().Grant())
	if err != nil {
		t.Fatal(err)
	}
	selection := selectSource(t, owner)
	receiver := selection.EntryNodeID
	if err := owner.issueTokens(t.Context(), [][32]byte{receiver, receiver}, 2); err != nil {
		t.Fatal(err)
	}
	var duty uint64
	for _, node := range source.view.Nodes[:source.view.NodeCount] {
		if node.NodeID == receiver {
			duty = node.DutyGeneration
		}
	}
	hello := ardp.Hello{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest,
		ProfileDigest: profile.Digest, RecipientNodeID: receiver, RecipientDutyGeneration: duty,
		Purpose: ardp.PurposeForwarding, ChannelNonce: fixtureID(254), Deadline: profile.NotAfter}
	owner.mu.Lock()
	token, err := owner.tokens.TakeTokenLocked(profile, time.Now(), hello, 2, t.Context())
	stockCount := owner.tokens.PermissionLocked().StockCountForDuty(profile.Digest, receiver, duty, 2)
	owner.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	oldCapability, err := endpoint.Admit(principal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	record := attempts.Attempt{Profile: profile.Digest, Receiver: receiver, Duty: duty,
		Window: owner.tokens.PermissionLocked().Grant().NotBefore, Class: 2, Nonce: hello.ChannelNonce}
	boundary := endpointCrashBoundary{Profile: profile, Principal: principal, Capability: oldCapability,
		PermissionRequest: requestRaw, PermissionResponse: permissionRaw, PermissionDigest: digest,
		Token: token, AttemptProfile: record.Profile, Receiver: record.Receiver, Nonce: record.Nonce,
		Duty: record.Duty, Window: record.Window, Observed: time.Now().UTC(), AdmissionActive: endpoint.admission.Active(),
		WorkerGrantActive: workerGrant.Active(), JobLive: owner.job != nil, StockCount: stockCount}
	raw, err := json.Marshal(boundary)
	if err != nil {
		t.Fatal(err)
	}
	pending := filepath.Join(root, "boundary.pending")
	if err := os.WriteFile(pending, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(pending, filepath.Join(root, "boundary.json")); err != nil {
		t.Fatal(err)
	}
	directory, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(directory.Sync(), directory.Close()); err != nil {
		t.Fatal(err)
	}
	<-time.After(time.Minute)
	t.Fatal("parent did not terminate child")
}

func attachPermissionJob(t *testing.T, owner *dutyContext, endpoint *endpoint) (*jobIdentity, *broker.Broker) {
	t.Helper()
	job, err := owner.beginJob(endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := broker.New(broker.Config{ID: fixtureID(255),
		Grants: []broker.Grant{{Principal: fixtureID(240), Surface: broker.Connection}}})
	if err != nil {
		t.Fatal(err)
	}
	job.workerGrant = grant
	owner.mu.Lock()
	owner.verifiedJob = job
	owner.mu.Unlock()
	return job, grant
}
