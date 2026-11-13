package join

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRecoveryCompletionBindsOriginalCutAndRefusesEarlierPrefixes(t *testing.T) {
	prefix, cut := []byte("immutable original prefix\n"), []byte("separate original cut\n")
	b := transitionCutResult{Boundary: transitionMessage{Operation: "replacement", Records: 62, Journal: "/var/lib/node.join-replacement-original-journal", Device: 8, Inode: 45, BindingSHA256: digest([]byte("selection"))}}
	wanted := recoveryCompletion{"ardents-join-recovery-completion-v1", b.Boundary.Journal, 8, 45, digest(prefix), digest(cut), b.Boundary.BindingSHA256, "selected-stopped"}
	encode := func(r recoveryCompletion) []byte { raw, _ := json.Marshal(r); return append(raw, '\n') }
	raw := encode(wanted)
	if _, err := decodeRecoveryCompletion(raw, prefix, cut, b); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*recoveryCompletion){
		func(r *recoveryCompletion) { r.Device++ }, func(r *recoveryCompletion) { r.Inode++ },
		func(r *recoveryCompletion) { r.Journal += "-foreign" }, func(r *recoveryCompletion) { r.OriginalCutSHA256 = digest(nil) },
		func(r *recoveryCompletion) { r.PrefixSHA256 = digest(nil) }, func(r *recoveryCompletion) { r.BindingSHA256 = digest(nil) },
		func(r *recoveryCompletion) { r.Outcome = "running" }, func(r *recoveryCompletion) { r.Schema = "ardents-forwarding-recovery-completion-v1" },
	} {
		other := wanted
		mutate(&other)
		if _, err := decodeRecoveryCompletion(encode(other), prefix, cut, b); err == nil {
			t.Fatal("foreign completion accepted", other)
		}
	}
	for n := uint32(0); n < 62; n++ {
		before := b
		before.Boundary.Records = n
		if _, err := decodeRecoveryCompletion(raw, prefix, cut, before); err == nil {
			t.Fatal("unproved earlier prefix", n)
		}
	}
	for _, invalid := range [][]byte{raw[:len(raw)-1], append(bytes.Clone(raw), '\n'), bytes.Replace(raw, []byte("\"schema\":"), []byte("\"schema\": "), 1), append(bytes.Clone(raw[:len(raw)-2]), []byte(",\"unknown\":true}\n")...)} {
		if _, err := decodeRecoveryCompletion(invalid, prefix, cut, b); err == nil {
			t.Fatal("noncanonical completion accepted")
		}
	}
	if _, err := decodeRecoveryCompletion(raw, append(bytes.Clone(prefix), 'x'), cut, b); err == nil {
		t.Fatal("changed original prefix accepted")
	}
	if _, err := decodeRecoveryCompletion(raw, prefix, append(bytes.Clone(cut), 'x'), b); err == nil {
		t.Fatal("changed cut accepted")
	}
	// Pure framing controls supply no actual original cut, native join or Release.
}

func TestJoinRecoveryControllerHasOneExplicitOperation(t *testing.T) {
	for _, op := range []string{"install-initial-stopped", "start-initial", "restart", "stop", "recover", "forwarding"} {
		if transitionOperation(op) != "" {
			t.Fatal("foreign operation", op)
		}
	}
	if transitionRecordLimit("replacement") != 62 || transitionRecordLimit("retirement") != 0 {
		t.Fatal("expanded cut grammar")
	}
}
