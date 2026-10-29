package forwarding

import (
	"bytes"
	"path/filepath"
	"testing"
)

// Detached byte commitments only. The actual prior chain, authority, native
// cut and safe-prefix observations are still separate accepting obligations.
func TestForwardingTransitionStartIntentRetainsOriginalPreimage(t *testing.T) {
	b, bindingRaw, _ := selectedGenerationVector(t)
	for _, number := range []attemptNumber{0, 7} {
		predecessor := digest([]byte("original predecessor"))
		if number == 0 {
			predecessor = ""
		}
		initial, targets := digest([]byte("original initial history")), digest([]byte("original Release targets"))
		intent, err := encodeHistoryStartIntent(b, digest(bindingRaw), initial, targets, number, predecessor, 0)
		if err != nil {
			t.Fatal(err)
		}
		message := transitionMessage{Schema: "ardents-forwarding-transition-boundary-v2", Operation: "start", Journal: number.journal(b.Request.InstallationRoot), Generation: b.Generation, BindingSHA256: digest(bindingRaw), Binding: bindingRaw, StartIntent: intent}
		read, err := readTransitionStartIntent(message)
		if err != nil || read.number != number || read.initial != initial || read.predecessor != predecessor || read.history != 0 {
			t.Fatal("original intent commitments lost", err)
		}
		if err := checkTransitionStartIntent(message, nil); err != nil {
			t.Fatal("original preimage before any guard birth", err)
		}
		guard := filepath.Join(number.directory(b.Request.InstallationRoot), "start-guard.json")
		rows := []initialRecord{{Phase: "file-intent", Path: guard, SHA256: digest(intent)}, {Phase: "file-born", Path: guard, SHA256: digest(intent)}, {Phase: "file-ready", Path: guard, SHA256: digest(intent)}, {Phase: "start-attempt", Path: filepath.Dir(guard), SHA256: digest(intent)}}
		if err := checkTransitionStartIntent(message, rows); err != nil {
			t.Fatal("original intent/record commitments", err)
		}
		for n := range rows {
			bad := append([]initialRecord(nil), rows...)
			bad[n].SHA256 = digest([]byte("another intent"))
			if checkTransitionStartIntent(message, bad) == nil {
				t.Fatal("original guard/start commitment substituted", n)
			}
		}
		for _, fault := range []string{"v1-field", "missing", "role", "attempt", "binding", "uid", "framing", "oversize"} {
			bad := message
			switch fault {
			case "v1-field":
				bad.Schema = "ardents-forwarding-transition-boundary-v1"
			case "missing":
				bad.StartIntent = nil
			case "role":
				bad.Operation = "replacement"
			case "attempt":
				bad.Journal = (number + 1).journal(b.Request.InstallationRoot)
			case "binding":
				bad.BindingSHA256 = digest([]byte("other binding"))
			case "uid":
				bad.StartIntent = bytes.Replace(intent, []byte(`"uid":999`), []byte(`"uid":998`), 1)
			case "framing":
				bad.StartIntent = bytes.TrimSuffix(intent, []byte{'\n'})
			case "oversize":
				bad.StartIntent = bytes.Repeat([]byte{'i'}, (8<<10)+1)
			}
			if checkTransitionStartIntent(bad, nil) == nil {
				t.Fatal("foreign original intent admitted", fault)
			}
		}
		fresh := message
		fresh.StartIntent, err = encodeHistoryStartIntent(b, digest(bindingRaw), initial, digest([]byte("later fresh targets")), number, predecessor, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := readTransitionStartIntent(fresh); err != nil {
			t.Fatal("separate canonical later preimage fixture", err)
		}
		if sameTransitionSnapshots(message, fresh) || checkTransitionStartIntent(fresh, rows) == nil {
			t.Fatal("later metadata silently replaced original preimage")
		}
	}
}

func TestForwardingTransitionV1KeepsOmittedIntentBytes(t *testing.T) {
	message := transitionMessage{Schema: "ardents-forwarding-transition-boundary-v1", Operation: "start"}
	raw, err := encodeTransitionMessage(message)
	if err != nil || bytes.Contains(raw, []byte("start_intent")) || !transitionBoundarySchema(message) {
		t.Fatal("legacy boundary bytes changed", err)
	}
	if err := checkTransitionStartIntent(message, nil); err != nil {
		t.Fatal("legacy observation became accepting invented intent", err)
	}
	message.StartIntent = []byte("copied intent\n")
	if transitionBoundarySchema(message) {
		t.Fatal("legacy boundary acquired unversioned intent")
	}
}
