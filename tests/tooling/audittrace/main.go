package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type traceManifest struct {
	SchemaVersion    int            `json:"schema_version"`
	CriticalPatterns []string       `json:"critical_patterns"`
	Gates            []traceGate    `json:"gates"`
	Findings         []traceFinding `json:"findings"`
}

type traceGate struct {
	ID    string `json:"id"`
	CIJob string `json:"ci_job"`
}

type traceFinding struct {
	AuditID       string          `json:"audit_id"`
	Issue         string          `json:"issue"`
	Priority      string          `json:"priority"`
	CriticalFiles []string        `json:"critical_files"`
	Evidence      []traceEvidence `json:"evidence"`
}

type traceEvidence struct {
	Kind   string   `json:"kind"`
	File   string   `json:"file"`
	Name   string   `json:"name"`
	Match  string   `json:"match,omitempty"`
	Gate   string   `json:"gate"`
	Covers []string `json:"covers"`
}

type backlogFinding struct {
	AuditID  string
	Issue    string
	Priority string
}

var backlogRow = regexp.MustCompile(`(?m)^\|\s*([A-Z]+-\d+)\s*\|\s*(ARD-\d+)\s*\|\s*[^|]+\|\s*(P[12])\s*\|`)
var remediationLedgerRow = regexp.MustCompile(`(?m)^\|\s*([A-Z]+-\d+)\s*/\s*(ARD-\d+)\s*\|\s*(P[12])\s*\|`)
var repositoryScriptReference = regexp.MustCompile(`(?:\./|/release/v[0-9]+/)?((?:tests|scripts)/[A-Za-z0-9_./-]+\.(?:ps1|sh))`)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, _ io.Writer) error {
	flags := flag.NewFlagSet("audittrace", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", ".", "repository root")
	manifestPath := flags.String("manifest", "tests/ci/audit-test-traceability.json", "traceability manifest")
	backlogPath := flags.String("backlog", "docs/engineering/current-remediation-ledger.md", "current remediation ledger")
	workflowPath := flags.String("workflow", ".github/workflows/ci.yml", "CI workflow")
	base := flags.String("base", "", "optional Git base for critical-file diff coverage")
	if err := flags.Parse(args); err != nil {
		return err
	}

	manifest, err := loadManifest(filepath.Join(*root, filepath.FromSlash(*manifestPath)))
	if err != nil {
		return err
	}
	backlog, err := loadBacklog(filepath.Join(*root, filepath.FromSlash(*backlogPath)))
	if err != nil {
		return err
	}
	if err := validateP1Coverage(manifest, backlog); err != nil {
		return err
	}
	if err := validateManifest(*root, filepath.Join(*root, filepath.FromSlash(*workflowPath)), manifest); err != nil {
		return err
	}
	if strings.TrimSpace(*base) != "" {
		if err := validateCriticalDiff(*root, *base, manifest); err != nil {
			return err
		}
	}
	_, _ = fmt.Fprintf(stdout, "audit traceability valid: %d findings, %d gates", len(manifest.Findings), len(manifest.Gates))
	if strings.TrimSpace(*base) != "" {
		_, _ = fmt.Fprintf(stdout, ", base %s", *base)
	}
	_, _ = fmt.Fprintln(stdout)
	return nil
}

func loadManifest(path string) (traceManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return traceManifest{}, fmt.Errorf("read traceability manifest: %w", err)
	}
	var manifest traceManifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return traceManifest{}, fmt.Errorf("decode traceability manifest: %w", err)
	}
	return manifest, nil
}

func loadBacklog(path string) ([]backlogFinding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read remediation ledger: %w", err)
	}
	ledgerMatches := remediationLedgerRow.FindAllStringSubmatch(string(data), -1)
	legacyMatches := backlogRow.FindAllStringSubmatch(string(data), -1)
	findings := make([]backlogFinding, 0, len(ledgerMatches)+len(legacyMatches))
	for _, match := range ledgerMatches {
		findings = append(findings, backlogFinding{AuditID: match[1], Issue: match[2], Priority: match[3]})
	}
	matches := legacyMatches
	for _, match := range matches {
		findings = append(findings, backlogFinding{AuditID: match[1], Issue: match[2], Priority: match[3]})
	}
	return findings, nil
}

func validateP1Coverage(manifest traceManifest, backlog []backlogFinding) error {
	index := make(map[string]traceFinding, len(manifest.Findings))
	for _, finding := range manifest.Findings {
		index[finding.AuditID] = finding
	}
	backlogIndex := make(map[string]backlogFinding, len(backlog))
	for _, required := range backlog {
		if _, duplicate := backlogIndex[required.AuditID]; duplicate {
			return fmt.Errorf("audit backlog finding %s is duplicated", required.AuditID)
		}
		backlogIndex[required.AuditID] = required
		if required.Priority != "P1" {
			continue
		}
		finding, ok := index[required.AuditID]
		if !ok {
			return fmt.Errorf("P1 finding %s is missing from traceability manifest", required.AuditID)
		}
		if finding.Issue != required.Issue || finding.Priority != required.Priority {
			return fmt.Errorf("finding %s metadata does not match backlog", required.AuditID)
		}
	}
	for _, finding := range manifest.Findings {
		required, ok := backlogIndex[finding.AuditID]
		if !ok {
			return fmt.Errorf("finding %s is not declared in audit backlog", finding.AuditID)
		}
		if finding.Issue != required.Issue || finding.Priority != required.Priority {
			return fmt.Errorf("finding %s metadata does not match backlog", finding.AuditID)
		}
	}
	return nil
}

func validateManifest(root, workflowPath string, manifest traceManifest) error {
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("traceability schema version must be 1")
	}
	if len(manifest.CriticalPatterns) == 0 {
		return fmt.Errorf("traceability manifest requires critical patterns")
	}
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		return fmt.Errorf("read CI workflow: %w", err)
	}
	gates := make(map[string]traceGate, len(manifest.Gates))
	gateBodies := make(map[string][]byte, len(manifest.Gates))
	for _, gate := range manifest.Gates {
		if gate.ID == "" || gate.CIJob == "" {
			return fmt.Errorf("traceability gate id and ci_job are required")
		}
		if _, duplicate := gates[gate.ID]; duplicate {
			return fmt.Errorf("traceability gate %s is duplicated", gate.ID)
		}
		body, ok := workflowJobBody(workflow, gate.CIJob)
		if !ok {
			return fmt.Errorf("CI job %s for gate %s is not declared", gate.CIJob, gate.ID)
		}
		gates[gate.ID] = gate
		gateBodies[gate.ID] = body
	}

	seenFindings := make(map[string]struct{}, len(manifest.Findings))
	for _, finding := range manifest.Findings {
		if _, duplicate := seenFindings[finding.AuditID]; duplicate {
			return fmt.Errorf("finding %s is duplicated", finding.AuditID)
		}
		seenFindings[finding.AuditID] = struct{}{}
		if len(finding.CriticalFiles) == 0 || len(finding.Evidence) == 0 {
			return fmt.Errorf("finding %s requires critical files and evidence", finding.AuditID)
		}
		critical := make(map[string]bool, len(finding.CriticalFiles))
		for _, file := range finding.CriticalFiles {
			if err := validateRepositoryPath(root, file); err != nil {
				return fmt.Errorf("finding %s critical file: %w", finding.AuditID, err)
			}
			if !matchesAny(file, manifest.CriticalPatterns) {
				return fmt.Errorf("finding %s critical file %s is outside critical patterns", finding.AuditID, file)
			}
			critical[file] = false
		}
		for _, evidence := range finding.Evidence {
			if _, ok := gates[evidence.Gate]; !ok {
				return fmt.Errorf("finding %s evidence %s references unknown gate %s", finding.AuditID, evidence.Name, evidence.Gate)
			}
			if err := validateRepositoryPath(root, evidence.File); err != nil {
				return fmt.Errorf("finding %s evidence: %w", finding.AuditID, err)
			}
			if len(evidence.Covers) == 0 {
				return fmt.Errorf("finding %s evidence %s covers no critical files", finding.AuditID, evidence.Name)
			}
			for _, covered := range evidence.Covers {
				if _, ok := critical[covered]; !ok {
					return fmt.Errorf("finding %s evidence %s covers undeclared critical file %s", finding.AuditID, evidence.Name, covered)
				}
				critical[covered] = true
			}
			if err := validateEvidence(root, gateBodies[evidence.Gate], evidence); err != nil {
				return fmt.Errorf("finding %s: %w", finding.AuditID, err)
			}
		}
		for file, covered := range critical {
			if !covered {
				return fmt.Errorf("finding %s critical file %s has no deterministic evidence", finding.AuditID, file)
			}
		}
	}
	for _, pattern := range manifest.CriticalPatterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("critical pattern cannot be empty")
		}
		candidate := strings.TrimSuffix(pattern, "/**")
		if _, err := path.Match(candidate, "probe"); err != nil {
			return fmt.Errorf("critical pattern %q is invalid: %w", pattern, err)
		}
	}
	return nil
}

func validateRepositoryPath(root, relative string) error {
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, `\`) {
		return fmt.Errorf("repository path %q is not canonical", relative)
	}
	clean := path.Clean(relative)
	if clean != relative || clean == "." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("repository path %q is not canonical", relative)
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return fmt.Errorf("repository path %s does not exist", relative)
	}
	if info.IsDir() {
		return fmt.Errorf("repository path %s is not a file", relative)
	}
	return nil
}

func validateEvidence(root string, gateBody []byte, evidence traceEvidence) error {
	switch evidence.Kind {
	case "go_test":
		if !strings.HasPrefix(evidence.Name, "Test") {
			return fmt.Errorf("go test evidence %s must name a Test function", evidence.Name)
		}
		if !strings.HasSuffix(evidence.File, "_test.go") {
			return fmt.Errorf("go test evidence %s must be declared in a _test.go file", evidence.Name)
		}
		parsed, err := parser.ParseFile(
			token.NewFileSet(),
			filepath.Join(root, filepath.FromSlash(evidence.File)),
			nil,
			0,
		)
		if err != nil {
			return fmt.Errorf("parse go test evidence %s: %w", evidence.File, err)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.Name == evidence.Name {
				packageArgument := "./" + path.Dir(evidence.File)
				if !goTestPackageSelected(gateBody, packageArgument) {
					return fmt.Errorf("CI gate does not run Go test package %s for %s", packageArgument, evidence.Name)
				}
				return nil
			}
		}
		return fmt.Errorf("%s is not declared in %s", evidence.Name, evidence.File)
	case "ci_contract":
		if evidence.Name == "" || evidence.Match == "" {
			return fmt.Errorf("CI contract evidence requires name and match")
		}
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(evidence.File)))
		if err != nil {
			return err
		}
		if !bytes.Contains(contents, []byte(evidence.Match)) {
			return fmt.Errorf("CI contract %s marker is not present in %s", evidence.Name, evidence.File)
		}
		if !ciContractReachable(root, gateBody, evidence.File) {
			return fmt.Errorf("CI gate does not execute contract %s", evidence.File)
		}
		return nil
	default:
		return fmt.Errorf("evidence %s has unsupported kind %s", evidence.Name, evidence.Kind)
	}
}

func goTestPackageSelected(gateBody []byte, packageArgument string) bool {
	lines := strings.Split(string(gateBody), "\n")
	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		commandAt := strings.Index(line, "go test")
		if commandAt < 0 {
			continue
		}
		prefix := strings.TrimSpace(line[:commandAt])
		if prefix != "" && prefix != "run:" && prefix != "- run:" {
			continue
		}
		arguments := strings.Fields(line[commandAt+len("go test"):])
		for next := index + 1; next < len(lines); next++ {
			continuation := strings.TrimSpace(lines[next])
			if !strings.HasPrefix(continuation, "./") && !strings.HasPrefix(continuation, "-") {
				break
			}
			if strings.HasPrefix(continuation, "- name:") ||
				strings.HasPrefix(continuation, "- run:") ||
				strings.HasPrefix(continuation, "- uses:") {
				break
			}
			arguments = append(arguments, strings.Fields(continuation)...)
			index = next
		}
		if packageSelectedByArguments(arguments, packageArgument) {
			return true
		}
	}
	return false
}

func packageSelectedByArguments(arguments []string, packageArgument string) bool {
	selected := make(map[string]struct{}, len(arguments))
	for _, argument := range arguments {
		selected[strings.Trim(argument, `"'`)] = struct{}{}
	}
	if _, ok := selected[packageArgument]; ok {
		return true
	}
	if _, ok := selected[packageArgument+"/..."]; ok {
		return true
	}
	directory := strings.TrimPrefix(packageArgument, "./")
	for {
		parent := path.Dir(directory)
		if parent == "." || parent == directory {
			_, ok := selected["./..."]
			return ok
		}
		if _, ok := selected["./"+parent+"/..."]; ok {
			return true
		}
		directory = parent
	}
}

func workflowJobBody(workflow []byte, job string) ([]byte, bool) {
	header := regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(job) + `:\s*$`).FindIndex(workflow)
	if header == nil {
		return nil, false
	}
	tail := workflow[header[1]:]
	next := regexp.MustCompile(`(?m)^  [A-Za-z0-9_-]+:\s*$`).FindIndex(tail)
	if next == nil {
		return tail, true
	}
	return tail[:next[0]], true
}

func ciContractReachable(root string, gateBody []byte, target string) bool {
	queue := [][]byte{gateBody}
	visited := make(map[string]struct{})
	for len(queue) > 0 {
		contents := queue[0]
		queue = queue[1:]
		if bytes.Contains(contents, []byte(target)) {
			return true
		}
		for _, match := range repositoryScriptReference.FindAllSubmatch(contents, -1) {
			reference := string(match[1])
			if reference == target {
				return true
			}
			if _, seen := visited[reference]; seen {
				continue
			}
			visited[reference] = struct{}{}
			nested, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(reference)))
			if err == nil {
				queue = append(queue, nested)
			}
		}
	}
	return false
}

func validateCriticalDiff(root, base string, manifest traceManifest) error {
	command := exec.Command("git", "-C", root, "diff", "--name-only", "--diff-filter=ACMRTD", base+"...HEAD")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inspect critical-file diff from %s: %w: %s", base, err, strings.TrimSpace(string(output)))
	}
	mapped := make(map[string]struct{})
	for _, finding := range manifest.Findings {
		for _, file := range finding.CriticalFiles {
			mapped[file] = struct{}{}
		}
	}
	for _, raw := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		file := filepath.ToSlash(strings.TrimSpace(raw))
		if file == "" || strings.HasSuffix(file, "_test.go") || !matchesAny(file, manifest.CriticalPatterns) {
			continue
		}
		if _, ok := mapped[file]; !ok {
			return fmt.Errorf("changed critical file %s is not mapped to deterministic evidence", file)
		}
	}
	return nil
}

func matchesAny(file string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.HasSuffix(pattern, "/**") {
			root := strings.TrimSuffix(pattern, "**")
			if strings.HasPrefix(file, root) && len(file) > len(root) {
				return true
			}
			continue
		}
		matched, _ := path.Match(pattern, file)
		if matched {
			return true
		}
	}
	return false
}
