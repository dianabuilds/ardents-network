package endpoint

import "testing"

func TestPermissionEventRequiresPhaseAndRoleOnSameRecord(t *testing.T) {
	journal := []byte(`permission-accepted reader
{"phase":"permission-pending","role":"reader"}
{"phase":"permission-accepted","role":"publisher","sequence":4}
{"phase":"permission-accepted","role":false}
{"phase":"permission-accepted","role":"reader"
`)
	if !hasPermissionEvent(journal, "permission-pending", "reader") || !hasPermissionEvent(journal, "permission-accepted", "publisher") {
		t.Fatal("original role event missing")
	}
	if hasPermissionEvent(journal, "permission-accepted", "reader") {
		t.Fatal("combined or malformed records supplied Reader acceptance")
	}
}
