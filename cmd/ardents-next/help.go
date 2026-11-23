package main

import (
	"fmt"
	"io"
)

const participantUsage = `Usage
  ardents stream publish <absolute-administration-socket>
  ardents stream connect <absolute-application-socket> --link-file <absolute-private-link-file>
  ardents publish <absolute-administration-socket> <absolute-document-file>
  ardents link <absolute-administration-socket>
  ardents read <absolute-application-socket>
  ardents endpoint withdraw <absolute-administration-socket>

Stream exchanges arbitrary bytes through standard input and output.
Publication succeeds after Descriptor acknowledgement. Read takes a Target Link
on standard input.
Publishing, link presentation and withdrawal require Administration authorization.

For installed host and infrastructure actions, run ardents help operator.
For release tooling, run ardents help release.`

const operatorHelp = `Endpoint host
  ardents enrollment verify <bundle-root> <manifest-sha256>
  ardents platform initialize-trust <launch-declaration> <sha256>
  ardents platform stage-endpoint <request>
  ardents platform unit <launch-declaration>

Use the host service manager to start the staged Endpoint under its admitted
Ubuntu profile. The manager invokes endpoint start-installed and the confined
document worker.

Network infrastructure
  ardents platform template
  ardents network inspect --config <path>
  ardents diagnostics timeline

The infrastructure operator uses the signed distribution for the assigned
Source or Node duty. The installed service manager invokes source serve-retained
or node with that duty's retained configuration.

Service and Admission Authority
  Obtain the separate authenticated offline ardents-custody executable.
  Its own usage lists issuance, verification and Vault recovery operations.`

const releaseHelp = `Build and release verification
  ardents enrollment check-candidate <bundle-root>
  ardents enrollment check-protected-generation <static-root> <endpoint> <document-worker> <release-identity>

These packaging checks validate candidate files. First execution requires
the independent manifest pin and the applicable Release authorization.

Infrastructure artifact preparation
  ardents infrastructure provision <role> <operation> [arguments]
  ardents infrastructure sign-release <role> <operation> [arguments]

Use the exact role and operation from the command reference. The signed
Endpoint participant archive and the offline Authority Custody executable
are separate release assets.`

func runHelp(args []string, out, diagnostic io.Writer) int {
	var message string
	if len(args) == 0 {
		message = participantUsage
	} else if len(args) == 1 {
		switch args[0] {
		case "operator":
			message = operatorHelp
		case "release":
			message = releaseHelp
		}
	}
	if message == "" {
		fmt.Fprintln(diagnostic, "usage: ardents help [operator|release]")
		return 2
	}
	if _, err := fmt.Fprintln(out, message); err != nil {
		return 2
	}
	return 0
}
