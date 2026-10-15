//go:build linux && text_worker_installed

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

func TestInstalledServicePublication(t *testing.T) {
	carrier := os.Getenv("ARDENTS_EXECUTION_START_CARRIER")
	if carrier != string(transport.ClosedCarrierTCP) && carrier != string(transport.ClosedCarrierQUIC) {
		t.Fatal("invalid environment: independently selected installed Carrier required")
	}
	t.Run(carrier, func(t *testing.T) { runInstalledServiceStartup(t, true, carrier) })
}

func installedServiceNetwork(t *testing.T, carrier string) (*networkAdmissionFixture, *reachability.Store) {
	f, reservations, certificates := publicationRouteNetwork(t, transport.CarrierProfile(carrier), true)
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), Network: f.profile.NetworkID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	installedPublisherReceivers(t, f, reservations, certificates, store)
	return f, store
}

func installedServiceDeclaration(t *testing.T, raw, base string) string {
	quoted := func(value string) string {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	// Preserve the original fixture's canonical declaration field order and
	// newline. Re-marshalling an untyped map changes that Installation contract.
	for _, change := range [][2]string{
		{`"role":"reader",`, ""},
		{`"maxima":[1,0,0]`, `"maxima":[32,32,0]`},
		{`"publisher_permission":{"request_path":"","response_path":"","maxima":[0,0,0]}`,
			`"publisher_permission":{"request_path":` + quoted(filepath.Join(base, "publisher-permission", "request")) + `,"response_path":` + quoted(filepath.Join(base, "publisher-permission", "response")) + `,"maxima":[32,32,8]}`},
		{`"administration_principal":""`, `"administration_principal":` + quoted(hex.EncodeToString(bytes.Repeat([]byte{9}, 32)))},
		{`"administration_socket":""`, `"administration_socket":` + quoted(filepath.Join(base, "administration", "socket"))},
		{`"publication_root":""`, `"publication_root":` + quoted(filepath.Join(base, "publication")) + `,"service_instance_root":` + quoted(filepath.Join(base, "instance"))},
	} {
		if strings.Count(raw, change[0]) != 1 {
			t.Fatal("installed declaration fixture changed")
		}
		raw = strings.Replace(raw, change[0], change[1], 1)
	}
	return raw
}

func installedServicePermissions(t *testing.T, base string, original [3]string, uid, gid uint64, approve func(string, string, []byte) []byte, dual bool) {
	roles := []string{"reader"}
	if dual {
		roles = append(roles, "publisher")
	}
	for _, role := range roles {
		installedStartupWait(t, func() ([]byte, bool) {
			raw, err := exec.CommandContext(t.Context(), "journalctl", "--no-pager", "-o", "cat", "_SYSTEMD_INVOCATION_ID="+original[1]).CombinedOutput()
			if err != nil {
				t.Fatal("original permission journal", err)
			}
			return raw, installedServiceEvent(raw, "permission-pending", role)
		})
		directory := filepath.Join(base, "permission")
		if role == "publisher" {
			directory = filepath.Join(base, "publisher-permission")
		}
		request, err := os.ReadFile(filepath.Join(directory, "request"))
		if err != nil {
			t.Fatal(err)
		}
		response := approve(base, role, request)
		pending := filepath.Join(directory, "response.pending")
		installedStartupWrite(t, pending, response, 0600)
		if err := os.Chown(pending, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
		installedStartupSync(t, pending)
		if _, err := os.Lstat(filepath.Join(directory, "response")); !os.IsNotExist(err) {
			t.Fatal("refuse existing permission response", err)
		}
		if err := os.Rename(pending, filepath.Join(directory, "response")); err != nil {
			t.Fatal(err)
		}
		installedStartupSync(t, directory)
		installedStartupWait(t, func() ([]byte, bool) {
			raw, err := exec.CommandContext(t.Context(), "journalctl", "--no-pager", "-o", "cat", "_SYSTEMD_INVOCATION_ID="+original[1]).CombinedOutput()
			if err != nil {
				t.Fatal(err)
			}
			return raw, installedServiceEvent(raw, "permission-accepted", role)
		})
	}
}

// Approval stays with the fixture authority. A participant receives only the
// signed response to its actual qualified request, never this signing key.
func installedServiceApproval(t *testing.T, base, role string, request []byte, f *networkAdmissionFixture) []byte {
	t.Helper()
	prepared, err := allocation.Prepare(request, f.profile.NetworkID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	decision, err := prepared.Decide(nil, f.profile.IssuanceAuthorityKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(base, "allocation-"+role)
	installedStartupWrite(t, journal, decision.Journal(), 0600)
	readback, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(readback, decision.Journal()) {
		t.Fatal("allocation durable readback", err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(f.spec.Authority, admission.PermissionTranscript(permission)))
	response, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func installedServiceEvent(raw []byte, phase, role string) bool {
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		var event map[string]string
		if json.Unmarshal(line, &event) == nil && event["phase"] == phase && event["role"] == role {
			return true
		}
	}
	return false
}

// Host generation and external signed approval are distinct genuine commands;
// the fixture signature is no private Instance or qualified worker substitute.
func installedServiceInstance(t *testing.T, program string, credential *syscall.Credential, base string, network [32]byte) {
	before := time.Now().UTC().Truncate(time.Second)
	raw := installedStartupCommand(t, program, credential, "instance-request-prepared", "publication", "instance-initialize",
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
	public := authority[32:]
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), public...))
	value := append([]byte{0, 3}, public...)
	value = append(value, target[:]...)
	value = append(value, request.InstancePublic[:]...)
	for _, number := range []uint64{1, uint64(request.NotBefore), uint64(request.NotAfter)} {
		value = binary.BigEndian.AppendUint64(value, number)
	}
	value = append(value, network[:]...)
	value = binary.BigEndian.AppendUint32(value, 3)
	value = append(value, ed25519.Sign(authority, value)...)
	response := append([]byte("ardents-service-instance-response-v3\x00"), request.Commitment[:]...)
	response = append(response, value...)
	path := filepath.Join(base, "bootstrap", "credential-response")
	installedStartupWrite(t, path, response, 0644)
	installedStartupCommand(t, program, credential, "instance-response-accepted", "publication", "instance-accept", filepath.Join(base, "instance"), path)
}

func installedServicePublication(t *testing.T, base string, original [3]string) {
	socket := filepath.Join(base, "administration", "socket")
	installedStartupWait(t, func() ([]byte, bool) { _, err := os.Lstat(socket); return nil, err == nil })
	requestLink := func() (string, error) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		return administration.RequestPublishedLink(ctx, socket)
	}
	if _, err := requestLink(); err == nil {
		t.Fatal("unpublished owner supplied a Link")
	}
	ctx, cancel := context.WithTimeout(t.Context(), administration.SnapshotTimeout)
	if _, err := administration.RequestSnapshot(ctx, socket, []byte("genuine installed snapshot\n")); err != nil {
		cancel()
		t.Fatal("installed snapshot publication", err)
	}
	cancel()
	link, err := requestLink()
	if err != nil || link == "" {
		t.Fatal("actual committed Link", err)
	}
	if _, _, err := publication.ParseTargetLink(link); err != nil {
		t.Fatal("canonical Link", err)
	}
	ctx, cancel = context.WithTimeout(t.Context(), 15*time.Second)
	if _, err := administration.Request(ctx, socket, administration.Withdraw); err != nil {
		cancel()
		t.Fatal("installed withdrawal", err)
	}
	cancel()
	if _, err := requestLink(); err == nil {
		t.Fatal("withdrawn publication retained accepting Link")
	}
	if installedStartupIdentity(t) != original {
		t.Fatal("withdrawal replaced or retired the installed participant")
	}
	t.Log("original installed qualified Publisher published via real issuer/REGISTER/Store ACK and withdrew; no Reader Service stream accepted")
}

func installedPairRefreshObserver(t *testing.T, store *reachability.Store, f *networkAdmissionFixture, link string, publishStarted time.Time) func() {
	first := installedPairStoredProof(t, store, f, link)
	return func() { installedPairAwaitRefresh(t, store, f, link, first, publishStarted) }
}

func installedPairStoredProof(t *testing.T, store *reachability.Store, f *networkAdmissionFixture, link string) reachability.Descriptor {
	t.Helper()
	network, target, err := publication.ParseTargetLink(link)
	if err != nil || network != f.profile.NetworkID {
		t.Fatal("installed Link lacks original Network", err)
	}
	now := time.Now()
	raw, outcome, err := store.Lookup(target, f.profile.Digest, now)
	if err != nil || outcome != reachability.Accepted {
		t.Fatal("installed publication lacks actual accepted Store proof", outcome, err)
	}
	proof, err := reachability.Verify(raw, target, network, f.profile.Digest, now)
	if err != nil {
		t.Fatal("installed Store proof failed independent verification", err)
	}
	return proof
}

func installedPairAwaitRefresh(t *testing.T, store *reachability.Store, f *networkAdmissionFixture, link string, first reachability.Descriptor, publishStarted time.Time) {
	t.Helper()
	// No forced Refresh, clock adjustment or shortened product scheduler. This
	// observer alone polls the actual receiving Store under the timing profile.
	deadline := time.Now().Add(6 * time.Minute)
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		proof := installedPairStoredProof(t, store, f, link)
		if proof.Introduction.Revision > first.Introduction.Revision {
			// Registration cannot predate this actual Snapshot request. Its
			// canonical whole-second creation may round down by less than 1s.
			if time.Now().Before(publishStarted.Add(299 * time.Second)) {
				t.Fatal("installed refresh preceded original creation+300s lower bound")
			}
			if proof.Introduction.Revision != first.Introduction.Revision+1 || proof.PublicationDigest != first.PublicationDigest ||
				proof.Introduction.Slot == first.Introduction.Slot || proof.Introduction.RecipientKey == first.Introduction.RecipientKey {
				t.Fatal("installed automatic refresh changed Publication or reused recipient")
			}
			t.Logf("actual installed Store acknowledged automatic revision %d -> %d", first.Introduction.Revision, proof.Introduction.Revision)
			return
		}
		if proof.Digest() != first.Digest() {
			t.Fatal("installed Store changed original revision bytes")
		}
		if !time.Now().Before(deadline) {
			t.Fatal("installed automatic refresh lacked higher acknowledged revision")
		}
		select {
		case <-t.Context().Done():
			t.Fatal("installed refresh observation interrupted", t.Context().Err())
		case <-tick.C:
		}
	}
}
