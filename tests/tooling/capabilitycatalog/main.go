// Package main validates and generates the capability/evidence catalogue.
// It does not infer product truth or execute evidence gates.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"ardents/tests/tooling/doccontract"
	"ardents/tests/tooling/scenariocatalog"
)

var requiredDomains = []string{
	"application", "content-transfer", "discovery", "identity",
	"network", "node", "operations-release", "workload-hosting",
}

var requiredCapabilities = map[string]string{
	"application.discovery":              "application",
	"application.hosting":                "workload-hosting",
	"application.installation-content":   "application",
	"application.messaging":              "application",
	"content.operator-lifecycle":         "content-transfer",
	"deployment.kubernetes":              "operations-release",
	"deployment.multi-host":              "operations-release",
	"discovery.operator-resolution":      "discovery",
	"hosting.operator-publication":       "workload-hosting",
	"identity.principal-access":          "identity",
	"network.quic-webtransport-webrtc":   "network",
	"network.waku-foundation":            "network",
	"node.lifecycle":                     "node",
	"operations.backup-upgrade-rollback": "operations-release",
	"operations.configuration-reload":    "operations-release",
	"operations.diagnostics":             "operations-release",
	"operations.native-installation":     "operations-release",
	"operator.command-interface":         "node",
	"realm.channel-grant-authority":      "identity",
	"release.artifacts-provenance":       "operations-release",
	"sdk.non-go-or-remote":               "application",
	"service.direct-interaction":         "workload-hosting",
	"transfer.replication":               "content-transfer",
	"workload.lifecycle":                 "workload-hosting",
}

var ownedEvidenceEnvironments = map[string]struct{}{
	"canonical-linux-release": {},
	"linux-container":         {},
	"linux-multinode":         {},
	"linux-systemd":           {},
	"local":                   {},
	"release-matrix":          {},
	"ubuntu-latest":           {},
	"windows-latest":          {},
}

type catalogue struct {
	SchemaVersion          int                     `json:"schema_version"`
	ReportedSourceCommit   string                  `json:"reported_source_commit"`
	Domains                []domain                `json:"domains"`
	EvidenceGates          []evidenceGate          `json:"evidence_gates"`
	QualificationSnapshots []qualificationSnapshot `json:"qualification_snapshots"`
	Capabilities           []capability            `json:"capabilities"`
}
type domain struct {
	ID       string `json:"id"`
	Owner    string `json:"owner"`
	Required bool   `json:"required"`
}
type supportedInterface struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Contracts []string `json:"contracts"`
}
type capabilityStatus struct {
	Implemented           string `json:"implemented"`
	Reachable             string `json:"reachable"`
	Operable              string `json:"operable"`
	Qualified             string `json:"qualified"`
	AtCommit              string `json:"at_commit"`
	QualificationSnapshot string `json:"qualification_snapshot"`
}
type capability struct {
	ID                    string               `json:"id"`
	UserOutcome           string               `json:"user_outcome"`
	Domain                string               `json:"domain"`
	DomainOwner           string               `json:"domain_owner"`
	SupportedInterfaces   []supportedInterface `json:"supported_interfaces"`
	ImplementationOwners  []string             `json:"implementation_owners"`
	OperabilitySurfaces   []string             `json:"operability_surfaces"`
	EvidenceOwner         string               `json:"evidence_owner"`
	RequiredEvidenceGates []string             `json:"required_evidence_gates"`
	Status                capabilityStatus     `json:"status"`
	ResearchClass         string               `json:"research_class"`
	ADRs                  []string             `json:"adrs"`
	ResearchPackets       []string             `json:"research_packets"`
	Backlog               []string             `json:"backlog"`
	Constraints           []string             `json:"constraints"`
	UnsupportedBehavior   []string             `json:"unsupported_behavior"`
	ActiveDocuments       []string             `json:"active_documents"`
	Supersedes            []string             `json:"supersedes,omitempty"`
	SupersededBy          []string             `json:"superseded_by,omitempty"`
}
type evidenceGate struct {
	ID                  string `json:"id"`
	Owner               string `json:"owner"`
	Kind                string `json:"kind"`
	CIJob               string `json:"ci_job"`
	ScenarioID          string `json:"scenario_id,omitempty"`
	RequiredEnvironment string `json:"required_environment"`
	ReleaseGate         bool   `json:"release_gate"`
}
type qualificationSnapshot struct {
	ID           string           `json:"id"`
	SourceCommit string           `json:"source_commit"`
	Environment  string           `json:"environment"`
	CleanRun     bool             `json:"clean_run"`
	Results      []evidenceResult `json:"results"`
}
type evidenceResult struct {
	Gate        string `json:"gate"`
	Outcome     string `json:"outcome"`
	Environment string `json:"environment"`
	Artifact    string `json:"artifact"`
	SHA256      string `json:"sha256"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("capabilitycatalog", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", ".", "repository root")
	check := flags.Bool("check", false, "validate generated outputs")
	generate := flags.Bool("generate", false, "write generated outputs")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *check == *generate {
		return errors.New("exactly one of -check or -generate is required")
	}
	c, err := loadCatalogue(filepath.Join(*root, "docs", "engineering", "capabilities.json"))
	if err != nil {
		return err
	}
	if err := validateCatalogue(*root, c); err != nil {
		return err
	}
	projection := renderProjection(c)
	target := filepath.Join(*root, "docs", "engineering", "capability-evidence-register.md")
	if *generate {
		declared := declaredDocuments(c)
		if err := validateDocumentMarkers(*root, declared, false); err != nil {
			return err
		}
		active, err := prepareActiveDocuments(*root, c)
		if err != nil {
			return err
		}
		outputs := map[string][]byte{target: projection}
		for document, raw := range active {
			outputs[filepath.Join(*root, filepath.FromSlash(document))] = raw
		}
		if err := writeOutputSet(outputs, atomicWrite); err != nil {
			return err
		}
	} else {
		if err := checkBytes(target, projection, "generated projection differs"); err != nil {
			return err
		}
		if err := checkActiveDocuments(*root, c); err != nil {
			return err
		}
	}
	qualified := 0
	for _, item := range c.Capabilities {
		if item.Status.Qualified == "yes" {
			qualified++
		}
	}
	_, _ = fmt.Fprintf(stdout, "capability catalogue valid: %d capabilities, %d domains, %d qualified\n", len(c.Capabilities), len(c.Domains), qualified)
	return nil
}

func loadCatalogue(file string) (catalogue, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return catalogue{}, fmt.Errorf("read capability catalogue: %w", err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return catalogue{}, errors.New("capability catalogue is empty")
	}
	if len(raw) > 2<<20 {
		return catalogue{}, errors.New("capability catalogue exceeds 2 MiB")
	}
	if !utf8.Valid(raw) {
		return catalogue{}, errors.New("capability catalogue is not valid UTF-8")
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return catalogue{}, err
	}
	var c catalogue
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return catalogue{}, fmt.Errorf("decode capability catalogue (unknown field or invalid JSON): %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return catalogue{}, errors.New("trailing JSON after capability catalogue")
	}
	return c, nil
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]struct{}{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key := keyToken.(string)
				if _, exists := seen[key]; exists {
					return fmt.Errorf("duplicate JSON key %q", key)
				}
				seen[key] = struct{}{}
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
		}
	}
	if err := walk(); err != nil {
		return fmt.Errorf("inspect capability catalogue JSON: %w", err)
	}
	return nil
}

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)
var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func validateCatalogue(root string, c catalogue) error {
	if c.SchemaVersion != 1 {
		return fmt.Errorf("unsupported capability schema_version %d", c.SchemaVersion)
	}
	if !commitPattern.MatchString(c.ReportedSourceCommit) {
		return errors.New("reported_source_commit must be a full lowercase Git SHA")
	}
	if len(c.Capabilities) == 0 {
		return errors.New("catalogue has no capabilities")
	}
	if len(c.Capabilities) > 256 || len(c.Domains) > 64 || len(c.EvidenceGates) > 256 || len(c.QualificationSnapshots) > 256 {
		return errors.New("capability catalogue exceeds collection limits")
	}
	domains := map[string]domain{}
	for _, item := range c.Domains {
		if !idPattern.MatchString(item.ID) || validateInlineText(item.Owner) != nil {
			return fmt.Errorf("domain %q has invalid id or owner", item.ID)
		}
		if _, exists := domains[item.ID]; exists {
			return fmt.Errorf("duplicate domain %s", item.ID)
		}
		domains[item.ID] = item
	}
	for _, required := range requiredDomains {
		item, ok := domains[required]
		if !ok || !item.Required {
			return fmt.Errorf("required domain %s is missing", required)
		}
	}
	if len(domains) != len(requiredDomains) {
		return errors.New("unknown or duplicate capability domain")
	}
	gates, err := validateGates(root, c.EvidenceGates)
	if err != nil {
		return err
	}
	snapshots, err := validateSnapshots(root, c.QualificationSnapshots, gates)
	if err != nil {
		return err
	}
	if err := validateReadinessClaims(root, c); err != nil {
		return err
	}
	activeDocuments, err := activeDocumentSet(root)
	if err != nil {
		return err
	}
	ids, counts, capabilityByID := map[string]struct{}{}, map[string]int{}, map[string]capability{}
	for index, item := range c.Capabilities {
		if item.Status.AtCommit != c.ReportedSourceCommit {
			return fmt.Errorf("capabilities[%d] %s: status commit differs from reported_source_commit", index, item.ID)
		}
		if err := validateCapability(root, item, domains, gates, snapshots, activeDocuments); err != nil {
			return fmt.Errorf("capabilities[%d] %s: %w", index, item.ID, err)
		}
		if _, exists := ids[item.ID]; exists {
			return fmt.Errorf("duplicate capability %s", item.ID)
		}
		ids[item.ID], capabilityByID[item.ID], counts[item.Domain] = struct{}{}, item, counts[item.Domain]+1
	}
	for _, required := range requiredDomains {
		if counts[required] == 0 {
			return fmt.Errorf("required domain %s has no capabilities", required)
		}
	}
	requiredIDs := make([]string, 0, len(requiredCapabilities))
	for id := range requiredCapabilities {
		requiredIDs = append(requiredIDs, id)
	}
	sort.Strings(requiredIDs)
	for _, id := range requiredIDs {
		domain := requiredCapabilities[id]
		item, ok := capabilityByID[id]
		if !ok {
			return fmt.Errorf("required initial capability %s is missing from domain %s", id, domain)
		}
		if item.Domain != domain {
			return fmt.Errorf("required initial capability %s moved from domain %s to %s", id, domain, item.Domain)
		}
	}
	return validateSupersession(c.Capabilities, ids)
}

func validateCapability(root string, c capability, domains map[string]domain, gates map[string]evidenceGate, snapshots map[string]qualificationSnapshot, activeDocuments map[string]struct{}) error {
	if !idPattern.MatchString(c.ID) || validateInlineText(c.UserOutcome) != nil {
		return errors.New("canonical id and user_outcome are required")
	}
	if _, ok := domains[c.Domain]; !ok || validateInlineText(c.DomainOwner) != nil {
		return errors.New("known domain and domain_owner are required")
	}
	if len(c.SupportedInterfaces) == 0 {
		return errors.New("supported interface is required")
	}
	for _, supported := range c.SupportedInterfaces {
		if !idPattern.MatchString(supported.ID) || !oneOf(supported.Kind, "operator", "application", "sdk", "local", "artifact", "none") {
			return errors.New("supported interface is invalid")
		}
		if supported.Kind == "none" && len(c.UnsupportedBehavior) == 0 {
			return errors.New("none interface requires unsupported behavior")
		}
		for _, contract := range supported.Contracts {
			if err := validatePath(root, contract); err != nil {
				return err
			}
		}
	}
	if len(c.ImplementationOwners) == 0 {
		return errors.New("implementation owner is required")
	}
	if len(c.OperabilitySurfaces) == 0 {
		return errors.New("operability surface is required")
	}
	if strings.TrimSpace(c.EvidenceOwner) == "" {
		return errors.New("evidence_owner is required")
	}
	if err := validateInlineText(c.EvidenceOwner); err != nil {
		return fmt.Errorf("evidence_owner: %w", err)
	}
	for _, value := range append(append(append([]string{}, c.OperabilitySurfaces...), c.Constraints...), c.UnsupportedBehavior...) {
		if err := validateInlineText(value); err != nil {
			return fmt.Errorf("rendered catalogue text: %w", err)
		}
	}
	for _, owner := range c.ImplementationOwners {
		if err := validatePath(root, owner); err != nil {
			return err
		}
	}
	if len(c.RequiredEvidenceGates) == 0 {
		return errors.New("required evidence gate is required")
	}
	for _, gate := range c.RequiredEvidenceGates {
		if _, ok := gates[gate]; !ok {
			return fmt.Errorf("unknown evidence gate %s", gate)
		}
	}
	for _, status := range []struct{ name, value string }{
		{"implemented", c.Status.Implemented},
		{"reachable", c.Status.Reachable},
		{"operable", c.Status.Operable},
	} {
		if !oneOf(status.value, "yes", "partial", "no") {
			return fmt.Errorf("unknown %s status %q", status.name, status.value)
		}
	}
	if !oneOf(c.Status.Qualified, "yes", "no") || !commitPattern.MatchString(c.Status.AtCommit) {
		return errors.New("qualified status or status commit is invalid")
	}
	if !oneOf(c.ResearchClass, "R0", "R1", "R2", "R3", "Deferred") {
		return fmt.Errorf("unknown research class %q", c.ResearchClass)
	}
	for _, reference := range append(append(append([]string{}, c.ADRs...), c.ResearchPackets...), c.Backlog...) {
		if err := validatePath(root, reference); err != nil {
			return err
		}
	}
	for _, document := range c.ActiveDocuments {
		if err := validatePath(root, document); err != nil {
			return err
		}
		if _, ok := activeDocuments[document]; !ok {
			return fmt.Errorf("active document is outside the Markdown allowlist: %s", document)
		}
		if filepath.ToSlash(document) == "docs/engineering/capability-evidence-register.md" {
			continue
		}
	}
	if c.Status.Qualified == "yes" {
		snapshot, ok := snapshots[c.Status.QualificationSnapshot]
		if !ok {
			return errors.New("qualified capability requires a snapshot")
		}
		if err := validateQualification(c, snapshot, gates); err != nil {
			return err
		}
	} else if c.Status.QualificationSnapshot != "" {
		return errors.New("unqualified capability cannot select a qualification snapshot")
	}
	return nil
}

func validateGates(root string, values []evidenceGate) (map[string]evidenceGate, error) {
	if len(values) == 0 {
		return nil, errors.New("evidence gate catalogue is empty")
	}
	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		return nil, fmt.Errorf("read CI workflow: %w", err)
	}
	jobs, err := collectWorkflowJobs(workflow)
	if err != nil {
		return nil, err
	}
	scenarios, err := collectScenarioIDs(root)
	if err != nil {
		return nil, err
	}
	result := map[string]evidenceGate{}
	for _, gate := range values {
		if !idPattern.MatchString(gate.ID) || validateInlineText(gate.Owner) != nil || strings.TrimSpace(gate.RequiredEnvironment) == "" ||
			!oneOf(gate.Kind, "ci_job", "tagged_scenario", "release") {
			return nil, fmt.Errorf("evidence gate %q is invalid", gate.ID)
		}
		if _, ok := ownedEvidenceEnvironments[gate.RequiredEnvironment]; !ok {
			return nil, fmt.Errorf("evidence gate %s has unknown environment %s", gate.ID, gate.RequiredEnvironment)
		}
		if _, exists := result[gate.ID]; exists {
			return nil, fmt.Errorf("duplicate evidence gate %s", gate.ID)
		}
		if _, ok := jobs[gate.CIJob]; gate.CIJob == "" || !ok {
			return nil, fmt.Errorf("unknown CI job %s", gate.CIJob)
		}
		if gate.Kind == "tagged_scenario" {
			environment, ok := scenarios[gate.ScenarioID]
			if !ok {
				return nil, fmt.Errorf("unknown tagged scenario %s", gate.ScenarioID)
			}
			if gate.RequiredEnvironment != environment {
				return nil, fmt.Errorf("tagged scenario %s requires environment %s", gate.ScenarioID, environment)
			}
		} else if gate.ScenarioID != "" {
			return nil, fmt.Errorf("non-scenario gate %s declares a scenario", gate.ID)
		}
		if (gate.Kind == "release") != gate.ReleaseGate {
			return nil, fmt.Errorf("evidence gate %s has inconsistent release semantics", gate.ID)
		}
		result[gate.ID] = gate
	}
	return result, nil
}

func collectWorkflowJobs(raw []byte) (map[string]struct{}, error) {
	result, inJobs := map[string]struct{}{}, false
	jobLine := regexp.MustCompile(`^  ([A-Za-z0-9_-]+):\s*$`)
	for number, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if !inJobs {
			inJobs = strings.TrimSuffix(line, "\r") == "jobs:"
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if len(line) > 0 && line[0] != ' ' {
			break
		}
		if match := jobLine.FindStringSubmatch(strings.TrimSuffix(line, "\r")); match != nil {
			if _, duplicate := result[match[1]]; duplicate {
				return nil, fmt.Errorf("CI workflow repeats job %s", match[1])
			}
			result[match[1]] = struct{}{}
		} else if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
			return nil, fmt.Errorf("CI workflow has malformed job declaration at line %d", number+1)
		}
	}
	if !inJobs || len(result) == 0 {
		return nil, errors.New("CI workflow has no jobs")
	}
	return result, nil
}

func collectScenarioIDs(root string) (map[string]string, error) {
	result := map[string]string{}
	files, err := selectedScenarioFiles(root)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		parsed, parseErr := scenariocatalog.ParseFile(file)
		if parseErr != nil {
			return nil, fmt.Errorf("parse tagged scenario source %s: %w", filepath.ToSlash(file), parseErr)
		}
		for _, scenario := range parsed {
			if environment, exists := result[scenario.ScenarioID]; exists && environment != scenario.Environment {
				return nil, fmt.Errorf("tagged scenario %s has conflicting environments", scenario.ScenarioID)
			}
			result[scenario.ScenarioID] = scenario.Environment
		}
	}
	return result, nil
}

func selectedScenarioFiles(root string) ([]string, error) {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		packages, err := scenariocatalog.ListPackages(root, "integration e2e", []string{"./tests/..."})
		if err != nil {
			return nil, fmt.Errorf("select tagged scenario packages: %w", err)
		}
		return scenariocatalog.Files(packages), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var files []string
	err := filepath.WalkDir(filepath.Join(root, "tests"), func(file string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "testdata" {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(file, "_test.go") {
			return nil
		}
		files = append(files, file)
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("select fixture scenario files: %w", err)
	}
	return files, nil
}

func validateSnapshots(root string, values []qualificationSnapshot, gates map[string]evidenceGate) (map[string]qualificationSnapshot, error) {
	result := map[string]qualificationSnapshot{}
	for _, snapshot := range values {
		if !idPattern.MatchString(snapshot.ID) || !commitPattern.MatchString(snapshot.SourceCommit) || strings.TrimSpace(snapshot.Environment) == "" {
			return nil, fmt.Errorf("qualification snapshot %q is invalid", snapshot.ID)
		}
		if _, exists := result[snapshot.ID]; exists {
			return nil, fmt.Errorf("duplicate qualification snapshot %s", snapshot.ID)
		}
		seen := map[string]struct{}{}
		for _, evidence := range snapshot.Results {
			if _, ok := gates[evidence.Gate]; !ok || evidence.Outcome != "passed" {
				return nil, fmt.Errorf("snapshot %s has unknown or failed gate", snapshot.ID)
			}
			if evidence.Environment != gates[evidence.Gate].RequiredEnvironment {
				return nil, fmt.Errorf("snapshot %s gate %s has wrong environment", snapshot.ID, evidence.Gate)
			}
			if _, duplicate := seen[evidence.Gate]; duplicate {
				return nil, fmt.Errorf("snapshot %s repeats gate %s", snapshot.ID, evidence.Gate)
			}
			seen[evidence.Gate] = struct{}{}
			if err := validatePath(root, evidence.Artifact); err != nil {
				return nil, err
			}
			if !strings.HasPrefix(evidence.Artifact, "docs/engineering/evidence/") {
				return nil, fmt.Errorf("snapshot %s artifact is outside retained evidence", snapshot.ID)
			}
			raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(evidence.Artifact)))
			sum := sha256.Sum256(raw)
			if err != nil || hex.EncodeToString(sum[:]) != evidence.SHA256 {
				return nil, fmt.Errorf("snapshot %s artifact hash mismatch", snapshot.ID)
			}
		}
		result[snapshot.ID] = snapshot
	}
	return result, nil
}

func validateQualification(c capability, snapshot qualificationSnapshot, gates map[string]evidenceGate) error {
	if snapshot.SourceCommit != c.Status.AtCommit {
		return errors.New("qualification snapshot commit does not match capability")
	}
	if !snapshot.CleanRun {
		return errors.New("qualification snapshot must record a clean run")
	}
	passed, snapshotRelease := map[string]struct{}{}, false
	for _, item := range snapshot.Results {
		passed[item.Gate] = struct{}{}
		snapshotRelease = snapshotRelease || gates[item.Gate].ReleaseGate
	}
	requiredRelease := false
	releaseEnvironment := ""
	for _, required := range c.RequiredEvidenceGates {
		if _, ok := passed[required]; !ok {
			return fmt.Errorf("qualification snapshot is missing gate %s", required)
		}
		requiredRelease = requiredRelease || gates[required].ReleaseGate
		if gates[required].ReleaseGate {
			releaseEnvironment = gates[required].RequiredEnvironment
		}
	}
	if !requiredRelease {
		return errors.New("capability required evidence gates omit a release gate")
	}
	if !snapshotRelease {
		return errors.New("qualification snapshot is missing the release gate")
	}
	if snapshot.Environment != releaseEnvironment {
		return errors.New("qualification snapshot environment does not match the release gate")
	}
	return nil
}

func validatePath(root, value string) error {
	if value == "" || value != filepath.ToSlash(value) || path.Clean(value) != value || path.IsAbs(value) || strings.HasPrefix(value, "../") {
		return fmt.Errorf("referenced path %q is not canonical", value)
	}
	repository, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("open repository root: %w", err)
	}
	defer repository.Close()
	if _, err := repository.Stat(filepath.FromSlash(value)); err != nil {
		return fmt.Errorf("referenced path does not exist: %s", value)
	}
	return nil
}

func validateInlineText(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("value is empty")
	}
	if len(value) > 4096 {
		return errors.New("value exceeds 4096 bytes")
	}
	if strings.ContainsAny(value, "\r\n") {
		return errors.New("newlines are not allowed")
	}
	if strings.Contains(value, "<!-- capability-status:") {
		return errors.New("reserved capability marker text is not allowed")
	}
	return nil
}

func validateSupersession(values []capability, ids map[string]struct{}) error {
	edges := map[string][]string{}
	for _, item := range values {
		for _, target := range append(append([]string{}, item.Supersedes...), item.SupersededBy...) {
			if _, ok := ids[target]; !ok || target == item.ID {
				return fmt.Errorf("capability %s has invalid supersession target %s", item.ID, target)
			}
		}
		edges[item.ID] = append(edges[item.ID], item.SupersededBy...)
		for _, target := range item.Supersedes {
			edges[target] = append(edges[target], item.ID)
		}
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return errors.New("capability supersession cycle")
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, next := range edges[id] {
			if err := visit(next); err != nil {
				return err
			}
		}
		visiting[id], visited[id] = false, true
		return nil
	}
	for id := range ids {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func renderProjection(c catalogue) []byte {
	items := append([]capability(nil), c.Capabilities...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	var out strings.Builder
	out.WriteString("# Capability and evidence register\n\n")
	out.WriteString("<!-- Generated by `go run ./tests/tooling/capabilitycatalog -generate`; edit `capabilities.json`, not this file. -->\n\n")
	fmt.Fprintf(&out, "Reported source commit: `%s`.\n\n", c.ReportedSourceCommit)
	out.WriteString("| Capability | Domain | I | R | O | Q | Research |\n|---|---|:---:|:---:|:---:|:---:|---|\n")
	for _, item := range items {
		fmt.Fprintf(&out, "| `%s` | %s | %s | %s | %s | %s | %s |\n", item.ID, item.Domain, item.Status.Implemented, item.Status.Reachable, item.Status.Operable, item.Status.Qualified, item.ResearchClass)
	}
	out.WriteString("\n")
	for _, item := range items {
		out.WriteString(renderStatusBlock(item))
	}
	return []byte(strings.TrimRight(out.String(), "\n") + "\n")
}

func renderStatusBlock(item capability) string {
	var out strings.Builder
	fmt.Fprintf(&out, "<!-- capability-status:begin %s -->\n## `%s`\n\n%s\n\n", item.ID, item.ID, item.UserOutcome)
	fmt.Fprintf(&out, "- Domain: `%s`; owner: %s\n- Supported interfaces: %s\n- Implementation owners: %s\n- Operability: %s\n- Evidence owner: %s\n- Required evidence gates: %s\n- Status at `%s`: I=%s, R=%s, O=%s, Q=%s\n- Research class: %s\n",
		item.Domain, item.DomainOwner, renderInterfaces(item.SupportedInterfaces), renderCodeList(item.ImplementationOwners),
		strings.Join(item.OperabilitySurfaces, "; "), item.EvidenceOwner, renderCodeList(item.RequiredEvidenceGates),
		item.Status.AtCommit, item.Status.Implemented, item.Status.Reachable, item.Status.Operable, item.Status.Qualified, item.ResearchClass)
	if references := renderReferences("ADRs", item.ADRs); references != "" {
		out.WriteString(references)
	}
	if references := renderReferences("Research", item.ResearchPackets); references != "" {
		out.WriteString(references)
	}
	if references := renderReferences("Backlog", item.Backlog); references != "" {
		out.WriteString(references)
	}
	if len(item.Constraints) > 0 {
		fmt.Fprintf(&out, "- Constraints: %s\n", strings.Join(item.Constraints, "; "))
	}
	if len(item.UnsupportedBehavior) > 0 {
		fmt.Fprintf(&out, "- Unsupported: %s\n", strings.Join(item.UnsupportedBehavior, "; "))
	}
	fmt.Fprintf(&out, "<!-- capability-status:end %s -->\n\n", item.ID)
	return out.String()
}

func renderInterfaces(values []supportedInterface) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		part := "`" + value.ID + "` (" + value.Kind + ")"
		if len(value.Contracts) > 0 {
			part += ": " + renderCodeList(value.Contracts)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

func renderCodeList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, "`"+value+"`")
	}
	return strings.Join(quoted, ", ")
}

func renderReferences(label string, values []string) string {
	if len(values) == 0 {
		return ""
	}
	return "- " + label + ": " + renderCodeList(values) + "\n"
}

func checkActiveDocuments(root string, c catalogue) error {
	declared := declaredDocuments(c)
	for _, item := range c.Capabilities {
		for _, document := range item.ActiveDocuments {
			if filepath.ToSlash(document) == "docs/engineering/capability-evidence-register.md" {
				continue
			}
			raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(document)))
			if !bytes.Contains(raw, []byte(renderStatusBlock(item))) {
				return fmt.Errorf("active document %s has stale or missing capability block %s", document, item.ID)
			}
		}
	}
	return validateDocumentMarkers(root, declared, true)
}

func declaredDocuments(c catalogue) map[string]map[string]struct{} {
	declared := map[string]map[string]struct{}{}
	for _, item := range c.Capabilities {
		for _, document := range item.ActiveDocuments {
			if declared[document] == nil {
				declared[document] = map[string]struct{}{}
			}
			declared[document][item.ID] = struct{}{}
		}
	}
	return declared
}

func validateDocumentMarkers(root string, declared map[string]map[string]struct{}, requireGenerated bool) error {
	active, err := activeMarkdownPaths(root)
	if err != nil {
		return err
	}
	for _, document := range active {
		if !requireGenerated && document == "docs/engineering/capability-evidence-register.md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(document)))
		if err != nil {
			return err
		}
		scannable := regexp.MustCompile("(?s)```.*?```").ReplaceAll(raw, nil)
		tokens := regexp.MustCompile(`<!-- capability-status:(begin|end) ([a-z0-9.-]+) -->`).FindAllSubmatch(scannable, -1)
		if bytes.Count(scannable, []byte("<!-- capability-status:")) != len(tokens) {
			return fmt.Errorf("active document %s has malformed capability marker", document)
		}
		seen, open := map[string]struct{}{}, ""
		for _, token := range tokens {
			kind, id := string(token[1]), string(token[2])
			if _, ok := declared[document][id]; !ok {
				return fmt.Errorf("active document %s has unknown capability marker %s", document, id)
			}
			if kind == "begin" {
				if open != "" {
					return fmt.Errorf("active document %s has misnested capability markers", document)
				}
				if _, duplicate := seen[id]; duplicate {
					return fmt.Errorf("active document %s repeats capability marker %s", document, id)
				}
				open, seen[id] = id, struct{}{}
			} else if open != id {
				return fmt.Errorf("active document %s has misnested capability markers", document)
			} else {
				open = ""
			}
		}
		if open != "" {
			return fmt.Errorf("active document %s has unclosed capability marker %s", document, open)
		}
		required := make([]string, 0, len(declared[document]))
		for id := range declared[document] {
			required = append(required, id)
		}
		sort.Strings(required)
		for _, id := range required {
			if _, ok := seen[id]; !ok {
				return fmt.Errorf("active document %s has stale or missing capability block %s", document, id)
			}
		}
	}
	return nil
}

func activeMarkdownPaths(root string) ([]string, error) {
	paths, err := doccontract.ActiveMarkdownPaths(root)
	if err != nil {
		return nil, err
	}
	paths = append(paths, "CHANGELOG.md")
	return uniqueStrings(paths), nil
}

func activeDocumentSet(root string) (map[string]struct{}, error) {
	paths, err := activeMarkdownPaths(root)
	if err != nil {
		return nil, err
	}
	result := make(map[string]struct{}, len(paths))
	for _, value := range paths {
		result[value] = struct{}{}
	}
	return result, nil
}

func validateReadinessClaims(root string, c catalogue) error {
	allQualified := len(c.Capabilities) > 0
	for _, item := range c.Capabilities {
		allQualified = allQualified && item.Status.Qualified == "yes"
	}
	if allQualified {
		return nil
	}
	for _, guard := range []struct{ document, required string }{
		{"README.md", "stabilization candidate, not a production release"},
		{"CHANGELOG.md", "no accepted production release"},
	} {
		raw, err := os.ReadFile(filepath.Join(root, guard.document))
		if err != nil {
			return fmt.Errorf("%s readiness guard: %w", guard.document, err)
		}
		if !strings.Contains(strings.ToLower(string(raw)), guard.required) {
			return fmt.Errorf("%s readiness guard is missing %q", guard.document, guard.required)
		}
		claims := strings.ToLower(string(raw))
		claims = strings.ReplaceAll(claims, "stabilization candidate, not a production release", "")
		claims = strings.ReplaceAll(claims, "no accepted production release", "")
		if regexp.MustCompile(`\b(?:production[- ]ready|ready for production|accepted production release|is a production release|production[- ]grade)\b`).MatchString(claims) {
			return fmt.Errorf("%s readiness guard rejects production-readiness claims while capabilities remain unqualified", guard.document)
		}
	}
	return nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func prepareActiveDocuments(root string, c catalogue) (map[string][]byte, error) {
	documents := map[string][]byte{}
	for _, item := range c.Capabilities {
		for _, document := range item.ActiveDocuments {
			if filepath.ToSlash(document) == "docs/engineering/capability-evidence-register.md" {
				continue
			}
			target := filepath.Join(root, filepath.FromSlash(document))
			raw, ok := documents[document]
			if !ok {
				var err error
				raw, err = os.ReadFile(target)
				if err != nil {
					return nil, err
				}
			}
			begin := []byte("<!-- capability-status:begin " + item.ID + " -->")
			end := []byte("<!-- capability-status:end " + item.ID + " -->")
			start, finish := bytes.Index(raw, begin), bytes.Index(raw, end)
			if start < 0 || finish < start || bytes.Count(raw, begin) != 1 || bytes.Count(raw, end) != 1 {
				return nil, fmt.Errorf("active document %s requires one existing block for %s", document, item.ID)
			}
			finish += len(end)
			replaced := append(append(append([]byte{}, raw[:start]...), []byte(strings.TrimSuffix(renderStatusBlock(item), "\n\n"))...), raw[finish:]...)
			documents[document] = replaced
		}
	}
	return documents, nil
}

func checkBytes(target string, expected []byte, message string) error {
	actual, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(actual, expected) {
		return fmt.Errorf("%s: %s", message, filepath.ToSlash(target))
	}
	return nil
}

func writeOutputSet(outputs map[string][]byte, write func(string, []byte) error) error {
	type priorOutput struct {
		raw    []byte
		exists bool
	}
	targets := make([]string, 0, len(outputs))
	prior := map[string]priorOutput{}
	for target := range outputs {
		targets = append(targets, target)
		raw, err := os.ReadFile(target)
		if err == nil {
			prior[target] = priorOutput{raw: raw, exists: true}
		} else if errors.Is(err, os.ErrNotExist) {
			prior[target] = priorOutput{}
		} else {
			return err
		}
	}
	sort.Strings(targets)
	written := make([]string, 0, len(targets))
	for _, target := range targets {
		if err := write(target, outputs[target]); err != nil {
			var rollbackErrors []error
			for index := len(written) - 1; index >= 0; index-- {
				previous := prior[written[index]]
				if previous.exists {
					rollbackErrors = append(rollbackErrors, write(written[index], previous.raw))
				} else {
					rollbackErrors = append(rollbackErrors, os.Remove(written[index]))
				}
			}
			return errors.Join(append([]error{fmt.Errorf("replace generated output set: %w", err)}, rollbackErrors...)...)
		}
		written = append(written, target)
	}
	return nil
}

func atomicWrite(target string, raw []byte) (returnErr error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".capabilitycatalog-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer func() {
		if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			returnErr = errors.Join(returnErr, err)
		}
	}()
	if _, err := temp.Write(raw); err != nil {
		_ = temp.Close()
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		_ = temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, target)
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
