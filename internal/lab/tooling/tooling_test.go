package tooling

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/lab/preflight"
)

func TestCapabilityContractRejectsMissingCapability(t *testing.T) {
	t.Parallel()
	if hasEffectiveCapability("0000000000000000", capabilityNetAdmin) {
		t.Fatal("missing NET_ADMIN was accepted")
	}
	if !hasEffectiveCapability("0000000000003000", capabilityNetAdmin) || !hasEffectiveCapability("0000000000003000", capabilityNetRaw) {
		t.Fatal("declared NET_ADMIN and NET_RAW were not detected")
	}
}

func TestShapingFailureFailsClosed(t *testing.T) {
	t.Parallel()
	runner := func(string, ...string) ([]byte, error) {
		return []byte("operation not permitted"), errors.New("tc failed")
	}
	if _, err := applyAndObserveShaping(runner, "eth0"); err == nil {
		t.Fatal("shaping command failure was accepted")
	} else if !strings.Contains(err.Error(), "operation not permitted") {
		t.Fatalf("shaping error hid tc diagnostics: %v", err)
	}
}

func TestShapingObservationFailureRetainsDiagnostics(t *testing.T) {
	t.Parallel()
	calls := 0
	runner := func(string, ...string) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, nil
		}
		return []byte("device disappeared"), errors.New("tc show failed")
	}
	if _, err := applyAndObserveShaping(runner, "eth0"); err == nil {
		t.Fatal("qdisc observation failure was accepted")
	} else if !strings.Contains(err.Error(), "device disappeared") {
		t.Fatalf("qdisc observation error hid tc diagnostics: %v", err)
	}
}

func TestFixedQdiscRequiresDelayRateAndLimit(t *testing.T) {
	t.Parallel()
	complete := "qdisc netem 8001: root refcnt 2 limit 1000 delay 40ms rate 100Mbit"
	if !fixedQdiscState(complete) {
		t.Fatal("complete fixed qdisc was rejected")
	}
	for _, incomplete := range []string{
		"qdisc netem 8001: root delay 40ms rate 100Mbit",
		"qdisc netem 8001: root limit 1000 delay 40ms",
		"qdisc netem 8001: root limit 1000 rate 100Mbit",
	} {
		if fixedQdiscState(incomplete) {
			t.Fatalf("incomplete fixed qdisc was accepted: %s", incomplete)
		}
	}
}

func TestToolingPeerAndContainerSetsMustBeExact(t *testing.T) {
	t.Parallel()
	roles := map[string]toolingRoleResult{
		"tracer-alpha":  {ObservedPeer: "beta"},
		"tracer-beta":   {ObservedPeer: "alpha"},
		"shape-alpha":   {},
		"shape-beta":    {},
		"capture-alpha": {},
	}
	if !toolingPeerSetMatches(roles) {
		t.Fatal("exact peer set was rejected")
	}
	wrong := make(map[string]toolingRoleResult, len(roles))
	for name, result := range roles {
		wrong[name] = result
	}
	result := wrong["tracer-alpha"]
	result.ObservedPeer = "extra"
	wrong["tracer-alpha"] = result
	if toolingPeerSetMatches(wrong) {
		t.Fatal("unexpected observed peer was accepted")
	}
	expected := map[string]string{"alpha": "id-alpha", "beta": "id-beta"}
	if !exactContainerSet([]string{"id-beta", "id-alpha"}, expected) {
		t.Fatal("exact container set was rejected")
	}
	if exactContainerSet([]string{"id-alpha", "id-extra"}, expected) || exactContainerSet([]string{"id-alpha", "id-beta", "id-extra"}, expected) {
		t.Fatal("unexpected container set was accepted")
	}
}

func TestToolingNetworkRequiresExactTracerAttachments(t *testing.T) {
	t.Parallel()
	network := composeNetworkInspect{
		Name: "project_tooling-link", Internal: true,
		Containers: map[string]json.RawMessage{"id-alpha": {}, "id-beta": {}},
	}
	peers := map[string]string{"tracer-alpha": "id-alpha", "tracer-beta": "id-beta"}
	if !toolingNetworkContract([]composeNetworkInspect{network}, network.Name, peers) {
		t.Fatal("exact tracer attachments were rejected")
	}
	network.Containers["id-extra"] = json.RawMessage{}
	if toolingNetworkContract([]composeNetworkInspect{network}, network.Name, peers) {
		t.Fatal("extra network attachment was accepted")
	}
}

func TestToolingImageReceiptParsingAndSourceBinding(t *testing.T) {
	t.Parallel()
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	digestC := strings.Repeat("c", 64)
	output := digestA + "  /usr/share/ardents/carrier-lab-tools.lock\n" +
		digestB + "  /usr/local/bin/carrier-lab\n" + digestC + "\n"
	lock, binary, source, err := parseToolingImageIdentities(output)
	if err != nil || lock != digestA || binary != digestB || source != digestC {
		t.Fatalf("valid image receipt rejected: %q %q %q %v", lock, binary, source, err)
	}
	if _, _, _, err := parseToolingImageIdentities(digestA); err == nil {
		t.Fatal("incomplete image receipt was accepted")
	}
}

func TestApplicationImageReceiptParsingAndSourceBinding(t *testing.T) {
	t.Parallel()
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	binary, source, err := parseApplicationImageIdentities(digestA + "  /usr/local/bin/carrier-lab\n" + digestB + "\n")
	if err != nil || binary != digestA || source != digestB {
		t.Fatalf("valid application receipt rejected: %q %q %v", binary, source, err)
	}
	if _, _, err := parseApplicationImageIdentities(digestA); err == nil {
		t.Fatal("incomplete application image receipt was accepted")
	}
}

func TestCaptureStartupFailureFailsClosed(t *testing.T) {
	t.Parallel()
	if err := validateCaptureStartup(false, errors.New("tcpdump exited")); err == nil {
		t.Fatal("capture startup failure was accepted")
	}
}

func TestRetainedFailureRedactsHostPathsAndAddresses(t *testing.T) {
	t.Parallel()
	layout := runLayout{repositoryRoot: `C:\source\ardents`, runDir: `C:\temp\run`, evidenceDir: `C:\temp\evidence`}
	message := `mount C:\temp\run from C:\source\ardents failed via 192.0.2.7; see C:/temp/evidence`
	redacted := sanitizeToolingFailure(layout, message)
	for _, forbidden := range []string{`C:\temp\run`, `C:\source\ardents`, "192.0.2.7", "C:/temp/evidence"} {
		if strings.Contains(redacted, forbidden) {
			t.Fatalf("retained failure leaked %q: %s", forbidden, redacted)
		}
	}
}

func TestCaptureEvidenceRejectsEmptyOrMissingTracer(t *testing.T) {
	t.Parallel()
	marker := "carrier-lab-tooling-tracer/run-1"
	if err := validateCaptureEvidence(24, marker, marker); err == nil {
		t.Fatal("header-only capture was accepted")
	}
	if err := validateCaptureEvidence(128, "other traffic", marker); err == nil {
		t.Fatal("capture without the synthetic tracer was accepted")
	}
	if err := validateCaptureEvidence(128, "payload "+marker, marker); err != nil {
		t.Fatalf("valid capture evidence rejected: %v", err)
	}
}

func TestCapturePathMustBeInsideOwnedRunAndOutsideRepository(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runDir := filepath.Join(root, "run")
	repository := filepath.Join(root, "repository")
	for _, directory := range []string{runDir, repository} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateRawCapturePath(filepath.Join(runDir, "capture", "link.pcap"), runDir, repository); err != nil {
		t.Fatalf("owned capture rejected: %v", err)
	}
	if err := validateRawCapturePath(filepath.Join(root, "link.pcap"), runDir, repository); err == nil {
		t.Fatal("capture outside owned run was accepted")
	}
	if err := validateRawCapturePath(filepath.Join(repository, "link.pcap"), runDir, repository); err == nil {
		t.Fatal("capture inside repository was accepted")
	}
}

func TestRawCaptureCleanupIsIdempotentAfterSuccessOrPartialFailure(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"successful", "partial"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			capture := filepath.Join(root, "link.pcap")
			if name == "successful" {
				if err := os.WriteFile(capture, []byte("pcap"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := removeRawCapture(capture, root); err != nil {
				t.Fatal(err)
			}
			if err := removeRawCapture(capture, root); err != nil {
				t.Fatalf("repeated cleanup: %v", err)
			}
			if _, err := os.Stat(capture); !os.IsNotExist(err) {
				t.Fatalf("raw capture remains: %v", err)
			}
		})
	}
}

func TestToolingCleanupRemovesWorkspaceAfterPartialPreparationFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repository := filepath.Join(root, "repository")
	session := filepath.Join(root, "ardents-experiment-session.partial-failure")
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(session, 0o700); err != nil {
		t.Fatal(err)
	}
	identity, err := preflight.NewRunLayout(session, repository, root, "partial-failure")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := ownedLayout(identity, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareSmokeWorkspace(layout); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(layout.runDir, "control"), 0o700); err != nil {
		t.Fatal(err)
	}
	summary := toolingSmokeSummary{
		SchemaVersion: toolingSmokeSchema,
		RunID:         layout.runID,
		Status:        "failed",
		Checks:        map[string]bool{},
	}
	for _, check := range requiredToolingChecks {
		summary.Checks[check] = false
	}
	capturePath := filepath.Join(layout.runDir, "raw-capture", "alpha-link.pcap")
	noProjectResources := func(context.Context, string) bool { return false }
	if err := finishToolingSmokeWithCheck(layout, composeProjectName("tooling-"+layout.runID), nil, capturePath, &summary, errors.New("partial preparation"), noProjectResources); err == nil {
		t.Fatal("partial preparation failure was accepted")
	}
	if _, err := os.Stat(layout.runDir); !os.IsNotExist(err) {
		t.Fatalf("partial workspace remains: %v", err)
	}
	manifestData, err := os.ReadFile(filepath.Join(layout.evidenceDir, toolingManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var retained toolingSmokeSummary
	if err := json.Unmarshal(manifestData, &retained); err != nil {
		t.Fatal(err)
	}
	if retained.Status != "failed" || !retained.Checks["cleanup_complete"] || !retained.Checks["raw_capture_removed"] {
		t.Fatalf("partial cleanup evidence = %#v", retained)
	}
}

func TestToolingVerdictBindsImmutableManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repository := filepath.Join(root, "repository")
	session := filepath.Join(root, "ardents-experiment-session.manifest-binding")
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(session, 0o700); err != nil {
		t.Fatal(err)
	}
	identity, err := preflight.NewRunLayout(session, repository, root, "manifest-binding")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := ownedLayout(identity, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareSmokeWorkspace(layout); err != nil {
		t.Fatal(err)
	}
	summary := toolingSmokeSummary{SchemaVersion: toolingSmokeSchema, RunID: layout.runID, Status: "passed", Checks: map[string]bool{}}
	if err := writeToolingEvidence(layout, &summary); err != nil {
		t.Fatal(err)
	}
	verdictData, err := os.ReadFile(filepath.Join(layout.evidenceDir, toolingVerdictFile))
	if err != nil {
		t.Fatal(err)
	}
	var verdict toolingSmokeVerdict
	if err := json.Unmarshal(verdictData, &verdict); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(layout.evidenceDir, toolingManifestFile)
	if err := verifyToolingManifest(manifestPath, verdict.ManifestSHA256); err != nil {
		t.Fatalf("fresh manifest did not match verdict: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyToolingManifest(manifestPath, verdict.ManifestSHA256); err == nil {
		t.Fatal("mutated tooling manifest still matched its verdict")
	}
}

func TestToolingVerdictCannotPassBeforeShapingCaptureAndCleanup(t *testing.T) {
	t.Parallel()
	checks := map[string]bool{
		"tool_identity": true, "shaping_alpha": true, "shaping_beta": true,
		"capture_started": true, "capture_nonempty": true, "capture_tracer": true,
		"peer_set": true, "image_receipt": true, "isolation": true,
		"cleanup_complete": true, "raw_capture_removed": true,
	}
	if !toolingChecksPassed(checks) {
		t.Fatal("complete tooling checks did not pass")
	}
	for name := range checks {
		copy := make(map[string]bool, len(checks))
		for key, value := range checks {
			copy[key] = value
		}
		copy[name] = false
		if toolingChecksPassed(copy) {
			t.Fatalf("verdict passed with %s=false", name)
		}
	}
}
