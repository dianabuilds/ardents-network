//go:build linux

package textdocument

import (
	"os"
	"syscall"
	"testing"
)

func TestDescriptorSurvivedExecRejectsOnlyNonCloseOnExec(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()

	closeOnExec, err := descriptorSurvivedExec(int(read.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if closeOnExec {
		t.Fatal("a close-on-exec descriptor was treated as inherited")
	}

	foreign, err := syscall.Dup(int(read.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(foreign)
	survived, err := descriptorSurvivedExec(foreign)
	if err != nil {
		t.Fatal(err)
	}
	if !survived {
		t.Fatal("a descriptor without close-on-exec was not treated as inherited")
	}
}
