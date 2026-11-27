package ext4

import (
	"strings"
	"testing"
)

func TestMountIdentity(t *testing.T) {
	const ext4Mount = "12 1 8:1 / /storage rw - ext4 /dev/sda rw\n"
	const ext3Mount = "13 1 8:2 / /storage rw - ext3 /dev/sdb rw\n"
	tests := []struct {
		name   string
		mounts string
		id     uint64
		want   bool
	}{
		{"original ext4 despite another mount at its path", ext4Mount + ext3Mount, 12, true},
		{"other ext family refuses", ext4Mount + ext3Mount, 13, false},
		{"unmounted original refuses", ext3Mount, 12, false},
		{"duplicate identity refuses", ext4Mount + ext4Mount, 12, false},
		{"malformed table refuses", ext4Mount + "bad record\n", 12, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := requireMount(strings.NewReader(test.mounts), test.id)
			if (err == nil) != test.want {
				t.Fatalf("mount acceptance = %v, want %v: %v", err == nil, test.want, err)
			}
		})
	}
}

func TestMissingDescriptorRefuses(t *testing.T) {
	if err := Require(nil); err == nil {
		t.Fatal("missing descriptor accepted")
	}
}
