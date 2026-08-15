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
	shutdownRung := flag.String("shutdown-rung", "stdin", "stdin, sigterm, or sigkill")
	observer := flag.Bool("dns-observer", false, "observe DNS packets in the shared network namespace")
	observerSync := flag.String("observer-sync", "", "shared observer synchronization directory")
	flag.Parse()
	if *observer {
		if err := runDNSObserver(*observerSync); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	provenance := runProvenance{Candidate: *candidate, Seed: *seed, ClientSHA256: *clientSHA,
		ServerSHA256: *serverSHA, HarnessSHA256: *harnessSHA, Image: *image,
		ShutdownRung: *shutdownRung, Observer: "af-packet-port53-v1"}
	if err := run(*candidate, *evidence, *observerSync, provenance); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
