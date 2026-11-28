package artifacts

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestReadBoundedFileLimit(t *testing.T) {
	for _, test := range []struct {
		name    string
		body    string
		maximum int64
		refuse  bool
	}{
		{name: "negative", maximum: math.MinInt64, refuse: true},
		{name: "zero accepts empty"},
		{name: "zero refuses nonempty", body: "x", refuse: true},
		{name: "exact", body: "x", maximum: 1},
		{name: "oversized", body: "xy", maximum: 1, refuse: true},
		{name: "maximum int64", body: "x", maximum: math.MaxInt64},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input")
			if err := os.WriteFile(path, []byte(test.body), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := ReadBoundedFile(path, test.maximum)
			if test.refuse {
				if err == nil || got != nil {
					t.Fatalf("read disclosed %q with refusal %v", got, err)
				}
				return
			}
			if err != nil || string(got) != test.body {
				t.Fatalf("read = %q, %v; want %q, nil", got, err, test.body)
			}
		})
	}
}
