package systemd

import (
	"encoding/json"
	"testing"
)

// Pure native-value encoding controls grant no manager or Startup authority.
func TestManagerPrimitiveArraysPreserveCanonicalJSONAndDepth(t *testing.T) {
	for _, c := range []struct {
		value any
		want  string
	}{
		{[]string(nil), `[]`}, {[]string{}, `[]`}, {[]string{"a", "b"}, `["a","b"]`},
		{[]uint32(nil), `[]`}, {[]uint32{0, 4294967295}, `[0,4294967295]`},
		{[]uint64(nil), `[]`}, {[]uint64{0, 18446744073709551615}, `[0,18446744073709551615]`},
		{[]byte(nil), `[]`}, {[]byte{0, 255}, `[0,255]`},
	} {
		projected, err := managerJSONValue(c.value, 0)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(projected)
		if err != nil || string(raw) != c.want {
			t.Fatal("native numeric/empty array changed", string(raw), c.want, err)
		}
		projected, err = managerJSONValue(c.value, 16)
		if c.want != `[]` {
			if err != ErrObservation {
				t.Fatal("nonempty array bypassed original depth bound", err)
			}
			continue
		}
		raw, marshalErr := json.Marshal(projected)
		if err != nil || marshalErr != nil || string(raw) != `[]` {
			t.Fatal("empty array changed at original depth bound", err, marshalErr)
		}
	}
}

func TestManagerQuotedSizeMatchesFullStandardEncoder(t *testing.T) {
	values := []string{"", "ExecMainStartTimestamp", "a(sasasttttuii)", "a{sv}", "кириллица", "line\nquote\"slash\\", "<>&", "\u2028\u2029", string([]byte{0xff, 0xfe})}
	for i := range 256 {
		values = append(values, string([]byte{byte(i)}))
	}
	for _, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		size, err := managerJSONQuotedSize(value)
		if err != nil || size != len(raw) {
			t.Fatal("complete typed dictionary length changed", value, size, len(raw), err)
		}
	}
}
