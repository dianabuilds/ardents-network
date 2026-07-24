package archaccept

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const manifestPath = "docs/engineering/architecture-acceptance.json"
const requiredDefaultFileBudget = 12
const canonicalPrivateProto = "api/ardents/private/v1/private.proto"
const canonicalPrivateGenerated = "internal/messaging/protocol/private.pb.go"
const canonicalPrivateGoPackage = "ardents/internal/messaging/protocol;messagingprotocol"
const canonicalPrivateGenerator = "scripts/generate-api.ps1"

var legacyPrivateProtocolPaths = []string{
	"internal/messaging/private.proto",
	"internal/messaging/private.pb.go",
}

var requiredProductionRoots = []string{"api", "cmd", "internal", "scripts", "sdk/go"}

type Manifest struct {
	Version              int                        `json:"version"`
	ProductionRoots      []string                   `json:"production_roots"`
	FileBudget           FileBudgetPolicy           `json:"file_budget"`
	PackageDocumentation PackageDocumentationPolicy `json:"package_documentation"`
	Services             ServicePolicy              `json:"services"`
	AgentTooling         AgentToolingPolicy         `json:"agent_tooling"`
	PrivateProtocol      PrivateProtocolPolicy      `json:"private_protocol"`
}

type FileBudgetPolicy struct {
	DefaultMax int                        `json:"default_max"`
	Exceptions map[string]BudgetException `json:"exceptions"`
}

type BudgetException struct {
	Max    int    `json:"max"`
	Reason string `json:"reason"`
	Owner  string `json:"owner"`
}

type PackageDocumentationPolicy struct {
	Grandfathered map[string]string `json:"grandfathered_without_package_comment"`
}

type ServicePolicy struct {
	Operator    ServiceSurface `json:"operator"`
	Application ServiceSurface `json:"application"`
}

type ServiceSurface struct {
	ProtoRoots  []string                     `json:"proto_roots"`
	Composition map[string]CompositionTarget `json:"composition"`
}

type CompositionTarget struct {
	Path     string `json:"path"`
	Function string `json:"function"`
	Mode     string `json:"mode"`
}

type AgentToolingPolicy struct {
	Root            string   `json:"root"`
	AllowedPrefixes []string `json:"allowed_prefixes"`
}

type PrivateProtocolPolicy struct {
	Proto     string `json:"proto"`
	Generated string `json:"generated"`
	GoPackage string `json:"go_package"`
	Generator string `json:"generator"`
	Owner     string `json:"owner"`
}

type packageFacts struct {
	HandwrittenFiles int
	PackageName      string
	PackageDocs      []string
}

// Validate checks the repository tree against its machine-readable
// architecture acceptance policy.
func Validate(root string) error {
	manifest, err := loadManifest(root)
	if err != nil {
		return err
	}

	var violations []error
	packages, err := inspectPackages(root, manifest.ProductionRoots)
	if err != nil {
		violations = append(violations, err)
	} else {
		violations = append(violations, validateFileBudget(manifest.FileBudget, packages)...)
		violations = append(violations, validatePackageDocumentation(manifest.PackageDocumentation, packages)...)
	}
	violations = append(violations, validateServiceSurface(root, "operator", manifest.Services.Operator)...)
	violations = append(violations, validateServiceSurface(root, "application", manifest.Services.Application)...)
	violations = append(violations, validateAgentTooling(root, manifest.AgentTooling)...)
	violations = append(violations, validatePrivateProtocol(root, manifest.PrivateProtocol)...)
	return errors.Join(violations...)
}

func loadManifest(root string) (Manifest, error) {
	path := filepath.Join(root, filepath.FromSlash(manifestPath))
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("architecture acceptance manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("architecture acceptance manifest: decode: %w", err)
	}
	if manifest.Version != 1 {
		return Manifest{}, fmt.Errorf("architecture acceptance manifest: unsupported version %d", manifest.Version)
	}
	if manifest.FileBudget.DefaultMax != requiredDefaultFileBudget {
		return Manifest{}, fmt.Errorf(
			"architecture acceptance manifest: file budget default_max must be %d",
			requiredDefaultFileBudget,
		)
	}
	if len(manifest.ProductionRoots) == 0 {
		return Manifest{}, errors.New("architecture acceptance manifest: production_roots must not be empty")
	}
	declaredRoots := append([]string(nil), manifest.ProductionRoots...)
	slices.Sort(declaredRoots)
	if !slices.Equal(declaredRoots, requiredProductionRoots) {
		return Manifest{}, fmt.Errorf(
			"architecture acceptance manifest: production_roots must be exactly %s",
			strings.Join(requiredProductionRoots, ", "),
		)
	}
	return manifest, nil
}

func inspectPackages(root string, productionRoots []string) (map[string]packageFacts, error) {
	packages := make(map[string]packageFacts)
	seenRoots := make(map[string]struct{}, len(productionRoots))
	for _, productionRoot := range productionRoots {
		cleanRoot := filepath.ToSlash(filepath.Clean(filepath.FromSlash(productionRoot)))
		if cleanRoot == "." || strings.HasPrefix(cleanRoot, "../") || filepath.IsAbs(productionRoot) {
			return nil, fmt.Errorf("architecture acceptance: invalid production root %q", productionRoot)
		}
		if _, duplicate := seenRoots[cleanRoot]; duplicate {
			return nil, fmt.Errorf("architecture acceptance: duplicate production root %q", productionRoot)
		}
		seenRoots[cleanRoot] = struct{}{}
		scanRoot := filepath.Join(root, filepath.FromSlash(cleanRoot))
		err := filepath.WalkDir(scanRoot, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !isHandwrittenProductionGoFile(entry.Name()) {
				return nil
			}

			relativeDirectory, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			relativeDirectory = filepath.ToSlash(relativeDirectory)
			facts := packages[relativeDirectory]
			facts.HandwrittenFiles++
			parsed, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments|parser.PackageClauseOnly)
			if parseErr != nil {
				return fmt.Errorf("architecture acceptance: parse %s: %w", filepath.ToSlash(path), parseErr)
			}
			if facts.PackageName == "" {
				facts.PackageName = parsed.Name.Name
			} else if facts.PackageName != parsed.Name.Name {
				return fmt.Errorf(
					"architecture acceptance: package %s mixes names %s and %s",
					relativeDirectory,
					facts.PackageName,
					parsed.Name.Name,
				)
			}
			if parsed.Doc != nil {
				facts.PackageDocs = append(facts.PackageDocs, strings.TrimSpace(parsed.Doc.Text()))
			}
			packages[relativeDirectory] = facts
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("architecture acceptance: inspect production root %s: %w", cleanRoot, err)
		}
	}
	return packages, nil
}

func isHandwrittenProductionGoFile(name string) bool {
	return strings.HasSuffix(name, ".go") &&
		!strings.HasSuffix(name, "_test.go") &&
		!strings.HasSuffix(name, ".pb.go") &&
		!strings.HasSuffix(name, ".connect.go")
}

func hasPackageDocumentation(facts packageFacts) bool {
	for _, packageDoc := range facts.PackageDocs {
		doc := strings.TrimSpace(packageDoc)
		prefix := "Package " + facts.PackageName + " "
		if !strings.HasPrefix(doc, prefix) {
			continue
		}
		lower := strings.ToLower(doc)
		boundary := firstBoundaryIndex(lower, []string{"does not ", "do not ", "without owning"})
		if boundary < 0 {
			continue
		}
		responsibility := strings.TrimSpace(doc[len(prefix):boundary])
		if len(strings.Fields(responsibility)) >= 2 {
			return true
		}
	}
	return false
}

func firstBoundaryIndex(text string, markers []string) int {
	first := -1
	for _, marker := range markers {
		index := strings.Index(text, marker)
		if index >= 0 && (first < 0 || index < first) {
			first = index
		}
	}
	return first
}

func validateFileBudget(policy FileBudgetPolicy, packages map[string]packageFacts) []error {
	var violations []error
	for path, facts := range packages {
		limit := policy.DefaultMax
		if exception, ok := policy.Exceptions[path]; ok {
			if exception.Max < 1 || strings.TrimSpace(exception.Reason) == "" || strings.TrimSpace(exception.Owner) == "" {
				violations = append(violations, fmt.Errorf("file budget exception %s is incomplete", path))
				continue
			}
			if facts.HandwrittenFiles <= policy.DefaultMax {
				violations = append(violations, fmt.Errorf("file budget exception %s is unnecessary at %d files", path, facts.HandwrittenFiles))
			}
			if exception.Max != facts.HandwrittenFiles {
				violations = append(violations, fmt.Errorf(
					"file budget exception %s must use exact ceiling %d, got %d",
					path,
					facts.HandwrittenFiles,
					exception.Max,
				))
			}
			limit = exception.Max
		}
		if facts.HandwrittenFiles > limit {
			violations = append(violations, fmt.Errorf(
				"file budget %s: %d handwritten production files exceeds %d",
				path,
				facts.HandwrittenFiles,
				limit,
			))
		}
	}
	for path := range policy.Exceptions {
		if _, ok := packages[path]; !ok {
			violations = append(violations, fmt.Errorf("file budget exception %s references no handwritten package", path))
		}
	}
	return violations
}

func validatePackageDocumentation(policy PackageDocumentationPolicy, packages map[string]packageFacts) []error {
	var violations []error
	for path, facts := range packages {
		reason, grandfathered := policy.Grandfathered[path]
		hasPackageDoc := hasPackageDocumentation(facts)
		if !hasPackageDoc && (!grandfathered || strings.TrimSpace(reason) == "") {
			violations = append(violations, fmt.Errorf(
				"package documentation %s: canonical package comment with responsibility and explicit non-responsibility is missing",
				path,
			))
		}
		if hasPackageDoc && grandfathered {
			violations = append(violations, fmt.Errorf("package documentation %s: stale grandfathered exception", path))
		}
	}
	for path := range policy.Grandfathered {
		if _, ok := packages[path]; !ok {
			violations = append(violations, fmt.Errorf("package documentation %s: exception references no handwritten package", path))
		}
	}
	return violations
}

var servicePattern = regexp.MustCompile(`(?m)^\s*service\s+([A-Za-z][A-Za-z0-9_]*)\s*\{`)

func validateServiceSurface(root string, name string, surface ServiceSurface) []error {
	var violations []error
	discovered := make(map[string]struct{})
	for _, protoRoot := range surface.ProtoRoots {
		path := filepath.Join(root, filepath.FromSlash(protoRoot))
		info, err := os.Stat(path)
		if err != nil {
			violations = append(violations, fmt.Errorf("service contract %s: proto root %s: %w", name, protoRoot, err))
			continue
		}
		if !info.IsDir() {
			services, readErr := servicesInProto(path)
			if readErr != nil {
				violations = append(violations, fmt.Errorf("service contract %s: %w", name, readErr))
				continue
			}
			for _, service := range services {
				discovered[service] = struct{}{}
			}
			continue
		}
		walkErr := filepath.WalkDir(path, func(protoPath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
				return nil
			}
			services, err := servicesInProto(protoPath)
			if err != nil {
				return err
			}
			for _, service := range services {
				discovered[service] = struct{}{}
			}
			return nil
		})
		if walkErr != nil {
			violations = append(violations, fmt.Errorf("service contract %s: scan %s: %w", name, protoRoot, walkErr))
		}
	}

	for service := range discovered {
		target, ok := surface.Composition[service]
		if !ok {
			violations = append(violations, fmt.Errorf("service contract %s: undeclared service %s", name, service))
			continue
		}
		compositionFile := filepath.Join(root, filepath.FromSlash(target.Path))
		registered, err := hasServiceRegistration(compositionFile, target, "New"+service+"Handler")
		if err != nil {
			violations = append(violations, fmt.Errorf("service contract %s: composition %s for %s: %w", name, target.Path, service, err))
			continue
		}
		if !registered {
			violations = append(violations, fmt.Errorf("service contract %s: composition %s does not register %s", name, target.Path, service))
		}
	}
	for service := range surface.Composition {
		if _, ok := discovered[service]; !ok {
			violations = append(violations, fmt.Errorf("service contract %s: stale composition entry %s", name, service))
		}
	}
	return violations
}

func hasServiceRegistration(path string, target CompositionTarget, constructorName string) (bool, error) {
	if strings.TrimSpace(target.Path) == "" || strings.TrimSpace(target.Function) == "" {
		return false, errors.New("composition target path and function are required")
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return false, fmt.Errorf("parse Go composition root: %w", err)
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != target.Function || function.Body == nil {
			continue
		}
		switch target.Mode {
		case "register_argument":
			return hasNestedRegistration(function.Body, constructorName), nil
		case "returned_handler":
			return hasReturnedHandler(function.Body, constructorName), nil
		default:
			return false, fmt.Errorf("unsupported composition mode %q", target.Mode)
		}
	}
	return false, nil
}

func hasNestedRegistration(body *ast.BlockStmt, constructorName string) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		outer, ok := node.(*ast.CallExpr)
		if !ok || callName(outer.Fun) != "register" {
			return !found
		}
		for _, argument := range outer.Args {
			inner, ok := argument.(*ast.CallExpr)
			if ok && callName(inner.Fun) == constructorName {
				found = true
				return false
			}
		}
		return !found
	})
	return found
}

func hasReturnedHandler(body *ast.BlockStmt, constructorName string) bool {
	var assigned []string
	ast.Inspect(body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Rhs) != 1 {
			return true
		}
		call, ok := assignment.Rhs[0].(*ast.CallExpr)
		if !ok || callName(call.Fun) != constructorName {
			return true
		}
		assigned = assigned[:0]
		for _, expression := range assignment.Lhs {
			if identifier, ok := expression.(*ast.Ident); ok {
				assigned = append(assigned, identifier.Name)
			}
		}
		return true
	})
	if len(assigned) < 2 {
		return false
	}
	returned := make(map[string]struct{}, len(assigned))
	ast.Inspect(body, func(node ast.Node) bool {
		statement, ok := node.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for _, result := range statement.Results {
			if identifier, ok := result.(*ast.Ident); ok {
				returned[identifier.Name] = struct{}{}
			}
		}
		return true
	})
	for _, identifier := range assigned[:2] {
		if _, ok := returned[identifier]; !ok {
			return false
		}
	}
	return true
}

func callName(expression ast.Expr) string {
	switch function := expression.(type) {
	case *ast.Ident:
		return function.Name
	case *ast.SelectorExpr:
		return function.Sel.Name
	default:
		return ""
	}
}

func servicesInProto(path string) ([]string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read proto %s: %w", filepath.ToSlash(path), err)
	}
	matches := servicePattern.FindAllSubmatch(contents, -1)
	services := make([]string, 0, len(matches))
	for _, match := range matches {
		services = append(services, string(match[1]))
	}
	slices.Sort(services)
	return services, nil
}

func validateAgentTooling(root string, policy AgentToolingPolicy) []error {
	if policy.Root == "" || len(policy.AllowedPrefixes) == 0 {
		return []error{errors.New("agent tooling policy is incomplete")}
	}
	agentRoot := filepath.Join(root, filepath.FromSlash(policy.Root))
	var violations []error
	err := filepath.WalkDir(agentRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		allowed := false
		for _, prefix := range policy.AllowedPrefixes {
			if strings.HasPrefix(relativePath, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			violations = append(violations, fmt.Errorf("agent tooling %s is outside the allowlist", relativePath))
		}
		return nil
	})
	if err != nil {
		violations = append(violations, fmt.Errorf("agent tooling root %s: %w", policy.Root, err))
	}
	return violations
}

func validatePrivateProtocol(root string, policy PrivateProtocolPolicy) []error {
	var violations []error
	if policy.Owner != "ARD-028" {
		violations = append(violations, errors.New("private protocol owner must be ARD-028"))
	}
	if policy.Proto != canonicalPrivateProto ||
		policy.Generated != canonicalPrivateGenerated ||
		policy.GoPackage != canonicalPrivateGoPackage ||
		policy.Generator != canonicalPrivateGenerator {
		violations = append(violations, fmt.Errorf(
			"private protocol legacy location is forbidden; canonical boundary is %s -> %s with go_package %s generated by %s",
			canonicalPrivateProto,
			canonicalPrivateGenerated,
			canonicalPrivateGoPackage,
			canonicalPrivateGenerator,
		))
	}
	for _, legacyPath := range legacyPrivateProtocolPaths {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(legacyPath))); err == nil {
			violations = append(violations, fmt.Errorf("private protocol legacy location %s must be absent", legacyPath))
		} else if !errors.Is(err, os.ErrNotExist) {
			violations = append(violations, fmt.Errorf("private protocol inspect legacy location %s: %w", legacyPath, err))
		}
	}
	generator, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(policy.Generator)))
	if err != nil {
		violations = append(violations, fmt.Errorf("private protocol generator %s: %w", policy.Generator, err))
	} else {
		sourceAssignment := `$privateProtocolSource = "` + canonicalPrivateProto + `"`
		generationCall := regexp.MustCompile(`(?m)&\s+protoc[^\r\n]*\$privateProtocolSource`)
		if !strings.Contains(string(generator), sourceAssignment) || !generationCall.Match(generator) {
			violations = append(violations, fmt.Errorf(
				"private protocol generator %s does not generate %s",
				policy.Generator,
				canonicalPrivateProto,
			))
		}
	}
	proto, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(policy.Proto)))
	if err != nil {
		violations = append(violations, fmt.Errorf("private protocol proto %s: %w", policy.Proto, err))
	} else {
		expected := `option go_package = "` + policy.GoPackage + `";`
		if !strings.Contains(string(proto), expected) {
			violations = append(violations, fmt.Errorf("private protocol %s does not declare %s", policy.Proto, expected))
		}
	}
	generated, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(policy.Generated)))
	if err != nil {
		violations = append(violations, fmt.Errorf("private protocol generated %s: %w", policy.Generated, err))
	} else if !strings.Contains(string(generated), "source: "+policy.Proto) {
		violations = append(violations, fmt.Errorf("private protocol generated source does not reference %s", policy.Proto))
	}
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".pb.go") {
			return nil
		}
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			violations = append(violations, fmt.Errorf("private protocol inspect %s: %w", filepath.ToSlash(path), readErr))
			return nil
		}
		if !strings.Contains(string(contents), "source: "+policy.Proto) {
			return nil
		}
		relativePath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			violations = append(violations, fmt.Errorf("private protocol relative generated path: %w", relErr))
			return nil
		}
		relativePath = filepath.ToSlash(relativePath)
		if relativePath != policy.Generated {
			violations = append(violations, fmt.Errorf("private protocol has second generated output %s", relativePath))
		}
		return nil
	})
	if walkErr != nil {
		violations = append(violations, fmt.Errorf("private protocol scan generated outputs: %w", walkErr))
	}
	return violations
}
