//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	candidate := flag.String("candidate", "", "obfs4 or webtunnel")
	evidence := flag.String("evidence", "", "external evidence directory")
	seed := flag.String("seed", "", "shared 32-byte workload seed in hexadecimal")
	clientSHA := flag.String("client-sha256", "", "expected client binary SHA-256")
	serverSHA := flag.String("server-sha256", "", "expected server binary SHA-256")
	harnessSHA := flag.String("harness-sha256", "", "expected harness SHA-256")
	image := flag.String("image", "", "pinned runtime image identity")
	flag.Parse()
	provenance := runProvenance{Seed: *seed, ClientSHA256: *clientSHA,
		ServerSHA256: *serverSHA, HarnessSHA256: *harnessSHA, Image: *image}
	if err := run(*candidate, *evidence, provenance); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
