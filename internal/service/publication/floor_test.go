package publication

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestPublicationGenerationFloorRoundTrip(t *testing.T) {
	values := []uint64{1, ^uint64(0)}
	for power := uint64(10); ; power *= 10 {
		values = append(values, power-1, power)
		if power > ^uint64(0)/10 {
			break
		}
	}
	for _, value := range values {
		t.Run(strconv.FormatUint(value, 10), func(t *testing.T) {
			fixture := newPublicationFixture(t)
			root := t.TempDir()
			owner, err := Open(fixture.config(root))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = owner.Close() })
			if _, err := owner.Publish(t.Context(), fixture.input(t, value)); err != nil {
				t.Fatal(err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(root, floorName))
			if err != nil || string(raw) != strconv.FormatUint(value, 10)+"\n" {
				t.Fatalf("persisted floor = %q, %v", raw, err)
			}
			reopened, err := Open(fixture.config(root))
			if err != nil {
				t.Fatalf("reopen generation %d: %v", value, err)
			}
			defer reopened.Close()
			if floor, err := reopened.Floor(); err != nil || floor != value {
				t.Fatalf("restored floor = %d, %v; want %d", floor, err, value)
			}
			if _, err := reopened.Publish(t.Context(), fixture.input(t, value)); err == nil {
				t.Fatal("restart accepted repeated generation")
			}
		})
	}
}

func TestPublicationRefusesMalformedFloorWithoutChangingBytes(t *testing.T) {
	for _, raw := range []string{"", "\n", "0\n", "1", "1\r", "1\r\n", "1\n\n", " 1\n", "1 \n", "+1\n", "-1\n", "01\n", "a\n", "18446744073709551616\n", "100000000000000000000\n"} {
		t.Run(strconv.Quote(raw), func(t *testing.T) {
			fixture := newPublicationFixture(t)
			root := t.TempDir()
			owner, err := Open(fixture.config(root))
			if err != nil {
				t.Fatal(err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, floorName)
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(fixture.config(root))
			if err == nil {
				_ = reopened.Close()
				t.Fatal("malformed floor accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != raw {
				t.Fatalf("refusal changed floor: %q, %v", after, err)
			}
		})
	}
}
