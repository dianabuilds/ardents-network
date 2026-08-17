//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/lab/modulecache"
)

func main() {
	workspace := flag.String("workspace", ".", "clean repository root")
	output := flag.String("output", "", "new external gomodcache.tar.gz")
	flag.Parse()
	receipt, err := modulecache.Generate(modulecache.Options{Workspace: *workspace, Output: *output})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("sha256=%s bytes=%d\n", receipt.SHA256, receipt.Bytes)
}
