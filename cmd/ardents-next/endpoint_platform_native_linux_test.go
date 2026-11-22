//go:build linux && endpoint_platform_native

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	requestinput "github.com/dianabuilds/ardents-network/internal/endpoint/plan"
	endpointunit "github.com/dianabuilds/ardents-network/internal/endpoint/unit"
	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

// This native qualification exercises the protected text Endpoint platform-led
// pre-start trust transfer (F-25) against the REAL platform systemd manager on
// an admitted Ubuntu/systemd host. It provisions genuine platform delivery —
// dedicated account, enrolled protected candidate, genuine retained Release
// floors, the selected generation executable and the rendered platform main
// unit loaded in the system manager — then runs the actual privileged
// `ardents-next platform check` consumer. It is component/acceptance evidence
// with fixture Release authority; it does not start the participant, claim
// Service readiness or qualify the installed two-participant privacy journey.
// The transitional installation orchestration is not exercised or retired here.

// endpointPlatformNativeStand retains the provisioned real-host platform delivery
// and its joined cleanup. Every immutable/trust path is root-controlled; the
// dedicated account and loaded system unit are removed before the test returns.
type endpointPlatformNativeStand struct {
	binary           string
	binaryBytes      []byte
	base             string
	distribution     string
	installationRoot string
	releaseFloorRoot string
	generationDir    string
	program          string
	manifest         string
	requestPath      string
	certPath         string
	keyPath          string
	launch           platformLaunch
	decl             requestinput.Declaration
	pin              string
	unit             []byte
	accountUID       int
}

func TestNativeEndpointPlatformPrestartTrustTransfer(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("endpoint platform native qualification requires a root driver")
	}
	if _, err := exec.Command("/usr/bin/busctl", "--system", "--no-pager", "call", "org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss", "org.freedesktop.systemd1.Manager", "Version").Output(); err != nil {
		t.Fatal("real platform systemd manager unavailable:", err)
	}
	stand := provisionEndpointPlatformNativeStand(t)
	t.Run("foreign-generation-path", func(t *testing.T) {
		if out, err := exec.Command(stand.binary, "platform", "unit", stand.manifest).CombinedOutput(); err != nil || !bytes.Equal(out, stand.unit) {
			t.Fatalf("genuine generation unit refused or changed: %v: %s", err, out)
		}
		foreign := stand.launch
		foreign.Program = filepath.Join(stand.installationRoot, "generations", strings.Repeat("0", 64), "ardents-linux-amd64")
		if err := os.Mkdir(filepath.Dir(foreign.Program), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(foreign.Program, stand.binaryBytes, 0555); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(foreign)
		if err != nil {
			t.Fatal(err)
		}
		manifest := filepath.Join(filepath.Dir(stand.manifest), "foreign-generation.json")
		if err := os.WriteFile(manifest, append(raw, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(stand.binary, "platform", "unit", manifest).CombinedOutput(); err == nil || len(out) != 0 {
			t.Fatalf("foreign generation produced a platform unit: %v: %s", err, out)
		}
	})
	// Positive: the actual privileged consumer accepts the genuine provisioned
	// platform delivery against the real system manager and retained Release.
	// This first evaluation obtains a fresh release-accepted program proof and
	// advances the retained floor; causal refusals below refuse before Release.
	for _, name := range []string{"start-guard.json", "start-socket.json", "start-completion.socket"} {
		t.Run("retained-"+name, func(t *testing.T) {
			path := filepath.Join(stand.installationRoot, name)
			record := []byte("unresolved")
			if err := os.WriteFile(path, record, 0600); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Remove(path); err != nil {
					t.Error(err)
				}
			})
			if out, err := exec.Command(stand.binary, "platform", "check", stand.manifest, stand.pin).CombinedOutput(); err == nil {
				t.Fatalf("platform pre-start accepted unresolved guarded-start artifact %s: %s", name, out)
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, record) {
				t.Fatalf("platform pre-start changed guarded-start artifact %s: %v", name, err)
			}
		})
	}
	out, err := exec.Command(stand.binary, "platform", "check", stand.manifest, stand.pin).CombinedOutput()
	if err != nil {
		t.Fatalf("genuine F-25 platform check refused: %v\n%s", err, out)
	}
	t.Logf("F-25 platform check accepted against real systemd; launch pin=%s", stand.pin)

	// Restart causality: the first evaluation committed the candidate into the
	// retained floor, so a second pre-start check re-evaluates the SAME candidate
	// against the now-current floor and obtains no-update (the program is already
	// the accepted one). The check admits this fresh re-evaluation — it is the
	// current accepted program bound byte-for-byte, not a stored replay of a stale
	// proof. The causal refusals below still refuse a substituted program/unit,
	// foreign pin or authority loss against the committed floor.
	restartOut, restartErr := exec.Command(stand.binary, "platform", "check", stand.manifest, stand.pin).CombinedOutput()
	if restartErr != nil {
		t.Fatalf("F-25 restart no-update re-evaluation refused: %v\n%s", restartErr, restartOut)
	}
	t.Log("F-25 restart no-update re-evaluation accepted against the committed floor")

	for _, fault := range []string{"substituted-program", "substituted-unit", "wrong-account", "missing-account", "authority-loss", "foreign-pin", "substituted-credential"} {
		t.Run(fault, func(t *testing.T) {
			restore := mutateEndpointPlatformNativeStand(t, stand, fault)
			defer restore()
			out, err := exec.Command(stand.binary, "platform", "check", stand.manifest, stand.pin).CombinedOutput()
			if err == nil {
				t.Fatalf("causal refusal missing for %s: %s", fault, out)
			}
		})
	}
}

func TestNativeEndpointPlatformStagesGeneration(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("endpoint platform native qualification requires a root driver")
	}
	stand := provisionEndpointPlatformNativeStand(t)
	decl := stand.decl
	decl.InstallationRoot = filepath.Join(stand.base, "platform-staged")
	roots := requestinput.MutableRoots(decl.Headless)
	for _, root := range roots {
		if err := os.Chown(root, stand.accountUID, stand.accountUID); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(root, 0700); err != nil {
			t.Fatal(err)
		}
	}
	request, err := json.Marshal(decl)
	if err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(filepath.Dir(stand.requestPath), "stage-request.json")
	if err := os.WriteFile(requestPath, append(request, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(decl.BundleRoot, "ardents-linux-amd64")
	newer := decl
	newer.BundleRoot = stand.distribution
	newerManifest, err := os.ReadFile(filepath.Join(newer.BundleRoot, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	newer.ManifestSHA256 = platformDigest(newerManifest)
	raw, err := json.Marshal(newer)
	if err != nil {
		t.Fatal(err)
	}
	newerRequest := filepath.Join(filepath.Dir(requestPath), "newer-stage-request.json")
	if err := os.WriteFile(newerRequest, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(filepath.Join(newer.BundleRoot, "ardents-linux-amd64"), "platform", "stage-endpoint", newerRequest).CombinedOutput(); err == nil {
		t.Fatalf("staging accepted metadata beyond its retained floors: %s", out)
	}
	if _, err := os.Lstat(decl.InstallationRoot); !os.IsNotExist(err) {
		t.Fatalf("refused metadata created the installation root: %v", err)
	}
	verifier, err := release.OpenRetained(decl.ReleaseFloorRoot)
	if err != nil {
		t.Fatal(err)
	}
	floors, err := verifier.CurrentFloors(t.Context())
	if err = errors.Join(err, verifier.Close()); err != nil || floors.TargetsVersion != 1 {
		t.Fatalf("refused staging changed the target floor: version=%d err=%v", floors.TargetsVersion, err)
	}
	marker := filepath.Join(roots[0], "existing-history")
	if err := os.WriteFile(marker, []byte("retain"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(program, "platform", "stage-endpoint", requestPath).CombinedOutput(); err == nil {
		t.Fatalf("nonempty platform root accepted: %s", out)
	}
	if _, err := os.Lstat(decl.InstallationRoot); !os.IsNotExist(err) {
		t.Fatalf("refused staging created the installation root: %v", err)
	}
	if raw, err := os.ReadFile(marker); err != nil || string(raw) != "retain" {
		t.Fatalf("refused staging changed retained data: %v", err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(program, "platform", "stage-endpoint", requestPath).CombinedOutput(); err != nil {
		t.Fatalf("genuine platform generation staging refused: %v: %s", err, out)
	}
	selectedPath := filepath.Join(decl.InstallationRoot, "selection.json")
	selectedBytes, err := os.ReadFile(selectedPath)
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := os.ReadFile(filepath.Join(decl.BundleRoot, "protected-endpoint.json"))
	if err != nil {
		t.Fatal(err)
	}
	digest := platformDigest(descriptor)
	if !bytes.Contains(selectedBytes, []byte(digest)) {
		t.Fatal("selection does not identify the independently pinned descriptor")
	}
	sealed := filepath.Join(decl.InstallationRoot, "generations", digest)
	entries, err := os.ReadDir(sealed)
	if err != nil || len(entries) != 15 {
		t.Fatalf("sealed inventory incomplete: entries=%d err=%v", len(entries), err)
	}
	for _, path := range []string{"writer.lock", "preparation", "journals", "transition.json", "start-guard.json", "start-socket.json", "start-completion.socket"} {
		if _, err := os.Lstat(filepath.Join(decl.InstallationRoot, path)); !os.IsNotExist(err) {
			t.Fatalf("platform staging created transitional artifact %s: %v", path, err)
		}
	}
	if out, err := exec.Command(program, "platform", "stage-endpoint", requestPath).CombinedOutput(); err == nil {
		t.Fatalf("repeat staging accepted: %s", out)
	}
	if raw, err := os.ReadFile(selectedPath); err != nil || !bytes.Equal(raw, selectedBytes) {
		t.Fatalf("repeat staging changed selection: %v", err)
	}
}

// provisionEndpointPlatformNativeStand builds genuine platform delivery on the
// real host. Release floors are established at version 1 through the actual
// initial authorization; the enrolled candidate is version 2 so the pre-start
// check obtains a fresh release-accepted program proof (never a stored replay).
func provisionEndpointPlatformNativeStand(t *testing.T) *endpointPlatformNativeStand {
	t.Helper()
	ctx := t.Context()
	binary := compiledCommand(t)
	binaryBytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	ref := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	authority := newConsumerReleaseAuthority(t, ref.Add(365*24*time.Hour))
	resources := endpointPlatformNativeResources()

	// Version 1 establishes complete retained Release floors through genuine
	// initial-pin provenance and the actual two-target authorization.
	firstDir, firstPin, firstProgram := signedConsumerRelease(t, "linux-amd64", true, true, 1, resources, authority, 1, 1)

	stand := &endpointPlatformNativeStand{binary: binary, binaryBytes: binaryBytes}
	stand.base = filepath.Join("/var/lib/ardents", fmt.Sprintf("ep-proof-%d-%d", os.Getpid(), time.Now().UnixNano()))
	stand.installationRoot = filepath.Join(stand.base, "endpoint")
	stand.releaseFloorRoot = filepath.Join(stand.base, "release-trust")
	for _, dir := range []string{stand.base, stand.installationRoot, stand.releaseFloorRoot, filepath.Join(stand.installationRoot, "generations")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(stand.releaseFloorRoot, 0700); err != nil {
		t.Fatal(err)
	}

	// First-pin distribution must have root-controlled ancestors.
	initialDistribution := filepath.Join(stand.base, "initial-distribution")
	if err := os.Mkdir(initialDistribution, 0700); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(firstDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(firstDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(initialDistribution, entry.Name()), data, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
	}
	firstDir = initialDistribution
	firstProgram = filepath.Join(firstDir, filepath.Base(firstProgram))

	// Version 2 is the enrolled protected candidate the platform delivers. Its
	// program bytes are the actual compiled consumer; the generation executable
	// is the same bytes at the selected generation path.
	stand.distribution, _, _ = signedConsumerRelease(t, "linux-amd64", true, true, 2, resources, authority, 2, 2)
	candidate, err := enrollment.ReadCandidate(ctx, stand.distribution)
	if err != nil {
		t.Fatal("enrolled candidate:", err)
	}
	descriptor, ok := candidate.File("protected-endpoint.json")
	if !ok {
		t.Fatal("protected descriptor absent")
	}
	generationDigest := sha256.Sum256(descriptor)
	stand.generationDir = filepath.Join(stand.installationRoot, "generations", hex.EncodeToString(generationDigest[:]))
	stand.program = filepath.Join(stand.generationDir, "ardents-linux-amd64")
	if err := os.MkdirAll(stand.generationDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stand.program, binaryBytes, 0555); err != nil {
		t.Fatal(err)
	}

	etcDir := filepath.Join("/etc/ardents", fmt.Sprintf("ep-proof-%d-%d", os.Getpid(), time.Now().UnixNano()))
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupEndpointPlatformNativeStand(t, stand, etcDir) })
	stand.requestPath = filepath.Join(etcDir, "request.json")
	stand.manifest = filepath.Join(etcDir, "launch.json")
	stand.certPath = filepath.Join(etcDir, "client.crt")
	stand.keyPath = filepath.Join(etcDir, "client.key")
	stand.decl = endpointPlatformNativeDeclaration(t, stand)
	stand.decl.BundleRoot = firstDir
	stand.decl.ManifestSHA256 = firstPin
	requestRaw, err := json.Marshal(stand.decl)
	if err != nil {
		t.Fatal(err)
	}
	requestRaw = append(requestRaw, '\n')
	if err := os.WriteFile(stand.requestPath, requestRaw, 0644); err != nil {
		t.Fatal(err)
	}
	endpointPlatformNativeCredentials(t, stand)

	roots, err := platformEndpointRoots(stand.decl)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range roots {
		if err := os.MkdirAll(root, 0755); err != nil {
			t.Fatal(err)
		}
	}
	certBytes, err := os.ReadFile(stand.certPath)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := os.ReadFile(stand.keyPath)
	if err != nil {
		t.Fatal(err)
	}
	certDigest := sha256.Sum256(certBytes)
	keyDigest := sha256.Sum256(keyBytes)
	configDigest := sha256.Sum256(requestRaw)
	stand.launch = platformLaunch{
		Role:          "endpoint",
		Distribution:  stand.distribution,
		ReleaseRoot:   stand.releaseFloorRoot,
		Environment:   "h3-test",
		Network:       "new-release-test",
		Program:       stand.program,
		Configuration: stand.requestPath,
		ConfigSHA256:  hex.EncodeToString(configDigest[:]),
		WritableRoots: roots,
		Inputs: map[string]string{
			stand.certPath: hex.EncodeToString(certDigest[:]),
			stand.keyPath:  hex.EncodeToString(keyDigest[:]),
		},
	}
	launchRaw, err := json.Marshal(stand.launch)
	if err != nil {
		t.Fatal(err)
	}
	launchRaw = append(launchRaw, '\n')
	launchDigest := sha256.Sum256(launchRaw)
	stand.pin = hex.EncodeToString(launchDigest[:])
	if err := os.WriteFile(stand.manifest, launchRaw, 0644); err != nil {
		t.Fatal(err)
	}

	initial := stand.launch
	initial.Distribution = firstDir
	initialRaw, err := json.Marshal(initial)
	if err != nil {
		t.Fatal(err)
	}
	initialPath := filepath.Join(etcDir, "initialize.json")
	if err := os.WriteFile(initialPath, append(initialRaw, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	wrongPin := strings.Repeat("0", 64)
	if out, err := exec.Command(firstProgram, "platform", "initialize-trust", initialPath, wrongPin).CombinedOutput(); err == nil {
		t.Fatalf("wrong independent pin accepted: %s", out)
	}
	if out, err := exec.Command(stand.binary, "platform", "initialize-trust", initialPath, firstPin).CombinedOutput(); err == nil {
		t.Fatalf("non-enrolled executable accepted: %s", out)
	}
	if retained, err := release.OpenRetained(stand.releaseFloorRoot); err == nil {
		_ = retained.Close()
		t.Fatal("refused pin created retained Release history")
	}
	if out, err := exec.Command(firstProgram, "platform", "initialize-trust", initialPath, firstPin).CombinedOutput(); err != nil {
		t.Fatalf("platform Endpoint first pin refused: %v: %s", err, out)
	}
	retained, err := release.OpenRetained(stand.releaseFloorRoot)
	if err != nil {
		t.Fatal("platform initialization did not establish complete Release history:", err)
	}
	if err := retained.Close(); err != nil {
		t.Fatal(err)
	}

	template, ok := candidate.File("ardents-endpoint.service")
	if !ok {
		t.Fatal("enrolled endpoint template absent")
	}
	prepared, err := endpointunit.PrepareEndpointTemplate(template, stand.decl.InstallationRoot, stand.generationDir)
	if err != nil {
		t.Fatal(err)
	}
	base, err := prepared.Render(roots)
	if err != nil {
		t.Fatal(err)
	}
	checkProgram := filepath.Join(stand.distribution, "ardents-linux-amd64")
	stand.unit, err = renderEndpointPlatformUnit(base, checkProgram, stand.manifest, stand.pin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("/etc/systemd/system/ardents-endpoint.service", stand.unit, 0644); err != nil {
		t.Fatal(err)
	}
	stand.accountUID = endpointPlatformNativeAccount(t)
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		t.Fatal("daemon-reload:", err)
	}
	return stand
}

// endpointPlatformNativeDeclaration builds a genuine canonical installation
// request using fixture Network authority keys and real Source credentials. The
// F-25 check validates declaration grammar, Release and confinement; it opens no
// Network State, so the declared roots are vacant here (a later start-installed
// stage seeds and accepts them through their actual owners).
func endpointPlatformNativeDeclaration(t *testing.T, stand *endpointPlatformNativeStand) requestinput.Declaration {
	t.Helper()
	f := newNetworkAdmissionFixture(t)
	key := f.spec.Authority.Public().(ed25519.PublicKey)
	keyHex := hex.EncodeToString(key)
	networkID := hex.EncodeToString(f.spec.NetworkID[:])
	roles := filepath.Join(stand.base, "roles", "endpoint")
	run := filepath.Join("/run/ardents", fmt.Sprintf("ep-proof-%d", os.Getpid()))
	clock, closeClock := joinProcessClock(t)
	t.Cleanup(closeClock)
	return requestinput.Declaration{
		Schema:           "ardents-endpoint-installation-request-v1",
		BundleRoot:       stand.distribution,
		InstallationRoot: stand.installationRoot,
		ReleaseFloorRoot: stand.releaseFloorRoot,
		ReferenceTime:    "2030-01-02T03:04:05Z",
		Headless: requestinput.Headless{
			Schema:                  "ardents-headless-runtime-v2",
			Role:                    "",
			NetworkStateRoot:        filepath.Join(roles, "state"),
			EntryStateRoot:          filepath.Join(roles, "entry"),
			LocalRoleStateRoot:      filepath.Join(roles, "participation"),
			TextTokenRoot:           filepath.Join(roles, "token"),
			PublicationRoot:         filepath.Join(roles, "publication"),
			ServiceInstanceRoot:     filepath.Join(roles, "instance"),
			ApplicationSocket:       filepath.Join(run, "application.sock"),
			AdministrationSocket:    filepath.Join(run, "administration.sock"),
			TimeConfidenceFile:      clock,
			NetworkID:               networkID,
			NetworkAuthorities:      []string{keyHex},
			NetworkThreshold:        1,
			NetworkProfile:          "ardents-route-v3",
			ClosedProfileAuthority:  keyHex,
			BrokerID:                strings.Repeat("03", 32),
			ConnectionPrincipal:     strings.Repeat("04", 32),
			AdministrationPrincipal: strings.Repeat("09", 32),
			ReaderPermission: requestinput.Permission{
				RequestPath:  filepath.Join(run, "reader-request"),
				ResponsePath: filepath.Join(run, "reader-response"),
				Maxima:       [3]uint32{1, 0, 0},
			},
			PublisherPermission: requestinput.Permission{
				RequestPath:  filepath.Join(run, "publisher-request"),
				ResponsePath: filepath.Join(run, "publisher-response"),
				Maxima:       [3]uint32{128, 0, 0},
			},
		},
		Source: requestinput.Source{
			Schema:               "ardents-source-plan-v1",
			NetworkID:            networkID,
			AuthorityPublic:      []string{keyHex},
			Threshold:            1,
			ClockObservedAt:      "2030-01-02T03:04:05Z",
			ClockObservationFile: clock,
			OrderSeed:            strings.Repeat("05", 32),
			MaterializationIndex: 0,
			RefreshIntervalMS:    1000,
			RuntimeProfile:       "ardents-route-v3",
			LocalRoleStateRoot:   filepath.Join(roles, "participation"),
			ClientCertificate:    stand.certPath,
			ClientKey:            stand.keyPath,
			Sources: []requestinput.SourceMember{
				{Address: "source-a.example:443", ServerName: "source-a.example", Identity: strings.Repeat("06", 32), Family: "a", EndpointHandle: "a", RootCA: filepath.Join(filepath.Dir(stand.certPath), "source-root.pem"), LeafKeyDigest: strings.Repeat("08", 32)},
				{Address: "source-b.example:443", ServerName: "source-b.example", Identity: strings.Repeat("07", 32), Family: "b", EndpointHandle: "b", RootCA: filepath.Join(filepath.Dir(stand.certPath), "source-root.pem"), LeafKeyDigest: strings.Repeat("08", 32)},
			},
		},
	}
}

func endpointPlatformNativeCredentials(t *testing.T, stand *endpointPlatformNativeStand) {
	t.Helper()
	_, client, _, root, _, _ := sourceNativeTLS(t)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: client.Certificate[0]})
	keyDER, err := x509.MarshalPKCS8PrivateKey(client.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	clear(keyDER)
	if err := os.WriteFile(stand.certPath, certPEM, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stand.keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	clear(keyPEM)
	if err := os.WriteFile(filepath.Join(filepath.Dir(stand.certPath), "source-root.pem"), root, 0644); err != nil {
		t.Fatal(err)
	}
}

// endpointPlatformNativeAccount creates the fixed unprivileged dedicated account
// with a unique 100-999 UID/GID, nologin shell and /nonexistent home. It selects
// a free numeric identity rather than assuming one, and removes it on cleanup.
func endpointPlatformNativeAccount(t *testing.T) int {
	t.Helper()
	uid := 0
	for candidate := 990; candidate >= 100; candidate-- {
		name := fmt.Sprintf("%d", candidate)
		passwdUsed := exec.Command("getent", "passwd", name).Run() == nil
		groupUsed := exec.Command("getent", "group", name).Run() == nil
		if !passwdUsed && !groupUsed {
			uid = candidate
			break
		}
	}
	if uid == 0 {
		t.Fatal("no free unprivileged system UID/GID available")
	}
	if err := exec.Command("groupadd", "-g", fmt.Sprintf("%d", uid), "ardents-endpoint").Run(); err != nil {
		t.Fatal("groupadd:", err)
	}
	if err := exec.Command("useradd", "-u", fmt.Sprintf("%d", uid), "-g", fmt.Sprintf("%d", uid), "-d", "/nonexistent", "-s", "/usr/sbin/nologin", "-M", "ardents-endpoint").Run(); err != nil {
		t.Fatal("useradd:", err)
	}
	return uid
}

func cleanupEndpointPlatformNativeStand(t *testing.T, stand *endpointPlatformNativeStand, etcDir string) {
	t.Helper()
	_ = exec.Command("systemctl", "stop", "ardents-endpoint.service").Run()
	_ = os.Remove("/etc/systemd/system/ardents-endpoint.service")
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("userdel", "ardents-endpoint").Run()
	_ = exec.Command("groupdel", "ardents-endpoint").Run()
	_ = os.RemoveAll(etcDir)
	_ = os.RemoveAll(stand.base)
	_ = os.RemoveAll(filepath.Join("/run/ardents", fmt.Sprintf("ep-proof-%d", os.Getpid())))
}

// mutateEndpointPlatformNativeStand applies one causal fault and returns a restore
// closure. Each fault is a genuine pre-effect substitution the privileged check
// must refuse; the healthy positive proof above establishes the baseline.
func mutateEndpointPlatformNativeStand(t *testing.T, stand *endpointPlatformNativeStand, fault string) func() {
	t.Helper()
	switch fault {
	case "substituted-program":
		original, err := os.ReadFile(stand.program)
		if err != nil {
			t.Fatal(err)
		}
		changed := bytes.Clone(original)
		changed[len(changed)-1] ^= 1
		if err := os.WriteFile(stand.program, changed, 0555); err != nil {
			t.Fatal(err)
		}
		return func() { _ = os.WriteFile(stand.program, original, 0555) }
	case "substituted-unit":
		changed := append(bytes.Clone(stand.unit), []byte("# foreign drop-in comment\n")...)
		if err := os.WriteFile("/etc/systemd/system/ardents-endpoint.service", changed, 0644); err != nil {
			t.Fatal(err)
		}
		_ = exec.Command("systemctl", "daemon-reload").Run()
		return func() {
			_ = os.WriteFile("/etc/systemd/system/ardents-endpoint.service", stand.unit, 0644)
			_ = exec.Command("systemctl", "daemon-reload").Run()
		}
	case "wrong-account":
		if err := exec.Command("usermod", "-s", "/bin/bash", "ardents-endpoint").Run(); err != nil {
			t.Fatal(err)
		}
		return func() { _ = exec.Command("usermod", "-s", "/usr/sbin/nologin", "ardents-endpoint").Run() }
	case "missing-account":
		if err := exec.Command("userdel", "ardents-endpoint").Run(); err != nil {
			t.Fatal(err)
		}
		return func() {
			_ = exec.Command("useradd", "-u", fmt.Sprintf("%d", stand.accountUID), "-g", fmt.Sprintf("%d", stand.accountUID), "-d", "/nonexistent", "-s", "/usr/sbin/nologin", "-M", "ardents-endpoint").Run()
		}
	case "authority-loss":
		if err := os.Rename(stand.releaseFloorRoot, stand.releaseFloorRoot+".held"); err != nil {
			t.Fatal(err)
		}
		return func() { _ = os.Rename(stand.releaseFloorRoot+".held", stand.releaseFloorRoot) }
	case "foreign-pin":
		// Rewrite the launch declaration so its actual digest no longer matches
		// the pin argument the consumer is invoked with; the privileged check
		// must refuse the foreign manifest digest before any trust effect.
		original, err := os.ReadFile(stand.manifest)
		if err != nil {
			t.Fatal(err)
		}
		changed := bytes.Replace(original, []byte(stand.launch.ConfigSHA256), []byte(strings.Repeat("0", 64)), 1)
		if err := os.WriteFile(stand.manifest, changed, 0644); err != nil {
			t.Fatal(err)
		}
		return func() { _ = os.WriteFile(stand.manifest, original, 0644) }
	case "substituted-credential":
		original, err := os.ReadFile(stand.keyPath)
		if err != nil {
			t.Fatal(err)
		}
		changed := bytes.Clone(original)
		changed[len(changed)-1] ^= 1
		if err := os.WriteFile(stand.keyPath, changed, 0600); err != nil {
			t.Fatal(err)
		}
		return func() { _ = os.WriteFile(stand.keyPath, original, 0600) }
	default:
		t.Fatal("unknown fault", fault)
		return func() {}
	}
}
