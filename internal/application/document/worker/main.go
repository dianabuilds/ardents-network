// The fixed worker must not open host cgroup files before its descriptor audit.
// Its installed unit supplies a fixed GOMAXPROCS and enforces the resource cap.
//
//go:debug containermaxprocs=0
//go:debug updatemaxprocs=0
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/application/document"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "document worker unavailable")
		os.Exit(2)
	}
}

func run(arguments []string) error {
	if len(arguments) == 1 {
		switch arguments[0] {
		case "worker-reader":
			return document.RunInheritedWorker(document.ReaderWorker)
		case "worker-stream":
			return document.RunInheritedWorker(document.ByteStreamWorker)
		case "worker-publisher":
			return document.RunInheritedWorker(document.PublisherWorker)
		}
	}
	return errors.New("document worker entrypoint is invalid")
}
