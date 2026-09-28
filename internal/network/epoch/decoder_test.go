package epoch

import "testing"

func TestDecoderText(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		raw       []byte
		want      string
		wantError bool
	}{
		{name: "canonical", raw: []byte{2, 'o', 'k'}, want: "ok"},
		{name: "empty", raw: []byte{0}, wantError: true},
		{name: "over limit", raw: []byte{4, 't', 'e', 's', 't'}, wantError: true},
		{name: "truncated", raw: []byte{2, 'o'}, wantError: true},
		{name: "invalid UTF-8", raw: []byte{1, 0xff}, wantError: true},
		{name: "nonprinting", raw: []byte{1, ' '}, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := newDecoder(test.raw)
			got, err := reader.text(3)
			if (err != nil) != test.wantError || got != test.want {
				t.Fatalf("text(3) = %q, %v; want %q, error=%v", got, err, test.want, test.wantError)
			}
			if !test.wantError && !reader.done() {
				t.Fatalf("consumed %d of %d bytes", reader.offset, len(test.raw))
			}
		})
	}
}
