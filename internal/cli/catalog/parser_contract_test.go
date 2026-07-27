package catalog_test

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	authoritycmd "ardents/internal/cli/authority"
	"ardents/internal/cli/catalog"
	"ardents/internal/cli/client"
	commandctx "ardents/internal/cli/command"
	configurationcmd "ardents/internal/cli/configuration"
	contentcmd "ardents/internal/cli/content"
	diagnosticscmd "ardents/internal/cli/diagnostics"
	identitycmd "ardents/internal/cli/identity"
	networkcmd "ardents/internal/cli/network"
	nodecmd "ardents/internal/cli/node"
	"ardents/internal/cli/output"
	tuicmd "ardents/internal/cli/tui"
	workloadcmd "ardents/internal/cli/workload"
	identityaccess "ardents/internal/identity/access"
)

func TestProductionParsersAndCatalogueContainTheSameLeaves(t *testing.T) {
	requireNoParserPanics(t)
	if err := catalog.ValidateReachability(catalog.Commands(), productionParserPaths(t)); err != nil {
		t.Fatal(err)
	}
}

func requireNoParserPanics(t *testing.T) {
	t.Helper()
	probeClient, err := client.New(client.Config{
		BaseURL: "unix:///ocs01-parser-probe.sock",
		Timeout: time.Millisecond,
		Signer:  rejectingSigner{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer probeClient.Close()
	for _, spec := range catalog.Commands() {
		t.Run(spec.ID, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			command := commandctx.Context{
				Client:   probeClient,
				Input:    strings.NewReader(""),
				Timeout:  time.Millisecond,
				Interval: time.Millisecond,
				Renderer: output.NewRenderer(&stdout, &stderr, false),
			}
			args := append(append([]string(nil), spec.Path[1:]...), "--ocs-01-parser-probe")
			runProductionParser(t, spec.Path[0], command, args)
			combined := stdout.String() + stderr.String()
			if strings.Contains(combined, "unknown command") || strings.Contains(combined, "unknown subcommand") {
				t.Fatalf("registered leaf is unreachable: %s", combined)
			}
		})
	}
}

func runProductionParser(t *testing.T, group string, command commandctx.Context, args []string) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("production parser panicked: %v", recovered)
		}
	}()
	switch group {
	case "authority":
		authoritycmd.New(command).Run(context.Background(), args)
	case "node":
		nodecmd.New(command).Run(context.Background(), args)
	case "network":
		networkcmd.New(command).Run(context.Background(), args)
	case "workload":
		workloadcmd.New(command).Run(context.Background(), args)
	case "data":
		contentcmd.New(command).Run(context.Background(), args)
	case "diagnostics":
		diagnosticscmd.New(command).Run(context.Background(), args)
	case "config":
		configurationcmd.FromContext(command).Run(context.Background(), args)
	case "identity":
		identitycmd.NewOnline(command.Renderer, command.Client, time.Millisecond, command.Input).Run(context.Background(), args)
	case "shell":
		tuicmd.NewShell(command).Run(context.Background(), args)
	case "tui":
		tuicmd.New(command).Run(context.Background(), args)
	case "version":
		return
	default:
		t.Fatalf("catalogue has unknown root parser %q", group)
	}
}

type rejectingSigner struct{}

func (rejectingSigner) Principal(context.Context) (string, error) {
	return "", errors.New("parser probe signer")
}

func (rejectingSigner) Credential(context.Context) (*identityaccess.Artifact, error) {
	return nil, errors.New("parser probe signer")
}

func (rejectingSigner) SignAuthenticationChallenge(context.Context, identityaccess.Challenge) ([]byte, error) {
	return nil, errors.New("parser probe signer")
}

func productionParserPaths(t *testing.T) []string {
	t.Helper()
	var paths []string
	addFlat := func(prefix, file, function string) {
		for _, leaf := range switchTokens(t, file, function) {
			paths = append(paths, prefix+" "+leaf)
		}
	}
	addFlat("node", filepath.Join("..", "node", "command.go"), "Run")
	addParserTree(t, &paths, "network", filepath.Join("..", "network"), "command.go", "Run", map[string]parserBranch{
		"resolve": {file: "records.go", function: "resolve"},
		"records": {file: "records.go", function: "records"},
	})
	addFlat("workload", filepath.Join("..", "workload", "command.go"), "Run")
	addParserTree(t, &paths, "data", filepath.Join("..", "content"), "command_data.go", "Run", map[string]parserBranch{
		"objects":   {file: "command_data_catalog.go", function: "run"},
		"blobs":     {file: "command_data_blobs.go", function: "dataBlobs"},
		"manifests": {file: "command_data_catalog.go", function: "run"},
		"transfers": {file: "command_data_transfers.go", function: "dataTransfers"},
	})
	addFlat("diagnostics", filepath.Join("..", "diagnostics", "command.go"), "Run")
	addFlat("config", filepath.Join("..", "configuration", "command.go"), "Run")
	addFlat("authority", filepath.Join("..", "authority", "command.go"), "Run")
	addParserTree(t, &paths, "identity", filepath.Join("..", "identity"), "command.go", "Run", map[string]parserBranch{
		"principal":          {file: "command.go", function: "runPrincipal"},
		"device":             {file: "command.go", function: "runDevice"},
		"grant":              {file: "administration.go", function: "runGrant"},
		"delegation":         {file: "delegation.go", function: "runDelegation"},
		"application-ticket": {file: "administration.go", function: "runApplicationTicket", comparisons: true},
	})
	paths = append(paths, "shell", "tui", "version")
	return paths
}

type parserBranch struct {
	file        string
	function    string
	comparisons bool
}

func addParserTree(t *testing.T, paths *[]string, prefix, directory, file, function string, nested map[string]parserBranch) {
	t.Helper()
	for _, tokenValue := range switchTokens(t, filepath.Join(directory, file), function) {
		branch, ok := nested[tokenValue]
		if !ok {
			*paths = append(*paths, prefix+" "+tokenValue)
			continue
		}
		var leaves []string
		if branch.comparisons {
			leaves = comparisonTokens(t, filepath.Join(directory, branch.file), branch.function)
		} else {
			leaves = switchTokens(t, filepath.Join(directory, branch.file), branch.function)
		}
		for _, leaf := range leaves {
			*paths = append(*paths, prefix+" "+tokenValue+" "+leaf)
		}
	}
}

func switchTokens(t *testing.T, path, function string) []string {
	t.Helper()
	return syntaxTokens(t, path, function, func(node ast.Node) (string, bool) {
		clause, ok := node.(*ast.CaseClause)
		if !ok || len(clause.List) != 1 {
			return "", false
		}
		literal, ok := clause.List[0].(*ast.BasicLit)
		return stringLiteral(literal, ok)
	})
}

func comparisonTokens(t *testing.T, path, function string) []string {
	t.Helper()
	return syntaxTokens(t, path, function, func(node ast.Node) (string, bool) {
		expression, ok := node.(*ast.BinaryExpr)
		if !ok {
			return "", false
		}
		if literal, ok := expression.Y.(*ast.BasicLit); ok && isArgsZero(expression.X) {
			return stringLiteral(literal, true)
		}
		if literal, ok := expression.X.(*ast.BasicLit); ok && isArgsZero(expression.Y) {
			return stringLiteral(literal, true)
		}
		return "", false
	})
}

func syntaxTokens(t *testing.T, path, function string, extract func(ast.Node) (string, bool)) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var result []string
	for _, declaration := range parsed.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Name.Name != function {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if value, ok := extract(node); ok && value != "help" {
				result = append(result, value)
			}
			return true
		})
	}
	if len(result) == 0 {
		t.Fatalf("%s: parser function %s exposes no command tokens", path, function)
	}
	return unique(result)
}

func isArgsZero(expression ast.Expr) bool {
	index, ok := expression.(*ast.IndexExpr)
	if !ok {
		return false
	}
	name, ok := index.X.(*ast.Ident)
	if !ok || name.Name != "args" {
		return false
	}
	literal, ok := index.Index.(*ast.BasicLit)
	return ok && literal.Value == "0"
}

func stringLiteral(literal *ast.BasicLit, ok bool) (string, bool) {
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
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
