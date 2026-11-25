//go:build linux && endpoint_launch_native

package endpoint

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// LaunchStand identifies one installed Endpoint and its prepared replacement.
type LaunchStand struct {
	NetworkID                                              [32]byte
	Base                                                   string
	EtcDir                                                 string
	Binary                                                 string
	InstallationRoot                                       string
	Program                                                string
	Manifest                                               string
	Pin                                                    string
	UID, GID                                               uint32
	PermissionDir                                          string
	PublisherDir                                           string
	ApplicationSock                                        string
	AdministrationSock                                     string
	SuccessorRequest, SuccessorRoot, SuccessorDistribution string
}

// ProvisionLaunchStand prepares signed installation inputs and stages the native systemd unit.
func ProvisionLaunchStand(t *testing.T, binary string, refreshMS int, spec networkfixture.ClosedSpec, successor bool, predecessor *EnrollmentV5Artifacts) *LaunchStand {
	t.Helper()
	stand := &LaunchStand{Binary: binary, NetworkID: spec.NetworkID}
	stand.Base = filepath.Join("/var/lib/ardents", fmt.Sprintf("ep-launch-%d-%d", os.Getpid(), time.Now().UnixNano()))
	stand.EtcDir = filepath.Join("/etc/ardents", fmt.Sprintf("ep-launch-%d-%d", os.Getpid(), time.Now().UnixNano()))
	stand.InstallationRoot = filepath.Join(stand.Base, "installation")
	floors := filepath.Join(stand.Base, "floors")
	for _, dir := range []string{stand.Base, stand.EtcDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Registered first so joined cleanup runs last (LIFO), after the clock and
	// Source servers have stopped, before the root directories are removed.
	t.Cleanup(func() { stand.cleanup(t) })

	// Genuine Network authority + signed closed Epoch/profile.
	epoch, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	var authority [32]byte
	copy(authority[:], spec.Authority.Public().(ed25519.PublicKey))

	// Independent clock observation file kept fresh by a joined controller.
	clock := filepath.Join(stand.Base, "clock")
	WriteLaunchFile(t, clock, nil, 0644)
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

	plan := &state.AuthorityDeclaration{Root: filepath.Join(stand.Base, "state"), NetworkID: stand.NetworkID,
		Authorities: [][32]byte{authority}, Threshold: 1, ProfileAuthority: authority, ClockObservationFile: clock}
	config, err := plan.Config()
	if err != nil {
		t.Fatal(err)
	}

	sources := StartLaunchSources(t, stand.Base, stand.EtcDir, config, epoch.Epoch.Raw, epoch.Epoch.Inputs, epoch.Epoch.Materials)

	releases := buildLaunchReleases(t, stand.Binary, stand.Base, successor, predecessor)
	stand.SuccessorDistribution = releases.Successor
	request := string(InstallationRequest(t, releases.Initial, releases.Pin, floors))
	request = DualInstallationRequest(t, request, stand.Base)
	quoted := func(s string) string {
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	request = BindInstallationNetwork(t, request, InstallationNetwork{
		Base: stand.Base, InstallationRoot: stand.InstallationRoot, StateRoot: plan.Root, ClockFile: clock,
		ClientCertificate: sources.Certificate, ClientKey: sources.Key,
		Network: plan.NetworkID, Authority: authority, Seed: spec.Seed, Addresses: sources.Addresses, SourcePins: sources.Pins,
		ObservedAt: time.Now(), RefreshMilliseconds: refreshMS,
	})
	// The request lives below /etc/ardents: provision reads it as a root-owned
	// canonical declaration, and the platform launch declaration binds it as its
	// Configuration (the platform check requires Configuration below /etc/ardents).
	requestPath := filepath.Join(stand.EtcDir, "request.json")
	WriteLaunchFile(t, requestPath, []byte(request), 0600)
	stand.PermissionDir = filepath.Join(stand.Base, "permission")
	stand.PublisherDir = filepath.Join(stand.Base, "publisher-permission")
	stand.ApplicationSock = filepath.Join(stand.Base, "application", "reader")
	stand.AdministrationSock = filepath.Join(stand.Base, "administration", "socket")

	if successor {
		stand.SuccessorRoot = filepath.Join(stand.Base, "successor")
		candidate := strings.Replace(request, quoted(releases.Initial), quoted(stand.SuccessorDistribution), 1)
		candidate = strings.Replace(candidate, quoted(stand.InstallationRoot), quoted(stand.SuccessorRoot), 1)
		candidate = strings.Replace(candidate, ",\"manifest_sha256\":"+quoted(releases.Pin), "", 1)
		successorConfig := filepath.Join(stand.EtcDir, "successor")
		if err := os.Mkdir(successorConfig, 0755); err != nil {
			t.Fatal(err)
		}
		stand.SuccessorRequest = filepath.Join(successorConfig, "request.json")
		WriteLaunchFile(t, stand.SuccessorRequest, []byte(candidate), 0600)
	}

	stand.UID, stand.GID = StageNativeEndpoint(t, requestPath, releases.Program, releases.Pin, releases.Candidate, sources.Key)
	selection, err := os.ReadFile(filepath.Join(stand.InstallationRoot, "selection.json"))
	if err != nil {
		t.Fatal(err)
	}
	var selected struct {
		GenerationDigest string `json:"generation_digest"`
	}
	if err := json.Unmarshal(selection, &selected); err != nil || len(selected.GenerationDigest) != 64 {
		t.Fatalf("selection: %v: %s", err, selection)
	}
	stand.Program = filepath.Join(stand.InstallationRoot, "generations", selected.GenerationDigest, "ardents-linux-amd64")
	PrepareLaunchRoots(t, stand.Base, stand.Program, request, "platform-led-launch-fixture", stand.UID, stand.GID,
		epoch.Epoch.Raw, epoch.Profile, epoch.Epoch.Inputs, epoch.Epoch.Materials)

	stand.Manifest, stand.Pin = InstallNativeUnit(t, requestPath, releases.Candidate, stand.Program)
	return stand
}

func (s *LaunchStand) cleanup(t *testing.T) {
	t.Helper()
	_ = exec.Command("systemctl", "stop", "ardents-endpoint.service").Run()
	_ = exec.Command("systemctl", "stop", "ardents-text-reader.socket", "ardents-text-publisher.socket").Run()
	_ = exec.Command("pkill", "-9", "-f", "ardents-linux-amd64").Run()
	_ = exec.Command("pkill", "-9", "-f", "ardents-text").Run()
	// Remove the main unit AND every fixed resource/managed path the delivery
	// staged, so a subsequent initial provision finds them absent (it never
	// adopts existing state). preflightInitialEffects checks each of these.
	for _, path := range []string{
		"/etc/systemd/system/ardents-endpoint.service",
		"/etc/systemd/system/ardents-text-reader@.service",
		"/etc/systemd/system/ardents-text-publisher@.service",
		"/etc/systemd/system/ardents-text-reader.socket",
		"/etc/systemd/system/ardents-text-publisher.socket",
		"/usr/share/polkit-1/rules.d/50-ardents-text.rules",
		"/usr/lib/tmpfiles.d/ardents-text.conf",
		"/etc/ardents/text-worker-artifact.json",
	} {
		_ = os.Remove(path)
	}
	_ = os.RemoveAll("/usr/lib/ardents/text-worker-root")
	_ = os.RemoveAll("/run/ardents-text")
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("systemctl", "reset-failed", "ardents-endpoint.service").Run()
	// Socket activation spawns transient worker instances; reset every loaded
	// ardents-text unit so the next initial provision observes none loaded.
	_ = exec.Command("bash", "-c", "systemctl list-units --all 'ardents-text-*' --plain --no-legend | awk '{print $1}' | xargs -r systemctl reset-failed").Run()
	_ = exec.Command("userdel", "-f", "ardents-endpoint").Run()
	_ = exec.Command("groupdel", "ardents-endpoint").Run()
	_ = os.RemoveAll(s.EtcDir)
	_ = os.RemoveAll(s.Base)
}
