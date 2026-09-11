//go:build linux

package endpoint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type textRoleDurableStateCapture struct {
	Role           string                       `json:"role"`
	Phase          string                       `json:"phase"`
	InputSHA256    string                       `json:"inputSHA256"`
	SequentialRead bool                         `json:"sequentialRead"`
	Roots          []textRoleDurableRootCapture `json:"roots"`
}

type textRoleDurableRootCapture struct {
	Name  string                       `json:"name"`
	Files []textRoleDurableFileCapture `json:"files"`
}

type textRoleDurableFileCapture struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type textRoleDurableReceipt struct {
	Carrier string                       `json:"carrier"`
	Roles   []textRoleDurableReceiptRole `json:"roles"`
}

type textRoleDurableReceiptRole struct {
	Path        string                        `json:"path"`
	Role        string                        `json:"role"`
	InputSHA256 string                        `json:"inputSHA256"`
	Phases      []textRoleDurableReceiptPhase `json:"phases"`
}

type textRoleDurableReceiptPhase struct {
	Phase          string   `json:"phase"`
	Manifest       string   `json:"manifest"`
	ManifestSHA256 string   `json:"manifestSHA256"`
	Roots          []string `json:"roots"`
}

type textRoleDurableRoot struct {
	name string
	path string
}

func TestCaptureTextRoleDurableStateRejectsRootInsideOutput(t *testing.T) {
	output := t.TempDir()
	inputPath := filepath.Join(output, "input.json")
	if err := os.WriteFile(inputPath, []byte(`{"role":"issuer"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(output, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	err := captureTextRoleDurableState(textRoleProcessInput{Output: output, StateRoot: nested}, inputPath, "startup")
	if err == nil || !strings.Contains(err.Error(), "root overlaps role output") {
		t.Fatalf("capture error = %v, want root overlap", err)
	}
	if _, err := os.Stat(filepath.Join(output, "startup-durable")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("capture output exists after root rejection: %v", err)
	}
}

func captureTextRoleDurableState(input textRoleProcessInput, inputPath, phase string) error {
	if phase != "startup" && phase != "published" && phase != "data" && phase != "withdrawn" && phase != "stopped" {
		return errors.New("invalid durable state phase")
	}
	if filepath.Clean(inputPath) != filepath.Join(input.Output, "input.json") {
		return errors.New("durable state input is outside role output")
	}
	if !filepath.IsAbs(input.Output) || filepath.Clean(input.Output) != input.Output {
		return errors.New("role output is not a clean absolute path")
	}
	roots := textRoleDurableRoots(input)
	for _, root := range roots {
		if err := durableRootWithinRoleOutput(input.Output, root.path); err != nil {
			return fmt.Errorf("%s root: %w", root.name, err)
		}
	}
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}
	inputDigest := sha256.Sum256(inputBytes)
	output := filepath.Join(input.Output, phase+"-durable")
	if err := os.Mkdir(output, 0o700); err != nil {
		return err
	}
	completed := false
	defer func() {
		if !completed {
			_ = writeTextRoleDurableFile(filepath.Join(input.Output, phase+".durable.incomplete"), []byte("durable capture incomplete; do not use this phase"))
		}
	}()
	manifest := textRoleDurableStateCapture{Role: input.Role, Phase: phase, InputSHA256: hex.EncodeToString(inputDigest[:]), SequentialRead: true}
	for _, root := range roots {
		captured, err := copyTextRoleDurableRoot(root, output)
		if err != nil {
			return err
		}
		manifest.Roots = append(manifest.Roots, captured)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := writeTextRoleDurableFile(filepath.Join(input.Output, phase+".durable.json"), encoded); err != nil {
		return err
	}
	completed = true
	return nil
}

func textRoleDurableRoots(input textRoleProcessInput) []textRoleDurableRoot {
	var roots []textRoleDurableRoot
	if input.StateRoot != "" {
		roots = append(roots, textRoleDurableRoot{name: "local-role-state", path: input.StateRoot})
	}
	if input.Root != "" {
		name := "role"
		if input.Role == "publisher" {
			name = "publisher"
		}
		roots = append(roots, textRoleDurableRoot{name: name, path: input.Root})
	}
	if input.AdmissionRoot != "" {
		roots = append(roots, textRoleDurableRoot{name: "admission", path: input.AdmissionRoot})
	}
	return roots
}

func startTextPublisherDurableCapture(t *testing.T, root, publisherRoot string) *textRoleProcess {
	t.Helper()
	input := textRoleProcessInput{Role: "publisher", Root: publisherRoot, Output: filepath.Join(root, "publisher")}
	if err := os.Mkdir(input.Output, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(input.Output, "input.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	clear(raw)
	process := &textRoleProcess{output: input.Output}
	process.capture = func(phase string) {
		t.Helper()
		if err := captureTextRoleDurableState(input, path, phase); err != nil {
			t.Fatal(err)
		}
	}
	process.dump = func(string) {}
	process.stop = func() error { return captureTextRoleDurableState(input, path, "stopped") }
	return process
}

func textRoleObservationCaptureRoot(root string) (string, error) {
	worktree, err := textRoleObservationWorktree()
	if err != nil {
		return "", err
	}
	return textRoleObservationCaptureRootAgainstWorktree(root, worktree)
}

func textRoleObservationCaptureRootAgainstWorktree(root, worktree string) (string, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "", errors.New("capture root must be a clean absolute path")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("capture root must be a non-symlink directory")
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	canonicalWorktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return "", err
	}
	if durablePathContains(canonicalWorktree, canonicalRoot) {
		return "", errors.New("capture root must be outside the Git worktree")
	}
	return canonicalRoot, nil
}

func textRoleObservationWorktree() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Lstat(filepath.Join(directory, ".git")); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("Git worktree root was not found")
		}
		directory = parent
	}
}

func TestTextRoleObservationCaptureRootRejectsWorktree(t *testing.T) {
	worktree, err := textRoleObservationWorktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := textRoleObservationCaptureRoot(worktree); err == nil || !strings.Contains(err.Error(), "outside the Git worktree") {
		t.Fatalf("capture root error = %v, want worktree rejection", err)
	}
}

func TestTextRoleObservationCaptureRootRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "capture-root")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := textRoleObservationCaptureRoot(link); err == nil || !strings.Contains(err.Error(), "non-symlink directory") {
		t.Fatalf("capture root error = %v, want symlink rejection", err)
	}
}

func TestTextRoleObservationCaptureRootRejectsIntermediateSymlink(t *testing.T) {
	worktree := t.TempDir()
	capture := filepath.Join(worktree, "captures")
	if err := os.Mkdir(capture, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "worktree-link")
	if err := os.Symlink(worktree, link); err != nil {
		t.Fatal(err)
	}
	if _, err := textRoleObservationCaptureRootAgainstWorktree(filepath.Join(link, "captures"), worktree); err == nil || !strings.Contains(err.Error(), "outside the Git worktree") {
		t.Fatalf("capture root error = %v, want intermediate symlink rejection", err)
	}
}

func durableRootWithinRoleOutput(output, root string) error {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return errors.New("root is not a clean absolute path")
	}
	if durablePathContains(root, output) || durablePathContains(output, root) {
		return errors.New("root overlaps role output")
	}
	return nil
}

func durablePathContains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && (relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)))
}

func writeTextRoleDurableFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(data, '\n'))
	syncErr, closeErr := file.Sync(), file.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}

func verifyTextRoleDurableStateCapture(t *testing.T, output string, phases ...string) {
	t.Helper()
	input, err := os.ReadFile(filepath.Join(output, "input.json"))
	if err != nil {
		t.Fatal(err)
	}
	inputDigest := sha256.Sum256(input)
	for _, phase := range phases {
		raw, err := os.ReadFile(filepath.Join(output, phase+".durable.json"))
		if err != nil {
			t.Fatal(err)
		}
		var capture textRoleDurableStateCapture
		if err := json.Unmarshal(raw, &capture); err != nil {
			t.Fatal(err)
		}
		if capture.Role == "" || capture.Phase != phase || !capture.SequentialRead || capture.InputSHA256 != hex.EncodeToString(inputDigest[:]) || len(capture.Roots) == 0 {
			t.Fatalf("invalid durable capture for %s: %#v", phase, capture)
		}
		for _, root := range capture.Roots {
			for _, file := range root.Files {
				path, err := textRoleDurablePath(output, filepath.Join(phase+"-durable", root.Name, filepath.FromSlash(file.Path)))
				if err != nil {
					t.Fatal(err)
				}
				copied, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				digest := sha256.Sum256(copied)
				if int64(len(copied)) != file.Bytes || hex.EncodeToString(digest[:]) != file.SHA256 {
					t.Fatalf("durable capture hash mismatch for %s", path)
				}
			}
		}
	}
}

func textRoleDurablePath(output, relative string) (string, error) {
	path := filepath.Join(output, relative)
	if !durablePathContains(output, path) {
		return "", errors.New("durable path is outside role output")
	}
	return path, nil
}

func writeAndVerifyTextRoleDurableReceipt(t *testing.T, output, carrier string, processes []*textRoleProcess, phases ...string) {
	t.Helper()
	if len(phases) == 0 {
		phases = []string{"startup", "published", "withdrawn", "stopped"}
	}
	receipt := textRoleDurableReceipt{Carrier: carrier}
	for _, process := range processes {
		inputPath := filepath.Join(process.output, "input.json")
		input, err := os.ReadFile(inputPath)
		if err != nil {
			t.Fatal(err)
		}
		var declared struct{ Role string }
		if err := json.Unmarshal(input, &declared); err != nil {
			t.Fatal(err)
		}
		inputDigest := sha256.Sum256(input)
		role := textRoleDurableReceiptRole{Path: filepath.Base(process.output), Role: declared.Role, InputSHA256: hex.EncodeToString(inputDigest[:])}
		for _, phase := range phases {
			path := filepath.Join(process.output, phase+".durable.json")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var capture textRoleDurableStateCapture
			if err := json.Unmarshal(raw, &capture); err != nil {
				t.Fatal(err)
			}
			if capture.Role != declared.Role || capture.Phase != phase || capture.InputSHA256 != role.InputSHA256 || !capture.SequentialRead {
				t.Fatalf("durable receipt manifest does not bind role and phase: %#v", capture)
			}
			digest := sha256.Sum256(raw)
			roots := make([]string, 0, len(capture.Roots))
			for _, root := range capture.Roots {
				roots = append(roots, root.Name)
			}
			role.Phases = append(role.Phases, textRoleDurableReceiptPhase{Phase: phase, Manifest: filepath.ToSlash(filepath.Join(role.Path, phase+".durable.json")), ManifestSHA256: hex.EncodeToString(digest[:]), Roots: roots})
		}
		receipt.Roles = append(receipt.Roles, role)
	}
	if len(receipt.Roles) != len(processes) {
		t.Fatalf("durable receipt roles = %d, want %d", len(receipt.Roles), len(processes))
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(output, "durable-receipt.json")
	if err := writeTextRoleDurableFile(path, encoded); err != nil {
		t.Fatal(err)
	}
	verifyTextRoleDurableReceipt(t, output, carrier, processes, phases...)
}

func verifyTextRoleDurableReceipt(t *testing.T, output, carrier string, processes []*textRoleProcess, phases ...string) {
	t.Helper()
	if len(phases) == 0 {
		phases = []string{"startup", "published", "withdrawn", "stopped"}
	}
	raw, err := os.ReadFile(filepath.Join(output, "durable-receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipt textRoleDurableReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Carrier != carrier || len(receipt.Roles) != len(processes) {
		t.Fatalf("durable receipt = %#v", receipt)
	}
	seen := make(map[string]bool, len(receipt.Roles))
	for _, role := range receipt.Roles {
		if seen[role.Path] || role.Role == "" || len(role.Phases) != len(phases) {
			t.Fatalf("invalid durable receipt role %#v", role)
		}
		seen[role.Path] = true
		for index, phase := range role.Phases {
			if phase.Phase != phases[index] || len(phase.Roots) == 0 || filepath.Base(phase.Manifest) != phase.Phase+".durable.json" {
				t.Fatalf("invalid durable receipt phase %#v", phase)
			}
			path, err := textRoleDurablePath(output, filepath.FromSlash(phase.Manifest))
			if err != nil {
				t.Fatal(err)
			}
			captured, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var capture textRoleDurableStateCapture
			if err := json.Unmarshal(captured, &capture); err != nil {
				t.Fatal(err)
			}
			if capture.Role != role.Role || capture.Phase != phase.Phase || capture.InputSHA256 != role.InputSHA256 || !capture.SequentialRead || len(capture.Roots) != len(phase.Roots) {
				t.Fatalf("durable receipt manifest does not bind role and phase: %#v", capture)
			}
			for index, root := range capture.Roots {
				if phase.Roots[index] != root.Name {
					t.Fatalf("durable receipt root %d = %q, want %q", index, phase.Roots[index], root.Name)
				}
			}
			digest := sha256.Sum256(captured)
			if hex.EncodeToString(digest[:]) != phase.ManifestSHA256 {
				t.Fatalf("durable receipt hash mismatch for %s", phase.Manifest)
			}
		}
	}
}
