package doccontract

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var activeDocuments = []string{
	"README.md",
	"docs/product/distribution-model.md",
	"docs/protocols/communication-contracts.md",
	"docs/operations/operator-runbook.md",
}

var legacyOperatorDirectives = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\btoken-authenticated\s+loopback\b`),
	regexp.MustCompile(`(?i)\b(?:create|creates|creating)\s+an\s+API\s+token\b`),
	regexp.MustCompile(`(?i)\bdaemon(?:'s|’s)\s+loopback\s+control\s+API\b`),
	regexp.MustCompile(`(?i)\bAPI\s+and\s+observability\s+token\b`),
	regexp.MustCompile(`(?i)\bserver-side\s+tokens?\b`),
	regexp.MustCompile(`(?i)\bloopback-only\s+authenticated\s+operator\s+API\b`),
	regexp.MustCompile(`(?i)--(?:api-)?token(?:-file)?\b`),
	regexp.MustCompile(`(?i)--addr\s+https?://`),
}

var (
	localLinkPattern       = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	scriptPattern          = regexp.MustCompile(`(?:^|\s)(\.{1,2}/[A-Za-z0-9_./-]+\.(?:ps1|sh))(?:\s|$)`)
	inlineCodePattern      = regexp.MustCompile("`([^`\\r\\n]+)`")
	legacyActionPattern    = regexp.MustCompile(`(?i)\b(?:accept|authenticate|bind|configure|connect|create|expose|provide|reach|set|supply|use)\w*\b`)
	legacyBoundaryPattern  = regexp.MustCompile(`(?i)\b(?:operator|control)\b`)
	legacyMechanismPattern = regexp.MustCompile(
		`(?i)\b(?:bearer|localhost|loopback|tokens?)\b`,
	)
	negationPattern = regexp.MustCompile(`(?i)\b(?:cannot|delete|does\s+not|forbidden|legacy|migrat\w*|never|no|not|obsolete|reject(?:ed|s)?|remov\w*|replac\w*|retir\w*|superseded|unsupported|without)\b`)
)

var commandGrammar = map[string]map[string]struct{}{
	"config":      words("show", "reload"),
	"data":        words("inventory", "objects", "blobs", "manifests", "transfers"),
	"diagnostics": words("snapshot", "health", "pending", "explain", "events"),
	"identity":    words("principal", "device", "enroll", "grant", "delegation", "application-ticket", "login", "status", "logout"),
	"network":     words("status", "discovery", "presence", "peers", "routes", "resolve", "records"),
	"node":        words("start", "stop", "status", "runtime", "features", "events"),
	"shell":       nil,
	"tui":         nil,
	"version":     nil,
	"workload":    words("list", "get", "register", "start", "stop", "restart", "services", "service", "publication"),
}

var identityCommandGrammar = map[string]map[string]struct{}{
	"application-ticket": words("issue"),
	"delegation":         words("issue", "revoke", "import-revocation"),
	"device":             words("create", "show", "revoke"),
	"grant":              words("list", "issue", "revoke"),
	"principal":          words("create", "import", "show"),
}

// Validate checks the active entry, distribution, protocol, and runbook
// documentation against the Principal-only Operator Interface contract.
func Validate(root string) error {
	var violations []error

	for _, relativePath := range activeDocuments {
		path := filepath.Join(root, filepath.FromSlash(relativePath))
		if _, err := os.Stat(path); err != nil {
			violations = append(violations, fmt.Errorf("%s: required active document: %w", relativePath, err))
		}
	}

	activePaths, err := activeMarkdownPaths(root)
	if err != nil {
		violations = append(violations, err)
	} else {
		for _, relativePath := range activePaths {
			contents, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
			if readErr != nil {
				violations = append(violations, fmt.Errorf("%s: read: %w", relativePath, readErr))
				continue
			}
			text := string(contents)
			violations = append(violations, findLegacyDirectives(relativePath, text)...)
			violations = append(violations, findBrokenLinks(root, relativePath, text)...)
			violations = append(violations, findCommandProblems(root, relativePath, text)...)
		}
	}

	return errors.Join(violations...)
}

func activeMarkdownPaths(root string) ([]string, error) {
	paths := []string{"README.md"}
	for _, relativeRoot := range []string{"docs", "deploy"} {
		walkRoot := filepath.Join(root, relativeRoot)
		err := filepath.WalkDir(walkRoot, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			relativePath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relativePath = filepath.ToSlash(relativePath)
			if entry.IsDir() && relativePath == "docs/audit" {
				return filepath.SkipDir
			}
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
				paths = append(paths, relativePath)
			}
			return nil
		})
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("scan active documentation under %s: %w", relativeRoot, err)
		}
	}
	return paths, nil
}

func findLegacyDirectives(relativePath string, text string) []error {
	var violations []error
	for _, paragraph := range regexp.MustCompile(`(?:\r?\n){2,}`).Split(text, -1) {
		normalized := strings.Join(strings.Fields(paragraph), " ")
		if negationPattern.MatchString(normalized) {
			continue
		}

		for _, pattern := range legacyOperatorDirectives {
			if match := pattern.FindString(normalized); match != "" {
				violations = append(
					violations,
					fmt.Errorf("%s: legacy operator directive %q", relativePath, match),
				)
			}
		}
	}
	for _, line := range strings.Split(text, "\n") {
		for _, sentence := range regexp.MustCompile(`[.!?](?:\s|$)`).Split(line, -1) {
			normalized := strings.Join(strings.Fields(sentence), " ")
			if negationPattern.MatchString(normalized) {
				continue
			}
			if legacyActionPattern.MatchString(normalized) &&
				legacyBoundaryPattern.MatchString(normalized) &&
				legacyMechanismPattern.MatchString(normalized) {
				violations = append(
					violations,
					fmt.Errorf("%s: legacy operator directive %q", relativePath, normalized),
				)
			}
		}
	}
	return violations
}

func findBrokenLinks(root string, relativePath string, text string) []error {
	var violations []error
	for _, match := range localLinkPattern.FindAllStringSubmatch(text, -1) {
		target := strings.TrimSpace(strings.SplitN(match[1], "#", 2)[0])
		if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
			continue
		}

		target = strings.Trim(target, "<>")
		path := filepath.Join(root, filepath.Dir(filepath.FromSlash(relativePath)), filepath.FromSlash(target))
		if _, err := os.Stat(path); err != nil {
			violations = append(violations, fmt.Errorf("%s: local link %s: %w", relativePath, target, err))
		}
	}
	return violations
}

func findCommandProblems(root string, relativePath string, text string) []error {
	var violations []error
	inFence := false
	var fence strings.Builder
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				violations = append(violations, validateCommandSnippet(root, relativePath, fence.String())...)
				fence.Reset()
			}
			inFence = !inFence
			continue
		}
		if inFence {
			fence.WriteString(line)
			fence.WriteByte('\n')
		}
	}
	for _, match := range inlineCodePattern.FindAllStringSubmatch(text, -1) {
		violations = append(violations, validateCommandSnippet(root, relativePath, match[1])...)
	}
	return violations
}

func validateCommandSnippet(root string, relativePath string, snippet string) []error {
	snippet = strings.ReplaceAll(snippet, "\\\r\n", " ")
	snippet = strings.ReplaceAll(snippet, "\\\n", " ")
	snippet = strings.ReplaceAll(snippet, "`\r\n", " ")
	snippet = strings.ReplaceAll(snippet, "`\n", " ")

	var violations []error
	for _, line := range strings.Split(snippet, "\n") {
		for _, match := range scriptPattern.FindAllStringSubmatch(line, -1) {
			target := strings.TrimPrefix(match[1], "./")
			path := filepath.Join(root, filepath.FromSlash(target))
			if _, err := os.Stat(path); err != nil {
				violations = append(violations, fmt.Errorf("%s: command script %s: %w", relativePath, match[1], err))
			}
		}

		tokens := strings.Fields(strings.TrimSpace(line))
		ctlIndex := indexToken(tokens, "ardentsctl")
		if ctlIndex < 0 {
			continue
		}
		commandIndex := documentedRootCommandIndex(tokens, ctlIndex+1)
		if commandIndex < 0 {
			continue
		}

		command := cleanToken(tokens[commandIndex])
		subcommands, known := commandGrammar[command]
		if !known {
			violations = append(violations, fmt.Errorf("%s: unknown ardentsctl command %q", relativePath, command))
			continue
		}
		if subcommands == nil {
			continue
		}
		subcommandIndex := nextCommandToken(tokens, commandIndex+1)
		if subcommandIndex < 0 {
			continue
		}
		subcommand := cleanToken(tokens[subcommandIndex])
		if _, ok := subcommands[subcommand]; !ok {
			violations = append(violations, fmt.Errorf("%s: unknown ardentsctl %s subcommand %q", relativePath, command, subcommand))
			continue
		}
		if command == "identity" {
			nested := identityCommandGrammar[subcommand]
			if nested == nil {
				continue
			}
			nestedIndex := nextCommandToken(tokens, subcommandIndex+1)
			if nestedIndex < 0 {
				continue
			}
			nestedCommand := cleanToken(tokens[nestedIndex])
			if _, ok := nested[nestedCommand]; !ok {
				violations = append(violations, fmt.Errorf("%s: unknown ardentsctl identity %s subcommand %q", relativePath, subcommand, nestedCommand))
			}
		}
	}
	return violations
}

func words(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func indexToken(tokens []string, target string) int {
	for index, token := range tokens {
		if cleanToken(token) == target {
			return index
		}
	}
	return -1
}

func documentedRootCommandIndex(tokens []string, start int) int {
	for index := start; index < len(tokens); index++ {
		token := cleanToken(tokens[index])
		if token == "" || token == "..." {
			continue
		}
		if strings.HasPrefix(token, "-") {
			if !strings.Contains(token, "=") && globalFlagTakesValue(token) && index+1 < len(tokens) {
				index++
			}
			continue
		}
		return index
	}
	return -1
}

func globalFlagTakesValue(flag string) bool {
	switch flag {
	case "--addr", "--node-name", "--output", "--principal", "--signer-file",
		"--ssh", "--ssh-identity", "--ssh-known-hosts", "--ssh-operator-socket",
		"--ssh-port", "--timeout":
		return true
	default:
		return false
	}
}

func nextCommandToken(tokens []string, start int) int {
	for index := start; index < len(tokens); index++ {
		token := cleanToken(tokens[index])
		if token == "" || token == "..." || strings.HasPrefix(token, "-") {
			continue
		}
		return index
	}
	return -1
}

func cleanToken(token string) string {
	return strings.Trim(token, "`'\"(),.:;\\")
}
