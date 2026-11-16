//go:build linux || darwin

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestSourceReleaseAuthenticatesSignedRetainedCandidate(t *testing.T) {
	root := sourceReleaseTestRoot(t)
	dir, _, _ := infrastructureDistributionRoot(t, root, []byte("source program fixture"), nil, 1, nil)
	candidate, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	history := filepath.Join(root, "history")
	verifier, err := release.Open(history)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := verifier.Evaluate(t.Context(), infrastructureCandidateInput(t, candidate)).Authorization(); !ok {
		t.Fatal("signed fixture did not establish Release floors")
	}
	if err := verifier.Close(); err != nil {
		t.Fatal(err)
	}
	request, invalid := parseSourceRelease([]string{"authenticate-candidate", dir, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339)})
	if invalid != "" {
		t.Fatal(invalid)
	}
	authorization, owner, refused, err := openSourceRelease(t.Context(), request, true)
	if err != nil || refused != "" || owner == nil {
		t.Fatalf("signed retained candidate refused: %s %v", refused, err)
	}
	if err := authorization.Check(t.Context(), owner); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ardents-source.service"), []byte("substituted"), 0600); err != nil {
		t.Fatal(err)
	}
	_, owner, refused, err = openSourceRelease(t.Context(), request, true)
	if owner != nil {
		_ = owner.Close()
	}
	if err == nil && refused == "" {
		t.Fatal("substituted artifact accepted")
	}
}

func TestDetachedInfrastructureInspectionCannotAdvanceRetainedFloors(t *testing.T) {
	for _, role := range []string{"source", "forwarding"} {
		t.Run(role, func(t *testing.T) {
			root := sourceReleaseTestRoot(t)
			oldParent, nextParent := filepath.Join(root, "old"), filepath.Join(root, "next")
			for _, parent := range []string{oldParent, nextParent} {
				if err := os.Mkdir(parent, 0700); err != nil {
					t.Fatal(err)
				}
			}
			authority := newConsumerReleaseAuthority(t, infrastructureReference.Add(365*24*time.Hour))
			old, _, _ := infrastructureDistributionInvocation(t, oldParent, []byte("old program"), authority, 1, nil, role)
			next, _, _ := infrastructureDistributionInvocation(t, nextParent, []byte("new program"), authority, 2, nil, role)
			candidate, err := enrollment.ReadInfrastructureCandidate(t.Context(), old)
			if err != nil {
				t.Fatal(err)
			}
			history := filepath.Join(root, "history")
			v, err := release.Open(history)
			if err != nil {
				t.Fatal(err)
			}
			oldInput := infrastructureCandidateInput(t, candidate)
			if err := initialInspectionMetadata(t.Context(), v, oldInput); err != nil {
				t.Fatalf("empty initial inspection history refused: %v", err)
			}
			if _, ok := v.Evaluate(t.Context(), oldInput).Authorization(); !ok {
				t.Fatal("initial signed test floors refused")
			}
			if err := sameReleaseMetadata(t.Context(), v, oldInput); err != nil {
				t.Fatalf("exact retained metadata refused: %v", err)
			}
			if err := initialInspectionMetadata(t.Context(), v, oldInput); err != nil {
				t.Fatalf("unchanged initial inspection refused: %v", err)
			}
			nextCandidate, err := enrollment.ReadInfrastructureCandidate(t.Context(), next)
			if err != nil {
				t.Fatal(err)
			}
			if err := initialInspectionMetadata(t.Context(), v, infrastructureCandidateInput(t, nextCandidate)); err == nil {
				t.Fatal("detached initial inspection accepted newer metadata")
			}
			missing := oldInput
			missing.Files = map[string][]byte{}
			if err := sameReleaseMetadata(t.Context(), v, missing); err == nil {
				t.Fatal("incomplete retained metadata accepted")
			}
			extra := oldInput
			extra.Files = make(map[string][]byte, len(oldInput.Files)+1)
			for name, raw := range oldInput.Files {
				extra.Files[name] = raw
			}
			nextRoot, err := metadata.Root().FromBytes(oldInput.RootBytes)
			if err != nil {
				t.Fatal(err)
			}
			nextRoot.Signed.Version = 2
			nextRoot.Signatures = nil
			for _, signer := range authority.signers[:3] {
				if _, err := nextRoot.Sign(signer); err != nil {
					t.Fatal(err)
				}
			}
			nextRootBytes, err := nextRoot.ToBytes(false)
			if err != nil {
				t.Fatal(err)
			}
			extra.Files["https://release.invalid/metadata/2.root.json"] = nextRootBytes
			if err := sameReleaseMetadata(t.Context(), v, extra); err == nil {
				t.Fatal("signed unretained root rotation input accepted")
			}
			control, err := release.Open(filepath.Join(root, "control-history"))
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := control.Evaluate(t.Context(), oldInput).Authorization(); !ok {
				t.Fatal("control initial floors refused")
			}
			control.Evaluate(t.Context(), extra)
			floors, err := control.CurrentFloors(t.Context())
			if err != nil || floors.RootVersion != 2 {
				t.Fatalf("signed root did not advance control history: %v %v", floors.RootVersion, err)
			}
			if err := control.Close(); err != nil {
				t.Fatal(err)
			}
			if err := v.Close(); err != nil {
				t.Fatal(err)
			}
			retained, err := release.OpenRetained(history)
			if err != nil {
				t.Fatal(err)
			}
			if err := sameReleaseMetadata(t.Context(), retained, infrastructureCandidateInput(t, nextCandidate)); err == nil {
				t.Fatal("detached inspection accepted a newer release")
			}
			if err := sameReleaseMetadata(t.Context(), retained, oldInput); err != nil {
				t.Fatal("previous retained metadata became unavailable", err)
			}
			if err := retained.Close(); err != nil {
				t.Fatal(err)
			}
			if role == "source" {
				request, invalid := parseSourceRelease([]string{"authenticate-candidate", next, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339)})
				if invalid != "" {
					t.Fatal("valid transition inputs refused", invalid)
				}
				a, owner, refused, err := openSourceRelease(t.Context(), request, false)
				if err != nil || refused != "" || owner == nil {
					t.Fatalf("retained verifier could not authenticate successor: %s %v", refused, err)
				}
				if err := a.Check(t.Context(), owner); err != nil {
					t.Fatal("successor lost its original authorization", err)
				}
				if err := owner.Close(); err != nil {
					t.Fatal(err)
				}
				oldRequest, invalid := parseSourceRelease([]string{"authenticate-candidate", old, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339)})
				if invalid != "" {
					t.Fatal(invalid)
				}
				_, oldOwner, refused, oldErr := openSourceRelease(t.Context(), oldRequest, true)
				if oldOwner != nil {
					_ = oldOwner.Close()
				}
				if oldErr == nil && refused == "" {
					t.Fatal("old release accepted after successor floor advancement")
				}
			}
		})
	}
}

func TestInfrastructureRoleCandidatesKeepSeparateSignedAuthority(t *testing.T) {
	for _, role := range []string{"forwarding", "join", "descriptor", "introduction"} {
		t.Run(role, func(t *testing.T) {
			root := sourceReleaseTestRoot(t)
			dir, _, _ := infrastructureDistributionInvocation(t, root, []byte("public program fixture"), nil, 1, nil, role)
			candidate, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
			if err != nil {
				t.Fatal(err)
			}
			input, err := releaseartifact.ProgramInputs(candidate)
			if err != nil {
				t.Fatal(err)
			}
			input.Local = release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "h3-test", Network: "new-release-test", RefTime: infrastructureReference}
			verifier, err := release.Open(filepath.Join(root, "history"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := verifier.Close(); err != nil {
					t.Error(err)
				}
			}()
			if _, ok := verifier.Evaluate(t.Context(), input).Authorization(); !ok {
				t.Fatal("signed program did not establish Release floors")
			}
			switch role {
			case "forwarding":
				a, e := releaseartifact.AuthenticateForwardingCandidate(t.Context(), verifier, candidate, input)
				err = e
				if err == nil {
					err = a.Check(t.Context(), verifier)
				}
			case "join":
				a, e := releaseartifact.AuthenticateJoinCandidate(t.Context(), verifier, candidate, input)
				err = e
				if err == nil {
					err = a.Check(t.Context(), verifier)
				}
			case "descriptor":
				a, e := releaseartifact.AuthenticateDescriptorCandidate(t.Context(), verifier, candidate, input)
				err = e
				if err == nil {
					err = a.Check(t.Context(), verifier)
				}
			case "introduction":
				a, e := releaseartifact.AuthenticateIntroductionCandidate(t.Context(), verifier, candidate, input)
				err = e
				if err == nil {
					err = a.Check(t.Context(), verifier)
				}
			}
			if err != nil {
				t.Fatalf("signed %s candidate refused: %v", role, err)
			}
			if _, err := releaseartifact.AuthenticateCandidate(t.Context(), verifier, candidate, input); err == nil {
				t.Fatal("Source accepted a different role")
			}

		})
	}
}

func TestSourceReleaseReceiptPreservesPublicResult(t *testing.T) {
	var out bytes.Buffer
	// This exercises only the detached report format, not Release authentication.
	err := writeInfrastructureReleaseReceipt(context.Background(), &out, releaseartifact.Authorization{}, release.Decision{Outcome: release.OutcomeReleaseAccepted}, release.Decision{Outcome: release.OutcomeNoUpdate})
	if err != nil {
		t.Fatal(err)
	}
	const emptyDigest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	want := `{"outcome":"authenticated-source-artifact","program_outcome":"release-accepted","generation_outcome":"no-update","program_sha256":"` + emptyDigest + `","descriptor_sha256":"` + emptyDigest + `","template_sha256":"` + emptyDigest + `"}` + "\n"
	if out.String() != want {
		t.Fatalf("unexpected Source release receipt: %s", out.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out.Reset()
	if writeInfrastructureReleaseReceipt(ctx, &out, releaseartifact.Authorization{}, release.Decision{}, release.Decision{}) == nil || out.Len() != 0 {
		t.Fatal("canceled receipt was published")
	}
}

func TestSourceReleaseRefusesMissingInventoryBeforeHistory(t *testing.T) {
	for _, initial := range []bool{true, false} {
		t.Run(map[bool]string{true: "initial", false: "candidate"}[initial], func(t *testing.T) {
			root := sourceReleaseTestRoot(t)
			bundle := filepath.Join(root, "absent-bundle")
			history := filepath.Join(root, "absent-history")
			args := []string{"authenticate-candidate", bundle, history, "closed", "network", "2030-01-02T03:04:05Z"}
			if initial {
				args = []string{"authenticate-initial", bundle, strings.Repeat("0", 64), history, "closed", "network", "2030-01-02T03:04:05Z"}
			}
			request, invalid := parseSourceRelease(args)
			if invalid != "" {
				t.Fatal("valid declarations refused", invalid)
			}
			if request.initial != initial || request.bundle != bundle || request.history != history || request.environment != "closed" || request.network != "network" || request.reference.Format(time.RFC3339) != "2030-01-02T03:04:05Z" {
				t.Fatal("Source release fields changed", request)
			}
			if initial && request.pin != strings.Repeat("0", 64) || !initial && request.pin != "" {
				t.Fatal("initial pin crossed the candidate boundary")
			}
			a, v, refused, err := openSourceRelease(t.Context(), request, false)
			if err == nil || refused != "infrastructure-inventory-refused" || v != nil || a.HasInventory() {
				t.Fatal("missing inventory was accepted", refused, err)
			}
			if _, err := os.Lstat(history); !os.IsNotExist(err) {
				t.Fatal("refusal created Release history", err)
			}
		})
	}
}

func TestSourceReleaseInputRefusesOverlappingAndIncompletePaths(t *testing.T) {
	root := sourceReleaseTestRoot(t)
	args := []string{"authenticate-candidate", filepath.Join(root, "bundle"), filepath.Join(root, "bundle", "history"), "closed", "network", "2030-01-02T03:04:05Z"}
	if _, outcome := parseSourceRelease(args); outcome != "infrastructure-paths-refused" {
		t.Fatal("overlapping history accepted", outcome)
	}
	for _, invalid := range [][]string{nil, args[:len(args)-1], {"unknown", "a", "b", "c", "d", "2030-01-02T03:04:05Z"}} {
		if _, outcome := parseSourceRelease(invalid); outcome != "invalid-input" {
			t.Fatal("invalid source arguments accepted", outcome)
		}
	}
}

func sourceReleaseTestRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
