//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/custody"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// Actual Custody and files; authenticated State and worker qualification remain
// explicit local fixtures. This does not claim command or installed acceptance.
func TestTextPermissionFilesConsumeActualCustodyApproval(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	var clock atomic.Int64
	clock.Store(now.Unix())
	currentTime := func() time.Time { return time.Unix(clock.Load(), 0).UTC() }
	vault, err := custody.Open(custody.VaultConfig{Root: t.TempDir(), Now: currentTime})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := vault.Close(); err != nil {
			t.Error(err)
		}
	})
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateAdmissionAuthority,
		Authority: custody.AuthorityState{Binding: custody.AuthorityBinding{Environment: fixtureID(221), Network: fixtureID(222), Root: fixtureID(223), Kind: custody.AuthorityAdmission}}}, textPermissionSecretFixture{})
	if err != nil {
		t.Fatal(err)
	}
	endpoint, principal := textContextEndpoint(t)
	endpoint.network, endpoint.clock = fixtureID(222), currentTime
	endpoint.closedState = &textPermissionStateFixture{profile: state.ClosedProfileView{NetworkID: endpoint.network,
		StateGeneration: fixtureID(224), StateDigest: fixtureID(225), Digest: fixtureID(226), IssuanceAuthorityKey: created.AdmissionAuthority.Public,
		IssuerNodeID: fixtureID(227), IssuerDutyGeneration: 1, NotBefore: now.Truncate(time.Hour), NotAfter: now.Truncate(time.Hour).Add(6 * time.Hour)}}
	owner := textPermissionContextFixture(t, endpoint, principal, broker.Administration)
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	requestPath, responsePath := filepath.Join(root, "request"), filepath.Join(root, "permission")
	maxima := [3]uint32{64, 64, 16}
	reported := make(chan [32]byte, 1)
	provisioned := make(chan error, 1)
	go func() {
		provisioned <- owner.provisionTextPermission(t.Context(), requestPath, responsePath, maxima, func(_ context.Context, digest [32]byte) error {
			reported <- digest
			return nil
		})
	}()
	var digest [32]byte
	select {
	case digest = <-reported:
	case err := <-provisioned:
		t.Fatalf("permission provisioning ended before request: %v", err)
	}
	select {
	case err := <-provisioned:
		t.Fatalf("permission provisioning completed without approval: %v", err)
	default:
	}

	request, err := os.ReadFile(requestPath)
	if err != nil || sha256.Sum256(request) != digest {
		t.Fatalf("public export differs: %v", err)
	}
	decoded, err := credential.DecodePermissionRequest(request)
	if err != nil || decoded.Role != credential.AllocationPublisher {
		t.Fatalf("publisher request: %v", err)
	}
	if repeated, err := owner.exportTextPermissionFile(t.Context(), requestPath, maxima); err != nil || repeated != digest {
		t.Fatalf("exact export retry: %v", err)
	}
	approved, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationIssueAdmissionPermission,
		RecordID: created.RecordID, Expected: created.Authority.Binding, AdmissionRequest: request, AdmissionRequestCommitment: digest}, textPermissionSecretFixture{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(responsePath, approved.AdmissionPermission, 0600); err != nil {
		t.Fatal(err)
	}
	if err := <-provisioned; err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"caller cancellation", "observer cancellation", "context retirement", "invalid response"} {
		t.Run(failure, func(t *testing.T) {
			waiting := textPermissionContextFixture(t, endpoint, principal, broker.Administration)
			directory := t.TempDir()
			if err := os.Chmod(directory, 0700); err != nil {
				t.Fatal(err)
			}
			response := filepath.Join(directory, "response")
			caller, cancel := context.WithCancel(t.Context())
			defer cancel()
			reported := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- waiting.provisionTextPermission(caller, filepath.Join(directory, "request"), response, maxima, func(reportContext context.Context, _ [32]byte) error {
					close(reported)
					if failure == "observer cancellation" {
						<-reportContext.Done()
						return reportContext.Err()
					}
					if failure == "invalid response" {
						return os.WriteFile(response, []byte("not a permission"), 0600)
					}
					return nil
				})
			}()
			select {
			case <-reported:
			case err := <-done:
				t.Fatalf("request was not reported: %v", err)
			}
			switch failure {
			case "caller cancellation", "observer cancellation":
				cancel()
			case "context retirement":
				if err := waiting.Close(); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("invalid provisioning completed successfully")
				}
				if failure != "invalid response" && !errors.Is(err, context.Canceled) {
					t.Fatalf("lost cancellation: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("provisioning did not stop")
			}
		})
	}
	foreign := textPermissionContextFixture(t, endpoint, principal, broker.Administration)
	_, foreignDigest, err := foreign.requestTextPermission(maxima)
	if err != nil {
		t.Fatal(err)
	}
	if err := foreign.importTextPermissionFile(t.Context(), responsePath, foreignDigest); err == nil {
		t.Fatal("foreign context imported permission")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := owner.importTextPermissionFile(canceled, responsePath, digest); err == nil {
		t.Fatal("canceled import succeeded")
	}
	if _, err := owner.exportTextPermissionFile(canceled, filepath.Join(root, "canceled"), maxima); err == nil {
		t.Fatal("canceled export succeeded")
	}
	if _, err := os.Stat(filepath.Join(root, "canceled")); !os.IsNotExist(err) {
		t.Fatal("canceled export created a file")
	}
	if err := os.WriteFile(requestPath, []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.exportTextPermissionFile(t.Context(), requestPath, maxima); err == nil {
		t.Fatal("export replaced conflicting destination")
	}
	retained, err := os.ReadFile(requestPath)
	if err != nil || !bytes.Equal(retained, []byte("occupied")) {
		t.Fatal("conflicting destination changed")
	}
	clock.Store(now.Truncate(time.Hour).Add(time.Hour).Unix())
	if err := owner.importTextPermissionFile(t.Context(), responsePath, digest); err == nil {
		t.Fatal("expired permission imported")
	}
	clock.Store(now.Unix()) // Keep closure refusal independent of expiry.
	if err := owner.importTextPermissionFile(t.Context(), responsePath, digest); err != nil {
		t.Fatalf("live control before closure: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := owner.importTextPermissionFile(t.Context(), responsePath, digest); err == nil {
		t.Fatal("closed context restored from file")
	}
}

func TestTextPermissionFilesRefuseUnsafeFilesystemInputs(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.Chmod(rootPath, 0700); err != nil {
		t.Fatal(err)
	}
	root, name, err := openTextPermissionDirectory(filepath.Join(rootPath, "response"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	regular := filepath.Join(rootPath, "regular")
	if err := os.WriteFile(regular, []byte("bounded"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"symlink", "hardlink", "fifo", "permissions", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(rootPath, name)
			switch kind {
			case "symlink":
				err = os.Symlink(regular, path)
			case "hardlink":
				err = os.Link(regular, path)
			case "fifo":
				err = syscall.Mkfifo(path, 0600)
			case "permissions":
				err = os.WriteFile(path, []byte("wide"), 0600)
				if err == nil {
					err = os.Chmod(path, 0644)
				}
			case "oversize":
				err = os.WriteFile(path, make([]byte, 229), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Remove(path); err != nil {
					t.Error(err)
				}
			}()
			if _, err := readTextPermissionFile(root, name, 228); err == nil {
				t.Fatal("unsafe input accepted")
			}
		})
	}
	if err := os.Chmod(rootPath, 0755); err != nil {
		t.Fatal(err)
	}
	if opened, _, err := openTextPermissionDirectory(filepath.Join(rootPath, name)); err == nil {
		if err := opened.Close(); err != nil {
			t.Error(err)
		}
		t.Fatal("shared directory accepted")
	}
	if _, _, err := openTextPermissionDirectory("relative/permission"); err == nil {
		t.Fatal("relative path accepted")
	}
}
