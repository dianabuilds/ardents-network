package forwarding

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestForwardingInvocationIntentPreservesFirstBytesAndBindsEveryPredecessor(t *testing.T) {
	b := binding{UID: 988, GID: 988}
	bh, ih, th, ph := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64)
	first := []byte(fmt.Sprintf("{\"schema\":\"ardents-forwarding-start-intent-v1\",\"binding_sha256\":%q,\"initial_history_sha256\":%q,\"targets_sha256\":%q,\"uid\":988,\"gid\":988,\"attempt\":0}\n", bh, ih, th))
	raw, err := encodeStartIntent(b, bh, ih, th, 0, "")
	if err != nil || !bytes.Equal(raw, first) {
		t.Fatal("first-start bytes changed", err)
	}
	if err := checkStartIntent(first, b, bh, ih, 0, ""); err != nil {
		t.Fatal(err)
	}
	for n := attemptNumber(1); n < attemptLimit; n++ {
		want := []byte(fmt.Sprintf("{\"schema\":\"ardents-forwarding-invocation-intent-v1\",\"binding_sha256\":%q,\"initial_history_sha256\":%q,\"targets_sha256\":%q,\"uid\":988,\"gid\":988,\"attempt\":%d,\"predecessor_sha256\":%q}\n", bh, ih, th, n, ph))
		raw, err := encodeStartIntent(b, bh, ih, th, n, ph)
		if err != nil || !bytes.Equal(raw, want) {
			t.Fatal("independent invocation bytes", n, err)
		}
		if err := checkStartIntent(want, b, bh, ih, n, ph); err != nil {
			t.Fatal(err)
		}
		if checkStartIntent(first, b, bh, ih, n, ph) == nil || checkStartIntent(want, b, bh, ih, n, "") == nil || checkStartIntent(want, b, bh, ih, n, bh) == nil || checkStartIntent(want, b, bh, ih, n-1, ph) == nil {
			t.Fatal("history gap/substitution admitted", n)
		}
		for _, changed := range [][]byte{bytes.Replace(want, []byte("ardents-forwarding-invocation-intent-v1"), []byte("ardents-forwarding-start-intent-v1"), 1), bytes.Replace(want, []byte(`"uid":988`), []byte(`"uid":989`), 1), bytes.Replace(want, []byte(`"targets_sha256":"`+th+`"`), []byte(`"targets_sha256":""`), 1), append(bytes.Clone(want), '\n'), want[:len(want)-1]} {
			if checkStartIntent(changed, b, bh, ih, n, ph) == nil {
				t.Fatal("noncanonical invocation accepted", n)
			}
		}
	}
	if _, err := encodeStartIntent(b, bh, ih, th, 0, ph); err == nil {
		t.Fatal("first start took predecessor")
	}
	if _, err := encodeStartIntent(b, bh, ih, th, attemptLimit, ph); err == nil {
		t.Fatal("attempt exhaustion recycled")
	}
}
