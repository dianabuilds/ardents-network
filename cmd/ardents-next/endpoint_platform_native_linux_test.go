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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/installation"
	requestinput "github.com/dianabuilds/ardents-network/internal/successor/installation/request"
	endpointunit "github.com/dianabuilds/ardents-network/internal/successor/installation/unit"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
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
	// Positive: the actual privileged consumer accepts the genuine provisioned
	// platform delivery against the real system manager and retained Release.
	// This first evaluation obtains a fresh release-accepted program proof and
	// advances the retained floor; causal refusals below refuse before Release.
	out, err := exec.Command(stand.binary, "platform", "check", stand.manifest, stand.pin).CombinedOutput()
	if err != nil {
		t.Fatalf("genuine F-25 platform check refused: %v\n%s", err, out)
	}
	t.Logf("F-25 platform check accepted against real systemd; launch pin=%s", stand.pin)

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
	firstBundle, err := enrollment.Verify(ctx, enrollment.Request{BundleRoot: firstDir, ExecutablePath: firstProgram, ManifestSHA256: firstPin, Scope: enrollment.Headless})
	if err != nil {
		t.Fatal("genuine initial enrollment:", err)
	}
	firstInput, ok := initialReleaseInputs(firstBundle, ref)
	if !ok {
		t.Fatal("initial release projection refused")
	}

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
	verifier, err := release.Open(stand.releaseFloorRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generationauthorization.AuthenticateInitial(ctx, verifier, firstBundle, firstInput); err != nil {
		t.Fatal("genuine initial Release authorization:", err)
	}
	if err := verifier.Close(); err != nil {
		t.Fatal(err)
	}

	// Version 2 is the enrolled protected candidate the platform delivers. Its
	// program bytes are the actual compiled consumer; the generation executable
	// is the same bytes at the selected generation path.
	stand.distribution, _, _ = signedConsumerRelease(t, "linux-amd64", true, true, 2, resources, authority, 2, 2)
	candidate, err := enrollment.ReadCandidate(ctx, stand.distribution, enrollment.Headless)
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
	stand.requestPath = filepath.Join(etcDir, "request.json")
	stand.manifest = filepath.Join(etcDir, "launch.json")
	stand.certPath = filepath.Join(etcDir, "client.crt")
	stand.keyPath = filepath.Join(etcDir, "client.key")
	stand.decl = endpointPlatformNativeDeclaration(t, stand)
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
	t.Cleanup(func() { cleanupEndpointPlatformNativeStand(t, stand, etcDir) })
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
