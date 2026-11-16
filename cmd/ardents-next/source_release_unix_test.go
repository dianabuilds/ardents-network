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
	decision := verifier.Evaluate(t.Context(), infrastructureCandidateInput(t, candidate))
	if _, ok := decision.Authorization(); !ok {
		t.Fatalf("signed fixture did not establish Release floors: %s %v", decision.Outcome, decision.Err())
	}
	if err := verifier.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	args := []string{"authenticate-candidate", dir, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339)}
	if code := runSourceRelease(t.Context(), args, &out); code != 0 || !strings.Contains(out.String(), `"program_outcome":"no-update"`) || !strings.Contains(out.String(), `"generation_outcome":"no-update"`) {
		t.Fatalf("signed retained candidate refused: %d %s", code, out.String())
	}
	if err := os.WriteFile(filepath.Join(dir, "ardents-source.service"), []byte("substituted"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := runSourceRelease(t.Context(), args, &out); code == 0 {
		t.Fatal("substituted artifact accepted")
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
			if err := verifier.Close(); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			args := []string{"authenticate-" + role + "-candidate", dir, filepath.Join(root, "history"), "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339)}
			cold := append([]string(nil), args...)
			cold[2] = filepath.Join(root, "missing-history")
			if code := runRelease(t.Context(), cold, &out); code == 0 {
				t.Fatal("candidate created initial trust")
			}
			if _, err := os.Lstat(cold[2]); !os.IsNotExist(err) {
				t.Fatal("candidate refusal changed missing history", err)
			}
			out.Reset()
			if code := runRelease(t.Context(), args, &out); code != 0 || !strings.Contains(out.String(), "authenticated-"+role+"-artifact") || !strings.Contains(out.String(), `"generation_outcome":"no-update"`) {
				t.Fatalf("role command refused signed candidate: %d %s", code, out.String())
			}
			args[0] = "authenticate-join-candidate"
			if role == "join" {
				args[0] = "authenticate-forwarding-candidate"
			}
			out.Reset()
			if code := runRelease(t.Context(), args, &out); code == 0 || !strings.Contains(out.String(), "infrastructure-artifact-refused") {
				t.Fatalf("role command accepted another role: %d %s", code, out.String())
			}
			args[0] = "authenticate-" + role + "-candidate"
			if err := os.WriteFile(filepath.Join(dir, "ardents-"+role+".service"), []byte("substituted"), 0600); err != nil {
				t.Fatal(err)
			}
			out.Reset()
			if code := runRelease(t.Context(), args, &out); code == 0 {
				t.Fatal("role command accepted substituted artifact")
			}
		})
	}
}

func TestSourceReleaseCandidateRefusesMissingInventoryWithoutHistory(t *testing.T) {
	root := sourceReleaseTestRoot(t)
	history := filepath.Join(root, "history")
	var out bytes.Buffer
	args := []string{"authenticate-candidate", filepath.Join(root, "absent"), history, "closed", "network", "2030-01-02T03:04:05Z"}
	if code := runSourceRelease(t.Context(), args, &out); code != 1 || !strings.Contains(out.String(), "infrastructure-inventory-refused") {
		t.Fatalf("missing candidate was not refused: %d %s", code, out.String())
	}
	if _, err := os.Lstat(history); !os.IsNotExist(err) {
		t.Fatal("candidate refusal touched history", err)
	}
	out.Reset()
	initial := []string{"authenticate-source-initial", filepath.Join(root, "absent"), strings.Repeat("0", 64), history, "closed", "network", "2030-01-02T03:04:05Z"}
	if code := runRelease(t.Context(), initial, &out); code != 2 || !strings.Contains(out.String(), "invalid-input") {
		t.Fatalf("detached initial verification accepted: %d %s", code, out.String())
	}
	if _, err := os.Lstat(history); !os.IsNotExist(err) {
		t.Fatal("detached initial refusal changed history", err)
	}
}

func TestSourceReleaseReceiptPreservesPublicResult(t *testing.T) {
	var out bytes.Buffer
	// This exercises only the detached report format, not Release authentication.
	err := writeInfrastructureReleaseReceipt(context.Background(), &out, "authenticated-source-artifact", releaseartifact.Authorization{}, release.Decision{Outcome: release.OutcomeReleaseAccepted}, release.Decision{Outcome: release.OutcomeNoUpdate})
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
	if writeInfrastructureReleaseReceipt(ctx, &out, "authenticated-source-artifact", releaseartifact.Authorization{}, release.Decision{}, release.Decision{}) == nil || out.Len() != 0 {
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
			a, v, refused, err := openSourceRelease(t.Context(), request)
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
