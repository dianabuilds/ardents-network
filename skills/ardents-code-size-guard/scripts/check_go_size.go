package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	fileSoftLimit = 300
	fileHardLimit = 450
	funcSoftLimit = 40
	funcHardLimit = 70
)

type finding struct {
	path   string
	kind   string
	name   string
	loc    int
	soft   int
	hard   int
	hardBr bool
}

func main() {
	includeTests := flag.Bool("include-tests", false, "include _test.go files")
	flag.Parse()

	if flag.NArg() == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "usage: go run ./skills/ardents-code-size-guard/scripts/check_go_size.go [--include-tests] <paths>")
		os.Exit(2)
	}

	files, err := collectFiles(flag.Args(), *includeTests)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Println("no matching Go files found")
		return
	}

	var findings []finding
	for _, path := range files {
		fileFindings, err := inspectFile(path)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			os.Exit(1)
		}
		findings = append(findings, fileFindings...)
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].path == findings[j].path {
			if findings[i].kind == findings[j].kind {
				return findings[i].name < findings[j].name
			}
			return findings[i].kind < findings[j].kind
		}
		return findings[i].path < findings[j].path
	})

	hardBreaches := 0
	if len(findings) == 0 {
		fmt.Println("code size check passed")
		return
	}

	for _, item := range findings {
		level := "SOFT"
		if item.hardBr {
			level = "HARD"
			hardBreaches++
		}
		label := item.kind
		if item.name != "" {
			label += " " + item.name
		}
		fmt.Printf("[%s] %s: %s (%d LOC, soft=%d, hard=%d)\n", level, item.path, label, item.loc, item.soft, item.hard)
	}

	if hardBreaches > 0 {
		os.Exit(1)
	}
}

func collectFiles(paths []string, includeTests bool) ([]string, error) {
	var files []string
	seen := map[string]struct{}{}

	for _, input := range paths {
		matches, err := filepath.Glob(input)
		if err == nil && len(matches) > 0 {
			for _, match := range matches {
				if err := collectPath(match, includeTests, seen, &files); err != nil {
					return nil, err
				}
			}
			continue
		}
		if err := collectPath(input, includeTests, seen, &files); err != nil {
			return nil, err
		}
	}

	sort.Strings(files)
	return files, nil
}

func collectPath(input string, includeTests bool, seen map[string]struct{}, files *[]string) error {
	info, err := os.Stat(input)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		if shouldCheckFile(input, includeTests) {
			addFile(input, seen, files)
		}
		return nil
	}

	return filepath.WalkDir(input, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "vendor" || base == "aim-core" {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldCheckFile(path, includeTests) {
			addFile(path, seen, files)
		}
		return nil
	})
}

func addFile(path string, seen map[string]struct{}, files *[]string) {
	clean := filepath.Clean(path)
	if _, ok := seen[clean]; ok {
		return
	}
	seen[clean] = struct{}{}
	*files = append(*files, clean)
}

func shouldCheckFile(path string, includeTests bool) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	if strings.Contains(filepath.ToSlash(path), "aim-core/") {
		return false
	}
	base := filepath.Base(path)
	if strings.HasSuffix(base, "_test.go") && !includeTests {
		return false
	}
	if strings.HasSuffix(base, ".pb.go") || strings.HasSuffix(base, ".gen.go") || strings.HasPrefix(base, "zz_generated") {
		return false
	}
	if isGeneratedFile(path) {
		return false
	}
	return true
}

func isGeneratedFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	head := string(data)
	if len(head) > 4096 {
		head = head[:4096]
	}
	return strings.Contains(head, "Code generated")
}

func inspectFile(path string) ([]finding, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(src), "\n")
	commentOnly := commentOnlyLines(lines)

	fileLOC := countRange(lines, commentOnly, 1, len(lines))
	var findings []finding
	if fileLOC > fileSoftLimit {
		findings = append(findings, finding{
			path:   path,
			kind:   "file",
			loc:    fileLOC,
			soft:   fileSoftLimit,
			hard:   fileHardLimit,
			hardBr: fileLOC > fileHardLimit,
		})
	}

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		loc := countRange(lines, commentOnly, start, end)
		if loc <= funcSoftLimit {
			continue
		}
		findings = append(findings, finding{
			path:   path,
			kind:   "func",
			name:   funcName(fn),
			loc:    loc,
			soft:   funcSoftLimit,
			hard:   funcHardLimit,
			hardBr: loc > funcHardLimit,
		})
	}

	return findings, nil
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return recvName(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

func recvName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return "*" + recvName(v.X)
	case *ast.IndexExpr:
		return recvName(v.X)
	case *ast.IndexListExpr:
		return recvName(v.X)
	default:
		return "recv"
	}
}

func countRange(lines []string, commentOnly map[int]bool, start, end int) int {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	total := 0
	for i := start; i <= end; i++ {
		trimmed := strings.TrimSpace(lines[i-1])
		if trimmed == "" || commentOnly[i] {
			continue
		}
		total++
	}
	return total
}

func commentOnlyLines(lines []string) map[int]bool {
	out := make(map[int]bool, len(lines))
	inBlock := false

	for idx, line := range lines {
		lineNo := idx + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		i := 0
		commentOnly := true
		for {
			for i < len(line) && (line[i] == ' ' || line[i] == '\t' || line[i] == '\r') {
				i++
			}
			if i >= len(line) {
				break
			}

			if inBlock {
				end := strings.Index(line[i:], "*/")
				if end == -1 {
					i = len(line)
					break
				}
				i += end + 2
				inBlock = false
				continue
			}

			if strings.HasPrefix(line[i:], "//") {
				i = len(line)
				break
			}
			if strings.HasPrefix(line[i:], "/*") {
				i += 2
				inBlock = true
				continue
			}

			commentOnly = false
			break
		}

		if commentOnly {
			out[lineNo] = true
		}
	}

	return out
}
