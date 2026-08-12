package byteio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBoundsApplyBeforeUnboundedRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(path, 4); err == nil {
		t.Fatal("oversized file was accepted")
	}
	buffer := NewBuffer(4)
	if written, err := buffer.Write([]byte("12345")); err != nil || written != 5 || string(buffer.Bytes()) != "1234" || !buffer.Overflowed() {
		t.Fatalf("bounded output = %d %q %v %v", written, buffer.Bytes(), buffer.Overflowed(), err)
	}
}
