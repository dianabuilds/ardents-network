package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	closedTLSKeyExchangeOwner  = "internal/route/transport/key_exchange.go"
	closedTLSKeyExchangeGroups = "ClosedProfileKeyExchangeGroups"
)

// The closed TLS profile fixes its key-exchange groups in one production owner.
// Every production tls.Config takes that owner's list: an inline group literal
// anywhere else, or a different list, would silently fork the profile (#903).
// Test peers keep their own independent literals and are not inspected.
func TestClosedTLSKeyExchangeGroupsHaveOneProductionOwner(t *testing.T) {
	inventory, err := inspectClosedTLSKeyExchange(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.owners) != 1 || inventory.owners[0] != closedTLSKeyExchangeOwner {
		t.Errorf("closed TLS key-exchange literal owners = %v, want exactly %s", inventory.owners, closedTLSKeyExchangeOwner)
	}
	if len(inventory.forks) != 0 {
		t.Errorf("production CurvePreferences outside the closed-profile owner: %v", inventory.forks)
	}

	fixture := t.TempDir()
	for name, source := range map[string]string{
		"internal/other/borrowed.go":    "package other\nimport \"crypto/tls\"\nvar _ = &tls.Config{CurvePreferences: transport.ClosedProfileKeyExchangeGroups()}\n",
		"internal/other/local.go":       "package other\nimport \"crypto/tls\"\nvar _ = &tls.Config{CurvePreferences: ClosedProfileKeyExchangeGroups()}\n",
		"internal/other/inline.go":      "package other\nimport \"crypto/tls\"\nvar _ = []tls.CurveID{tls.X25519MLKEM768, tls.X25519}\n",
		"internal/other/aliased.go":     "package other\nimport stdtls \"crypto/tls\"\nvar _ = stdtls.X25519MLKEM768\n",
		"internal/other/forked.go":      "package other\nimport \"crypto/tls\"\nvar _ = &tls.Config{CurvePreferences: []tls.CurveID{tls.CurveP256}}\n",
		"internal/other/inline_test.go": "package other\nimport \"crypto/tls\"\nvar _ = &tls.Config{CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519}}\n",
		"scripts/tool.go":               "package main\nimport \"crypto/tls\"\nvar _ = &tls.Config{CurvePreferences: []tls.CurveID{tls.X25519MLKEM768}}\n",
	} {
		path := filepath.Join(fixture, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	control, err := inspectClosedTLSKeyExchange(fixture)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"internal/other/aliased.go", "internal/other/inline.go"}
	if strings.Join(control.owners, ",") != strings.Join(want, ",") {
		t.Errorf("fixture literal owners = %v, want %v", control.owners, want)
	}
	if len(control.forks) != 1 || control.forks[0] != "internal/other/forked.go" {
		t.Errorf("fixture forks = %v, want exactly the non-owner list", control.forks)
	}
}

type closedTLSKeyExchangeInventory struct {
	owners []string // Production files that name the hybrid group.
	forks  []string // Production files that set CurvePreferences from anything but the owner.
}

// inspectClosedTLSKeyExchange reads every non-test Go file under internal/ and
// cmd/. Test files and non-product trees may hold independent peer literals.
func inspectClosedTLSKeyExchange(root string) (closedTLSKeyExchangeInventory, error) {
	inventory := closedTLSKeyExchangeInventory{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if !strings.HasSuffix(relative, ".go") || strings.HasSuffix(relative, "_test.go") ||
			!(strings.HasPrefix(relative, "internal/") || strings.HasPrefix(relative, "cmd/")) {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		packages := make(map[string]string)
		for _, imported := range file.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			name := pathpkg.Base(dependency)
			if imported.Name != nil {
				name = imported.Name.Name
			}
			packages[name] = dependency
		}
		hybrid := false
		ast.Inspect(file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.SelectorExpr:
				if qualifier, ok := typed.X.(*ast.Ident); ok && typed.Sel.Name == "X25519MLKEM768" && packages[qualifier.Name] == "crypto/tls" {
					hybrid = true
				}
			case *ast.KeyValueExpr:
				key, ok := typed.Key.(*ast.Ident)
				if !ok || key.Name != "CurvePreferences" {
					return true
				}
				call, ok := typed.Value.(*ast.CallExpr)
				if !ok {
					inventory.forks = append(inventory.forks, relative)
					return true
				}
				var function string
				switch callee := call.Fun.(type) {
				case *ast.Ident:
					function = callee.Name
				case *ast.SelectorExpr:
					function = callee.Sel.Name
				}
				if function != closedTLSKeyExchangeGroups {
					inventory.forks = append(inventory.forks, relative)
				}
			}
			return true
		})
		if hybrid {
			inventory.owners = append(inventory.owners, relative)
		}
		return nil
	})
	sort.Strings(inventory.owners)
	sort.Strings(inventory.forks)
	return inventory, err
}
