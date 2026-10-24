//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// An outside diagnostic of genuine Release decisions, never an Installation
// permission or a replacement for canonical recovery. No reference time is
// supplied: each observation captures the actual wall clock once for both
// targets. The completed installed cohort still needs its exact direct D6.
type sourceReleaseExpiryProfile struct {
	Schema              string            `json:"schema"`
	PublicReceiptSHA256 string            `json:"public_receipt_sha256"`
	Distribution        string            `json:"distribution"`
	History             string            `json:"history"`
	Network             string            `json:"network"`
	Phase               string            `json:"phase"`
	Files               map[string]string `json:"files"`
}

type sourceReleaseExpiryObservation struct {
	ProfileSHA256       string               `json:"profile_sha256"`
	PublicReceiptSHA256 string               `json:"public_receipt_sha256"`
	Phase               string               `json:"phase"`
	ReferenceTime       time.Time            `json:"reference_time"`
	TimestampExpires    time.Time            `json:"timestamp_expires"`
	OtherBounds         map[string]time.Time `json:"other_bounds"`
	Files               map[string]string    `json:"files"`
	Outcomes            []release.Outcome    `json:"outcomes"`
	Authorized          []bool               `json:"authorized"`
	FloorsUnchanged     bool                 `json:"floors_unchanged"`
	VerifierClosed      bool                 `json:"verifier_closed"`
	InstalledExecution  bool                 `json:"installed_execution"`
}

type sourceReleaseExpiryTarget struct {
	NoNew         time.Time `json:"build_safety_no_new_work_after"`
	Terminate     time.Time `json:"build_safety_terminate_after"`
	ProtocolPhase string    `json:"protocol_phase"`
	Overlapped    time.Time `json:"protocol_overlapped_since"`
	Emergency     string    `json:"emergency_reason"`
	EmergencyEnd  time.Time `json:"emergency_expiry"`
}

// This finite announced-source diagnostic does not generalize the product's
// other signed protocol phases. They keep their own Release policy; a second
// policy clock cannot be counted as Timestamp-only expiry evidence here.
func (c sourceReleaseExpiryTarget) noCompetingProtocolExpiry() bool {
	return c.ProtocolPhase == "announced" && c.Overlapped.IsZero() && c.Emergency == "" && c.EmergencyEnd.IsZero()
}

func sourceReleaseExpiryNames() []string {
	return []string{ProgramName, DescriptorName, TemplateName, "root.json", "timestamp.json", "snapshot.json", "targets.json", "infrastructure-enrollment.json"}
}

func readSourceReleaseExpiryProfile(path, pin string) (*sourceReleaseExpiryProfile, error) {
	if !sourcePath(path) || !sourceCutObserverSHA(pin) {
		return nil, errors.New("exact expiry observation profile required")
	}
	raw, _, err := readSourceCutObserverFile(path, 64<<10)
	if err != nil || digest(raw) != pin {
		return nil, errors.Join(errors.New("original expiry profile pin differs"), err)
	}
	p := new(sourceReleaseExpiryProfile)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(p); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || decoder.Decode(new(any)) != io.EOF ||
		p.Schema != "source-release-expiry-observer-v1" || !sourceCutObserverSHA(p.PublicReceiptSHA256) ||
		!sourcePath(p.Distribution) || !sourcePath(p.History) || contains(p.Distribution, p.History) || contains(p.History, p.Distribution) ||
		!sourceCutObserverNetwork(p.Network) || p.Phase != "before" && p.Phase != "expired" || len(p.Files) != 8 {
		return nil, errors.New("distinct closed expiry observation purpose required")
	}
	for _, name := range sourceReleaseExpiryNames() {
		if !sourceCutObserverSHA(p.Files[name]) {
			return nil, errors.New("complete exact expiry artifact inventory required")
		}
	}
	return p, nil
}

func observeSourceReleaseExpiry(ctx context.Context, p *sourceReleaseExpiryProfile) (result sourceReleaseExpiryObservation, err error) {
	if ctx == nil || p == nil {
		return result, ErrInput
	}
	candidate, err := enrollment.ReadInfrastructureCandidate(ctx, p.Distribution)
	if err != nil {
		return result, err
	}
	read := func(name string) []byte { raw, _ := candidate.File(name); return raw }
	for _, name := range sourceReleaseExpiryNames() {
		if digest(read(name)) != p.Files[name] {
			return result, errors.New("expiry artifact differs from exact original pin")
		}
	}
	declaration, err := ValidateResources(read(DescriptorName), read(ProgramName), read(TemplateName))
	if err != nil {
		return result, errors.Join(ErrBinding, err)
	}
	result.Phase, result.PublicReceiptSHA256, result.Files = p.Phase, p.PublicReceiptSHA256, p.Files
	result.ReferenceTime, result.OtherBounds = time.Now().UTC(), map[string]time.Time{}
	for _, name := range []string{"root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		var envelope struct {
			Signed struct {
				Expires time.Time `json:"expires"`
			} `json:"signed"`
		}
		if json.Unmarshal(read(name), &envelope) != nil || envelope.Signed.Expires.IsZero() {
			return result, errors.New("original metadata expiry observation missing")
		}
		if name == "timestamp.json" {
			result.TimestampExpires = envelope.Signed.Expires
		} else {
			result.OtherBounds[name] = envelope.Signed.Expires
		}
	}
	var targets struct {
		Signed struct {
			Targets map[string]struct {
				Custom sourceReleaseExpiryTarget `json:"custom"`
			} `json:"targets"`
		} `json:"signed"`
	}
	if json.Unmarshal(read("targets.json"), &targets) != nil {
		return result, ErrBinding
	}
	for _, target := range []string{ProgramTarget, GenerationTarget} {
		custom, ok := targets.Signed.Targets[target]
		if !ok || custom.Custom.NoNew.IsZero() || custom.Custom.Terminate.IsZero() {
			return result, errors.New("original target safety bounds missing")
		}
		if !custom.Custom.noCompetingProtocolExpiry() {
			return result, errors.New("announced non-emergency protocol required for Timestamp-only observation")
		}
		result.OtherBounds[target+"/no-new"] = custom.Custom.NoNew
		result.OtherBounds[target+"/terminate"] = custom.Custom.Terminate
	}
	for _, expiry := range result.OtherBounds {
		if !expiry.After(result.ReferenceTime) || !expiry.After(result.TimestampExpires) {
			return result, errors.New("another unchanged authority bound overlaps Timestamp expiry")
		}
	}
	if (p.Phase == "before") != result.ReferenceTime.Before(result.TimestampExpires) {
		return result, errors.New("actual wall time does not match selected expiry phase")
	}
	files, err := release.StaticMetadataFiles(read("root.json"), read("timestamp.json"), read("snapshot.json"), read("targets.json"))
	if err != nil {
		return result, err
	}
	verifier, err := release.OpenRetained(p.History)
	if err != nil {
		return result, err
	}
	defer func() {
		closed := verifier.Close()
		err = errors.Join(err, closed)
		result.VerifierClosed = closed == nil
	}()
	before, err := verifier.CurrentFloors(ctx)
	if err != nil || !before.Complete() {
		return result, errors.Join(release.ErrTrustUnavailable, err)
	}
	input := release.Inputs{RootBytes: read("root.json"), Files: files,
		Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "alpha", Network: p.Network, RefTime: result.ReferenceTime}}
	var decisions []release.Decision
	for i, target := range []string{ProgramTarget, GenerationTarget} {
		input.TargetPath, input.Artifact = target, read([]string{ProgramName, DescriptorName}[i])
		decision := verifier.Evaluate(ctx, input)
		_, authorized := decision.Authorization()
		result.Outcomes, result.Authorized = append(result.Outcomes, decision.Outcome), append(result.Authorized, authorized)
		if p.Phase == "before" {
			if !authorized || decision.Outcome != release.OutcomeNoUpdate {
				return result, errors.Join(errors.New("same already admitted target not authorized before expiry"), decision.Err())
			}
		} else if authorized || decision.Outcome != release.OutcomeReleaseExpired {
			return result, errors.Join(errors.New("Timestamp alone did not produce actual Release expiry"), decision.Err())
		}
		decisions = append(decisions, decision)
	}
	if p.Phase == "before" && !coherent(decisions[0], decisions[1], declaration) {
		return result, errors.New("actual pre-expiry target pair is incoherent")
	}
	after, err := verifier.CurrentFloors(ctx)
	a, encodeA := json.Marshal(before)
	b, encodeB := json.Marshal(after)
	if err != nil || encodeA != nil || encodeB != nil || !bytes.Equal(a, b) {
		return result, errors.Join(errors.New("expiry observation changed original floors"), err, encodeA, encodeB)
	}
	result.FloorsUnchanged = true
	rechecked, err := enrollment.ReadInfrastructureCandidate(ctx, p.Distribution)
	if err != nil {
		return result, err
	}
	for _, name := range sourceReleaseExpiryNames() {
		raw, _ := rechecked.File(name)
		if digest(raw) != p.Files[name] {
			return result, errors.New("original expiry artifacts changed through observation")
		}
	}
	return result, errors.Join(ctx.Err(), candidate.Check(), rechecked.Check())
}

func TestSourceReleaseExpiryObserverNative(t *testing.T) {
	mode := os.Getenv("ARDENTS_SOURCE_RELEASE_EXPIRY_OBSERVER_MODE")
	if mode == "" {
		return // Only an exact outside finite caller selects this observer.
	}
	if os.Geteuid() != 0 || mode != "native-release-expiry" && mode != "installed-release-expiry" {
		t.Fatal("closed native expiry observer mode required")
	}
	if mode == "installed-release-expiry" {
		marker, err := os.ReadFile("/etc/ardents-source-startup-native-stand")
		container, ce := os.ReadFile("/run/systemd/container")
		cgroup, ge := os.ReadFile("/proc/self/cgroup")
		if err != nil || ce != nil || ge != nil || string(marker) != "ardents-source-startup-native-v1\n" || strings.TrimSpace(string(container)) != "ardents-source-startup" || string(cgroup) != "0::/../supervisor\n" {
			t.Fatal("original private accepted installed expiry cohort required", err, ce, ge)
		}
	} else {
		marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
		if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" {
			t.Fatal("separate native preparation stand required", err)
		}
	}
	path, pin := os.Getenv("ARDENTS_SOURCE_RELEASE_EXPIRY_PROFILE"), os.Getenv("ARDENTS_SOURCE_RELEASE_EXPIRY_PROFILE_SHA256")
	profile, err := readSourceReleaseExpiryProfile(path, pin)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(profile.Distribution) != profile.Distribution || filepath.Clean(profile.History) != profile.History {
		t.Fatal("canonical original expiry paths required")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	result, err := observeSourceReleaseExpiry(ctx, profile)
	result.ProfileSHA256, result.InstalledExecution = pin, mode == "installed-release-expiry"
	if err != nil || !result.VerifierClosed || !result.FloorsUnchanged {
		t.Fatal("actual original Release expiry observation failed", err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("SOURCE_RELEASE_EXPIRY_OBSERVATION %s\n", raw)
}

func TestSourceReleaseExpiryProfileNative(t *testing.T) {
	zero := strings.Repeat("0", 64)
	p := sourceReleaseExpiryProfile{Schema: "source-release-expiry-observer-v1", PublicReceiptSHA256: zero,
		Distribution: "/testtmp/distribution", History: "/testtmp/history", Network: "one-network", Phase: "before", Files: map[string]string{}}
	for _, name := range sourceReleaseExpiryNames() {
		p.Files[name] = zero
	}
	encode := func(p sourceReleaseExpiryProfile) []byte {
		t.Helper()
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		return append(raw, '\n')
	}
	load := func(raw []byte) error {
		path := filepath.Join(t.TempDir(), "expiry-profile.json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		_, err := readSourceReleaseExpiryProfile(path, digest(raw))
		return err
	}
	for _, phase := range []string{"before", "expired"} {
		p.Phase = phase
		if err := load(encode(p)); err != nil {
			t.Fatal("closed finite expiry purpose", err)
		}
	}
	raw := encode(p)
	for name, body := range map[string][]byte{
		"other-purpose":           bytes.Replace(raw, []byte("source-release-expiry-observer-v1"), []byte("source-recovery-refusal-observer-v1"), 1),
		"supplied-reference-time": bytes.Replace(raw, []byte(`"phase":`), []byte(`"reference_time":"2030-01-01T00:00:00Z","phase":`), 1),
		"unknown":                 bytes.Replace(raw, []byte(`"phase":`), []byte(`"unknown":true,"phase":`), 1),
		"duplicate":               bytes.Replace(raw, []byte(`"phase":`), []byte(`"phase":"before","phase":`), 1),
		"trailing":                append(bytes.Clone(raw), []byte("{}\n")...),
	} {
		if err := load(body); err == nil {
			t.Fatal("mixed or supplied clock accepted", name)
		}
	}
	for name, change := range map[string]func(*sourceReleaseExpiryProfile){
		"relative-path":  func(p *sourceReleaseExpiryProfile) { p.Distribution = "relative" },
		"shared-history": func(p *sourceReleaseExpiryProfile) { p.History = p.Distribution },
		"nested-history": func(p *sourceReleaseExpiryProfile) { p.History = p.Distribution + "/history" },
		"unknown-phase":  func(p *sourceReleaseExpiryProfile) { p.Phase = "renew" },
		"missing-pin":    func(p *sourceReleaseExpiryProfile) { delete(p.Files, "timestamp.json") },
		"foreign-file":   func(p *sourceReleaseExpiryProfile) { delete(p.Files, "timestamp.json"); p.Files["foreign.json"] = zero },
		"wrong-pin":      func(p *sourceReleaseExpiryProfile) { p.Files["timestamp.json"] = strings.Repeat("F", 64) },
	} {
		changed := p
		changed.Files = map[string]string{}
		for name, pin := range p.Files {
			changed.Files[name] = pin
		}
		change(&changed)
		if err := load(encode(changed)); err == nil {
			t.Fatal("ambiguous expiry inventory accepted", name)
		}
	}
	// Decode the signed custom facts as the actual observation does. Neither a
	// simultaneous emergency expiry nor another protocol phase may disappear
	// behind the earlier Release metadata-expired outcome.
	for _, raw := range []string{
		`{"protocol_phase":"required","emergency_reason":"compromised-key","emergency_expiry":"2020-01-01T00:00:00Z"}`,
		`{"protocol_phase":"announced","emergency_reason":"compromised-key"}`,
		`{"protocol_phase":"announced","emergency_expiry":"2020-01-01T00:00:00Z"}`,
		`{"protocol_phase":"announced","protocol_overlapped_since":"2020-01-01T00:00:00Z"}`,
		`{"protocol_phase":"preferred"}`, `{}`,
	} {
		var target sourceReleaseExpiryTarget
		if err := json.Unmarshal([]byte(raw), &target); err != nil || target.noCompetingProtocolExpiry() {
			t.Fatal("competing signed protocol clock became Timestamp-only evidence", raw, err)
		}
	}
	var announced sourceReleaseExpiryTarget
	if err := json.Unmarshal([]byte(`{"protocol_phase":"announced","emergency_reason":"","emergency_expiry":"0001-01-01T00:00:00Z","protocol_overlapped_since":"0001-01-01T00:00:00Z"}`), &announced); err != nil || !announced.noCompetingProtocolExpiry() {
		t.Fatal("finite announced/no-emergency observation rejected", err)
	}
}
