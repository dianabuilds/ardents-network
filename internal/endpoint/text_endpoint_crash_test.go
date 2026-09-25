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

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/custody"
	"github.com/dianabuilds/ardents-network/internal/endpoint/durableroot"
	"github.com/dianabuilds/ardents-network/internal/endpoint/tokenjournal"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

type textEndpointCrashBoundary struct {
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
	AdmissionActive    uint32                  `json:"admission_active"`
	WorkerGrantActive  uint32                  `json:"worker_grant_active"`
	JobLive            bool                    `json:"job_live"`
	StockCount         int                     `json:"stock_count"`
}

// SIGKILL leaves no deferred cleanup path. The successor process reuses the
// same Endpoint identity and durable token root, so rejection cannot be
// attributed to a changed Broker ID or an expired capability. Custody and the
// qualified-worker precondition remain explicit fixtures; installed cgroup
// cleanup is covered by the separate worker lifecycle profile.
func TestTextEndpointCrashDropsVolatileAuthorityAndRetainsSpend(t *testing.T) {
	if root := os.Getenv("ARDENTS_TEXT_ENDPOINT_CRASH_ROOT"); root != "" {
		runTextEndpointCrashChild(t, root)
		return
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	boundary := runAndKillTextEndpointCrashChild(t, root)
	if boundary.AdmissionActive != 2 || boundary.WorkerGrantActive != 1 || !boundary.JobLive || boundary.StockCount != 1 {
		t.Fatalf("crash precondition missing: admission=%d worker-grant=%d job=%t stock=%d",
			boundary.AdmissionActive, boundary.WorkerGrantActive, boundary.JobLive, boundary.StockCount)
	}

	clock := func() time.Time { return textEndpointCrashTime() }
	reopened, err := newEndpoint(setup{NetworkID: boundary.Profile.NetworkID, BrokerID: fixtureID(243),
		ConnectionPrincipal: boundary.Principal, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.closedState = &textPermissionStateFixture{profile: boundary.Profile}
	reopened.closedTokenRoot = filepath.Join(root, "tokens")
	if active := reopened.admission.Active(); active != 0 {
		t.Fatalf("restart resurrected local admission: %d", active)
	}
	if owner, err := reopened.beginTextContext(t.Context(), boundary.Capability, boundary.Principal, broker.Connection); err == nil {
		_ = owner.Close()
		t.Fatal("restart accepted a capability from the lost Broker generation")
	}

	capability, err := reopened.Admit(boundary.Principal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := reopened.beginTextContext(t.Context(), capability, boundary.Principal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	owner.mu.Lock()
	fresh := owner.permission == nil && owner.job == nil && owner.verifiedJob == nil && owner.source.set == nil &&
		owner.currentTextSourceLocked() == nil && owner.registration == nil && owner.previousRegistration == nil &&
		owner.introductionExchanges.active == nil && owner.introductionAdmission.replays == nil && owner.descriptorHistory.Cleared()
	owner.mu.Unlock()
	if !fresh {
		t.Fatal("restart populated volatile permission, job, join, or Route state")
	}
	job, _ := attachTextPermissionJob(t, owner, reopened)
	owner.retireJob(job)
	if err := owner.finishJobCleanup(job, nil); err != nil {
		t.Fatal(err)
	}
	requestRaw, digest, err := owner.requestTextPermission([3]uint32{4, 4, 0})
	if err != nil {
		t.Fatal(err)
	}
	request, err := credential.DecodePermissionRequest(requestRaw)
	if err != nil {
		t.Fatal(err)
	}
	oldRequest, err := credential.DecodePermissionRequest(boundary.PermissionRequest)
	if err != nil {
		t.Fatal(err)
	}
	if digest == boundary.PermissionDigest || request.Permission.PermissionID == oldRequest.Permission.PermissionID ||
		request.Permission.HolderKey == oldRequest.Permission.HolderKey {
		t.Fatal("restart reused the lost context permission or holder")
	}
	if err := owner.importTextPermission(boundary.PermissionDigest, boundary.PermissionResponse); err == nil {
		t.Fatal("restart restored a response through the old request digest")
	}
	if err := owner.importTextPermission(digest, boundary.PermissionResponse); err == nil {
		t.Fatal("restart rebound the old response to the fresh holder")
	}
	owner.mu.Lock()
	stock := len(owner.permission.stock)
	owner.mu.Unlock()
	if stock != 0 {
		t.Fatal("restart resurrected token stock")
	}

	journal, err := reopened.textTokenJournal()
	if err != nil {
		t.Fatal(err)
	}
	record := tokenjournal.Attempt{Profile: boundary.AttemptProfile, Receiver: boundary.Receiver, Duty: boundary.Duty,
		Window: boundary.Window, Class: 2, Nonce: boundary.Nonce}
	if err := journal.Mark(boundary.Token, record); err == nil {
		t.Fatal("restart revived a token already durably marked before the crash")
	}
	receipts := readTextTokenReceipts(t, reopened.closedTokenRoot, reopened.network)
	if len(receipts) != 1 {
		t.Fatalf("restart changed durable spend journal: %d records", len(receipts))
	}
}

func runAndKillTextEndpointCrashChild(t *testing.T, root string) textEndpointCrashBoundary {
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
	var boundary textEndpointCrashBoundary
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

func runTextEndpointCrashChild(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"custody", "tokens"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	now := textEndpointCrashTime()
	clock := func() time.Time { return now }
	vault, err := custody.Open(custody.VaultConfig{Root: filepath.Join(root, "custody"), Now: clock})
	if err != nil {
		t.Fatal(err)
	}
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateAdmissionAuthority,
		Authority: custody.AuthorityState{Binding: custody.AuthorityBinding{Environment: fixtureID(244), Network: fixtureID(245),
			Root: fixtureID(246), Kind: custody.AuthorityAdmission}}}, textPermissionSecretFixture{})
	if err != nil {
		t.Fatal(err)
	}
	profile := state.ClosedProfileView{NetworkID: fixtureID(245), StateGeneration: fixtureID(247), StateDigest: fixtureID(248),
		Digest: fixtureID(249), IssuanceAuthorityKey: created.AdmissionAuthority.Public, IssuerNodeID: fixtureID(250),
		IssuerDutyGeneration: 3, NotBefore: now.Truncate(time.Hour), NotAfter: now.Truncate(time.Hour).Add(2 * time.Hour)}
	principal := fixtureID(251)
	endpoint, err := newEndpoint(setup{NetworkID: profile.NetworkID, BrokerID: fixtureID(243),
		ConnectionPrincipal: principal, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	endpoint.closedState = &textPermissionStateFixture{profile: profile}
	ownerCapability, err := endpoint.Admit(principal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := endpoint.beginTextContext(t.Context(), ownerCapability, principal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	_, workerGrant := attachTextPermissionJob(t, owner, endpoint)
	if _, err := workerGrant.Admit(fixtureID(240), broker.Connection); err != nil {
		t.Fatal(err)
	}
	requestRaw, digest, err := owner.requestTextPermission([3]uint32{4, 4, 0})
	if err != nil {
		t.Fatal(err)
	}
	issued, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationIssueAdmissionPermission,
		RecordID: created.RecordID, Expected: created.Authority.Binding, AdmissionRequest: requestRaw,
		AdmissionRequestCommitment: digest}, textPermissionSecretFixture{})
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.importTextPermission(digest, issued.AdmissionPermission); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	owner.permission.stock = []textTokenStock{{tokens: [][]byte{bytes.Repeat([]byte{0x5a}, 354)}}}
	stockCount := len(owner.permission.stock)
	owner.mu.Unlock()
	oldCapability, err := endpoint.Admit(principal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	token := bytes.Repeat([]byte{0xa5}, 354)
	record := tokenjournal.Attempt{Profile: fixtureID(252), Receiver: fixtureID(253), Duty: 7,
		Window: now.Truncate(time.Hour), Class: 2, Nonce: fixtureID(254)}
	journal, err := tokenjournal.Open(filepath.Join(root, "tokens"), profile.NetworkID, clock)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Mark(token, record); err != nil {
		t.Fatal(err)
	}
	boundary := textEndpointCrashBoundary{Profile: profile, Principal: principal, Capability: oldCapability,
		PermissionRequest: requestRaw, PermissionResponse: issued.AdmissionPermission, PermissionDigest: digest,
		Token: token, AttemptProfile: record.Profile, Receiver: record.Receiver, Nonce: record.Nonce,
		Duty: record.Duty, Window: record.Window, AdmissionActive: endpoint.admission.Active(),
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
	if err := durableroot.SyncDirectory(root); err != nil {
		t.Fatal(err)
	}
	<-time.After(time.Minute)
	t.Fatal("parent did not terminate child")
}

func attachTextPermissionJob(t *testing.T, owner *textContext, endpoint *endpoint) (*textJobIdentity, *broker.Broker) {
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

func textEndpointCrashTime() time.Time {
	return time.Date(2030, 8, 9, 10, 11, 12, 0, time.UTC)
}
