package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	identitylocalrealm "ardents/internal/identity/localrealm"
	runtimeprocess "ardents/internal/runtime/process"
)

type options struct {
	authorityDir  string
	nodeDir       string
	secretDir     string
	nodeName      string
	bootstrapPeer string
	transportPort int
}

func main() {
	if err := run(os.Args[1:], os.Stdout, time.Now); err != nil {
		fmt.Fprintf(os.Stderr, "provision local realm: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, clock func() time.Time) error {
	configured, err := parseOptions(args)
	if err != nil {
		return err
	}
	authority, err := identitylocalrealm.OpenOrCreate(configured.authorityDir)
	if err != nil {
		return err
	}
	provisioned, err := authority.ProvisionNode(identitylocalrealm.NodeOptions{
		DataDir: configured.nodeDir, SecretDir: configured.secretDir, Clock: clock,
	}, runtimeprocess.NewPolicyService(runtimeprocess.PolicyConfig{}))
	if err != nil {
		return err
	}
	document := operatorDocument(configured, provisioned)
	if err := writeOperatorDocument(configured.secretDir, document); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "local-realm-node=provisioned node=%s\n", configured.nodeName)
	return err
}

func parseOptions(args []string) (options, error) {
	set := flag.NewFlagSet("ard-provision", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	var configured options
	set.StringVar(&configured.authorityDir, "authority-dir", "", "protected local realm authority directory")
	set.StringVar(&configured.nodeDir, "node-dir", "", "persistent node data directory")
	set.StringVar(&configured.secretDir, "secret-dir", "", "persistent node secret directory")
	set.StringVar(&configured.nodeName, "node-name", "", "canonical node name")
	set.StringVar(&configured.bootstrapPeer, "bootstrap-peer", "", "validated Waku bootstrap multiaddr")
	set.IntVar(&configured.transportPort, "transport-port", 0, "Waku TCP listen port")
	if err := set.Parse(args); err != nil {
		return options{}, fmt.Errorf("invalid provisioning arguments")
	}
	if set.NArg() != 0 || strings.TrimSpace(configured.authorityDir) == "" ||
		strings.TrimSpace(configured.nodeDir) == "" || strings.TrimSpace(configured.secretDir) == "" ||
		strings.TrimSpace(configured.nodeName) == "" || configured.transportPort < 1 || configured.transportPort > 65535 {
		return options{}, fmt.Errorf("authority-dir, node-dir, secret-dir, node-name, and transport-port are required")
	}
	return configured, nil
}
