package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Delivery consumes the committed receipt and operation identifiers, not
// Vault opening, signing, or any other Custody operation. The command owns
// that original operation and closes it before delivery.
func TestCustodyDeliveryUsesOnlyReceiptFacts(t *testing.T) {
	directory := filepath.Join(repositoryRoot(t), "internal/domain/custody/delivery")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"Receipt":                           true,
		"OperationIssueServiceCredential":   true,
		"OperationIssueAdmissionPermission": true,
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, entry.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if path != modulePath+"/internal/domain/custody" {
				continue
			}
			alias := "custody"
			if imported.Name != nil {
				alias = imported.Name.Name
			}
			if alias == "." || alias == "_" {
				t.Errorf("%s imports custody without an explicit selector", entry.Name())
				continue
			}
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				owner, ok := selector.X.(*ast.Ident)
				if ok && owner.Name == alias && !allowed[selector.Sel.Name] {
					t.Errorf("%s accesses custody.%s beyond receipt facts", entry.Name(), selector.Sel.Name)
				}
				return true
			})
		}
	}
}
