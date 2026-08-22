package updatetransaction

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaInventoryBinding(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	oracleBootstrapV0(t, root)
	owner := sha256.Sum256([]byte("schema inventory owner"))
	content := sha256.Sum256([]byte("schema inventory content"))
	selection := SchemaSelection{Owner: owner, Content: content}
	selection.Identity = schemaSelectionIdentity(selection)
	raw, err := encodeSchemaCurrent(schemaCurrent{Selection: selection})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "schema-current")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	store, inspection, err := acquireStore(root, 1)
	if err != nil || inspection.schemaCurrent == nil || inspection.schemaCurrent.Selection != selection {
		t.Fatalf("acquire Store schema=%+v err=%v", inspection.schemaCurrent, err)
	}
	if releaseErr := store.release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}
	forged := append([]byte(nil), raw...)
	forged[recordHeaderBytes] = 1
	if err := os.WriteFile(path, forged, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := acquireStore(root, 1); !errors.Is(err, errRecordInvalid) {
		t.Fatalf("acquire forged schema = %v, want record invalid", err)
	}
}
