//go:build linux && endpoint_pair_native && document_worker_installed

package endpoint

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	releasefixture "github.com/dianabuilds/ardents-network/tests/releasefixture"
)

// These environment variables establish the participant's fixture channel.
const (
	JourneyRootEnv    = "ARDENTS_JOURNEY_ROOT"
	JourneyRoleEnv    = "ARDENTS_JOURNEY_ROLE"
	JourneyNonceEnv   = "ARDENTS_JOURNEY_NONCE"
	JourneyAddressEnv = "ARDENTS_JOURNEY_ADDRESS"
)

// JourneyParticipant describes the Original installed participant observed by the fixture.
type JourneyParticipant struct {
	Base       string
	Original   [3]string
	Principals [3]string
}

// RunJourneyParticipant provisions and platform-led-launches one
// dual-role Endpoint against the shared signed Network, then exercises the
// fixture commands. It reuses the justified delivery staging (installed-stopped,
// no start-guard.json) and the S1 platform launch (platform main unit + canonical
// workers, systemd start without root ACK, F-25 pre-start transfer).
func RunJourneyParticipant(t *testing.T, binary, base, etcDir string, network SignedNetwork,
	approve func(base, role string, request []byte) []byte, exercise func(JourneyParticipant)) {
	t.Helper()
	for _, dir := range []string{base, etcDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	clock := filepath.Join(base, "clock")
	WriteStartupFile(t, clock, nil, 0644)
	stopClock, clockJoined := make(chan struct{}), make(chan error, 1)
	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stopClock:
				clockJoined <- nil
				return
			case now := <-tick.C:
				if err := os.Chtimes(clock, now, now); err != nil {
					clockJoined <- err
					return
				}
			}
		}
	}()
	t.Cleanup(func() {
		close(stopClock)
		if err := <-clockJoined; err != nil {
			t.Error(err)
		}
	})
	authority := network.Authority
	plan := &state.AuthorityDeclaration{Root: filepath.Join(base, "state"), NetworkID: network.NetworkID,
		Authorities: [][32]byte{authority}, Threshold: 1, ProfileAuthority: authority, ClockObservationFile: clock}
	config, err := plan.Config()
	if err != nil {
		t.Fatal(err)
	}
	sources := StartLaunchSources(t, base, etcDir, config, network.Epoch, network.Inputs, network.Materials)
	rootAuthority := releasefixture.NewReleaseAuthority(t, time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC))
	resources := NativeResources()
	resources["ardents-text-linux-amd64"] = BuildNativeWorker(t, os.Getenv("ARDENTS_PREBUILT_DOCUMENT_WORKER"))
	bundle, pin, _ := releasefixture.SignedRelease(t, binary, "linux-amd64", true, true, 1, resources, rootAuthority, 1, 1)
	bundle, initialProgram := CopyNativeDistribution(t, bundle, filepath.Join(base, "initial-distribution"))
	// Advance the signed metadata while retaining the exact staged release and descriptor.
	candidateBundle, _, _ := releasefixture.SignedRelease(t, binary, "linux-amd64", true, true, 1, resources, rootAuthority, 2, 1)
	candidateBundle, _ = CopyNativeDistribution(t, candidateBundle, filepath.Join(base, "candidate-distribution"))
	request := string(InstallationRequest(t, bundle, pin, filepath.Join(base, "floors")))
	request = DualInstallationRequest(t, request, base)
	quoted := func(s string) string {
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	// Two Endpoints require distinct declared principals as well as distinct
	// roots/accounts/invocations. The network-facing authorities (Instance,
	// holder, permission, recipient) are already fresh per participant; replace
	// the shared local Broker/Connection/Administration identity placeholders
	// with this participant's own random declarations.
	var principals [3]string
	for i, placeholder := range []string{strings.Repeat("03", 32), strings.Repeat("04", 32), strings.Repeat("09", 32)} {
		var identity [32]byte
		if _, err := rand.Read(identity[:]); err != nil {
			t.Fatal(err)
		}
		principals[i] = hex.EncodeToString(identity[:])
		if strings.Count(request, quoted(placeholder)) != 1 {
			t.Fatal("journey declaration identity fixture changed")
		}
		request = strings.Replace(request, quoted(placeholder), quoted(principals[i]), 1)
	}
	request = BindInstallationNetwork(t, request, InstallationNetwork{
		Base: base, InstallationRoot: filepath.Join(base, "installation"), StateRoot: plan.Root, ClockFile: clock,
		ClientCertificate: sources.Certificate, ClientKey: sources.Key,
		Network: plan.NetworkID, Authority: authority, Seed: network.Seed, Addresses: sources.Addresses, SourcePins: sources.Pins,
		ObservedAt: time.Now(), RefreshMilliseconds: 60000,
	})
	requestPath := filepath.Join(etcDir, "request.json")
	WriteStartupFile(t, requestPath, []byte(request), 0600)
	uid, gid := StageNativeEndpoint(t, requestPath, initialProgram, pin, candidateBundle, sources.Key)
	installationRoot := filepath.Join(base, "installation")
	selection, err := os.ReadFile(filepath.Join(installationRoot, "selection.json"))
	if err != nil {
		t.Fatal(err)
	}
	var selected struct {
		GenerationDigest string `json:"generation_digest"`
	}
	if err := json.Unmarshal(selection, &selected); err != nil || len(selected.GenerationDigest) != 64 {
		t.Fatalf("selection: %v: %s", err, selection)
	}
	program := filepath.Join(installationRoot, "generations", selected.GenerationDigest, "ardents-linux-amd64")
	credential := &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: []uint32{}}
	PrepareLaunchRoots(t, base, program, request, "platform-led-journey-fixture", uid, gid,
		network.Epoch, network.Profile, network.Inputs, network.Materials)
	InstallNativeUnit(t, requestPath, candidateBundle, program)
	original := StartOriginalEndpoint(t)
	InitializeInstalledServiceInstance(t, program, credential, base, network.NetworkID)
	GrantOriginalPermissions(t, base, original[1], uid, gid, 60*time.Second, approve)
	if current := StartupIdentity(t, t.Context()); current != original {
		t.Fatal("accepted permission lost original invocation", current)
	}
	WaitOriginalReader(t, filepath.Join(base, "application", "reader"), original, uid, gid, 60*time.Second)
	stop := func() {
		if current := StartupIdentity(t, t.Context()); current[2] == "active" {
			if out, err := exec.Command("systemctl", "stop", "ardents-endpoint.service").CombinedOutput(); err != nil {
				t.Errorf("journey stop: %v: %s", err, out)
			}
		}
	}
	t.Cleanup(stop)
	t.Logf("guest platform-led Endpoint reached Service readiness; MainPID=%s InvocationID=%s", original[0], original[1])
	exercise(JourneyParticipant{Base: base, Original: original, Principals: principals})
}

// DumpJourneyJournal logs the installed participant diagnostics.
func DumpJourneyJournal(t *testing.T) {
	t.Helper()
	journal, _ := exec.Command("journalctl", "--no-pager", "-u", "ardents-endpoint.service", "-n", "80", "-o", "cat").CombinedOutput()
	t.Logf("participant journal:\n%s", journal)
	workers, _ := exec.Command("bash", "-c", "journalctl --no-pager -o cat _SYSTEMD_UNIT='ardents-text-*' -n 40 2>/dev/null; systemctl list-units --all 'ardents-text-*' --plain --no-legend").CombinedOutput()
	t.Logf("worker journal/units:\n%s", workers)
}
