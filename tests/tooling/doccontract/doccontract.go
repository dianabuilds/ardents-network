package doccontract

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ardents/internal/cli/catalog"
)

var requiredContractDocuments = []string{
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
	negatedActionPrefixPattern  = regexp.MustCompile(`(?i)(?:do(?:es)?\s+not|must\s+not|never)\s*$`)
	rejectedLegacyPrefixPattern = regexp.MustCompile(
		`(?i)(?:(?:do(?:es)?\s+not|must\s+not|never)\s+(?:accept|authenticate|bind|configure|connect|create|expose|provide|reach|set|supply|use)\w*|delete|migrat\w*|reject(?:ed|s)?|remov\w*|replac\w*|retir\w*)\s+(?:the\s+)?$`,
	)
	rejectedLegacySuffixPattern       = regexp.MustCompile(`(?i)^\s+(?:(?:control\s+API|surface)\s+)?(?:is|are)\s+(?:forbidden|rejected|unsupported)\b`)
	remediationBeforeMechanismPattern = regexp.MustCompile(`(?i)\b(?:delete|migrat\w*|remov\w*|replac\w*|retir\w*)\b`)
	separateObservabilityPattern      = regexp.MustCompile(`(?i)\bscrape\s+token\b.*\bis\s+not\s+the\s+control\s+API\b`)
)

var (
	goCommands            = words("build", "clean", "env", "fmt", "generate", "install", "list", "mod", "run", "test", "tool", "version", "vet")
	dockerComposeCommands = words(
		"build", "config", "create", "down", "exec", "images", "logs", "ps",
		"pull", "restart", "rm", "run", "start", "stop", "up",
	)
)

// Validate checks the active entry, distribution, protocol, and runbook
// documentation against the Principal-only Operator Interface contract.
func Validate(root string) error {
	var violations []error

	for _, relativePath := range requiredContractDocuments {
		path := filepath.Join(root, filepath.FromSlash(relativePath))
		if _, err := os.Stat(path); err != nil {
			violations = append(violations, fmt.Errorf("%s: required active document: %w", relativePath, err))
		}
	}

	activePaths, err := ActiveMarkdownPaths(root)
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

// ActiveMarkdownPaths returns the repository documents governed as current
// product and operator documentation. Historical audit documents are excluded.
func ActiveMarkdownPaths(root string) ([]string, error) {
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
		for _, sentence := range regexp.MustCompile(`[.!?](?:\s|$)`).Split(normalized, -1) {
			for _, pattern := range legacyOperatorDirectives {
				for _, location := range pattern.FindAllStringIndex(sentence, -1) {
					if legacyMatchIsExplicitlyRejected(sentence, location) {
						continue
					}
					violations = append(
						violations,
						fmt.Errorf("%s: legacy operator directive %q", relativePath, sentence[location[0]:location[1]]),
					)
				}
			}
		}
	}
	for _, line := range strings.Split(text, "\n") {
		for _, sentence := range regexp.MustCompile(`[.!?](?:\s|$)`).Split(line, -1) {
			normalized := strings.Join(strings.Fields(sentence), " ")
			if !legacyBoundaryPattern.MatchString(normalized) || !legacyMechanismPattern.MatchString(normalized) {
				continue
			}
			if separateObservabilityPattern.MatchString(normalized) {
				continue
			}
			for _, action := range legacyActionPattern.FindAllStringIndex(normalized, -1) {
				if negatedActionPrefixPattern.MatchString(normalized[:action[0]]) {
					continue
				}
				afterAction := normalized[action[1]:]
				mechanism := legacyMechanismPattern.FindStringIndex(afterAction)
				if mechanism == nil {
					continue
				}
				if remediationBeforeMechanismPattern.MatchString(afterAction[:mechanism[0]]) {
					continue
				}
				violations = append(violations, fmt.Errorf("%s: legacy operator directive %q", relativePath, normalized))
				break
			}
		}
	}
	return violations
}

func legacyMatchIsExplicitlyRejected(sentence string, location []int) bool {
	return rejectedLegacyPrefixPattern.MatchString(sentence[:location[0]]) ||
		rejectedLegacySuffixPattern.MatchString(sentence[location[1]:])
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
		if ctlIndex >= 0 {
			violations = append(violations, validateArdentsctlCommand(relativePath, line, tokens, ctlIndex)...)
		}

		goIndex := indexToken(tokens, "go")
		if goIndex >= 0 && goIndex+1 < len(tokens) {
			command := cleanToken(tokens[goIndex+1])
			if _, ok := goCommands[command]; !ok {
				violations = append(violations, fmt.Errorf("%s: unknown documented command %q", relativePath, "go "+command))
			}
		}

		dockerIndex := indexToken(tokens, "docker")
		if dockerIndex >= 0 && dockerIndex+1 < len(tokens) && cleanToken(tokens[dockerIndex+1]) == "compose" {
			commandIndex := dockerComposeCommandIndex(tokens, dockerIndex+2)
			if commandIndex >= 0 {
				command := cleanToken(tokens[commandIndex])
				if _, ok := dockerComposeCommands[command]; !ok {
					violations = append(violations, fmt.Errorf("%s: unknown documented command %q", relativePath, "docker compose "+command))
				}
			}
		}
	}
	return violations
}

func validateArdentsctlCommand(relativePath string, line string, tokens []string, ctlIndex int) []error {
	commandIndex := documentedRootCommandIndex(tokens, ctlIndex+1)
	if commandIndex < 0 {
		return nil
	}

	command := cleanToken(tokens[commandIndex])
	if command == "help" {
		return nil
	}
	args := make([]string, 0, len(tokens)-commandIndex)
	for _, token := range tokens[commandIndex:] {
		args = append(args, cleanToken(token))
	}
	if _, known := catalog.Match(args); known {
		return nil
	}
	if args[len(args)-1] == "help" && len(catalog.Under(args[:len(args)-1])) > 0 {
		return nil
	}
	if len(catalog.Under(args)) > 0 {
		return nil
	}
	if len(catalog.Under([]string{command})) == 0 {
		return []error{fmt.Errorf("%s: unknown ardentsctl command %q", relativePath, command)}
	}
	return []error{fmt.Errorf("%s: unknown ardentsctl command path in %q", relativePath, strings.TrimSpace(line))}
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
			if !strings.Contains(token, "=") && index+1 < len(tokens) {
				index++
			}
			continue
		}
		return index
	}
	return -1
}

func dockerComposeCommandIndex(tokens []string, start int) int {
	for index := start; index < len(tokens); index++ {
		token := cleanToken(tokens[index])
		if strings.HasPrefix(token, "-") {
			if !strings.Contains(token, "=") && index+1 < len(tokens) {
				index++
			}
			continue
		}
		return index
	}
	return -1
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
