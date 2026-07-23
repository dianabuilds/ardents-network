package provision

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	apppolicy "ardents/internal/policy"
)

type options struct {
	authorityDir     string
	nodeDir          string
	secretDir        string
	nodeName         string
	bootstrapPeer    string
	transportPort    int
	runtimeDataDir   string
	runtimeSecretDir string
}

func Run(args []string, stdout io.Writer) error {
	return run(args, stdout, time.Now)
}

func run(args []string, stdout io.Writer, clock func() time.Time) error {
	configured, err := parseOptions(args)
	if err != nil {
		return err
	}
	authority, err := OpenOrCreate(configured.authorityDir)
	if err != nil {
		return err
	}
	provisioned, err := authority.ProvisionNode(NodeOptions{
		DataDir: configured.nodeDir, SecretDir: configured.secretDir, Clock: clock,
	}, apppolicy.New(apppolicy.Config{}))
	if err != nil {
		return err
	}
	document := operatorDocument(configured, provisioned)
	if err := writeOperatorDocument(configured.secretDir, document); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "node=initialized name=%s principal=%s config=%s\n", configured.nodeName, provisioned.Subject, filepath.Join(configured.secretDir, "operator.json"))
	return err
}

func parseOptions(args []string) (options, error) {
	set := flag.NewFlagSet("ardentsd init", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	var configured options
	set.StringVar(&configured.authorityDir, "authority-dir", "", "protected local realm authority directory")
	set.StringVar(&configured.nodeDir, "node-dir", "", "persistent node data directory")
	set.StringVar(&configured.secretDir, "secret-dir", "", "persistent node secret directory")
	set.StringVar(&configured.nodeName, "node-name", "", "canonical node name")
	set.StringVar(&configured.bootstrapPeer, "bootstrap-peer", "", "validated Waku bootstrap multiaddr")
	set.IntVar(&configured.transportPort, "transport-port", 0, "Waku TCP listen port")
	set.StringVar(&configured.runtimeDataDir, "runtime-data-dir", "", "node data path visible to ardentsd")
	set.StringVar(&configured.runtimeSecretDir, "runtime-secret-dir", "", "node secret path visible to ardentsd")
	if err := set.Parse(args); err != nil {
		return options{}, fmt.Errorf("invalid provisioning arguments")
	}
	if set.NArg() != 0 || strings.TrimSpace(configured.authorityDir) == "" ||
		strings.TrimSpace(configured.nodeDir) == "" || strings.TrimSpace(configured.secretDir) == "" ||
		strings.TrimSpace(configured.nodeName) == "" || configured.transportPort < 1 || configured.transportPort > 65535 {
		return options{}, fmt.Errorf("authority-dir, node-dir, secret-dir, node-name, and transport-port are required")
	}
	if strings.TrimSpace(configured.runtimeDataDir) == "" {
		configured.runtimeDataDir = configured.nodeDir
	}
	if strings.TrimSpace(configured.runtimeSecretDir) == "" {
		configured.runtimeSecretDir = configured.secretDir
	}
	return configured, nil
}
