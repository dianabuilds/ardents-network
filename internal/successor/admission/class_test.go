package admission

import (
	"testing"
	"time"
)

func TestClosedClassContract(t *testing.T) {
	for _, test := range []struct {
		class    Class
		bytes    uint64
		lifetime time.Duration
	}{
		{1, 65536, 30 * time.Second}, {2, 33554432, 30 * time.Minute}, {3, 8388608, 10 * time.Minute},
	} {
		if test.class.ByteLimit() != test.bytes || test.class.Lifetime() != test.lifetime {
			t.Fatalf("class %d changed its closed contract: %d/%s", test.class, test.class.ByteLimit(), test.class.Lifetime())
		}
	}
	for value := 0; value <= 255; value++ {
		if value >= 1 && value <= 3 {
			continue
		}
		if class := Class(value); class.ByteLimit() != 0 || class.Lifetime() != 0 {
			t.Fatalf("unsupported class %d acquired allowance", value)
		}
	}
}
