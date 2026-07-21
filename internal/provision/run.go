package provision

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	apppolicy "ardents/internal/policy"
	"ardents/internal/storage"
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
	if err := ensureToken(configured.secretDir, "api-token"); err != nil {
		return err
	}
	if err := ensureToken(filepath.Join(configured.nodeDir, "applications"), "application-token"); err != nil {
		return err
	}
	if err := writeOperatorDocument(configured.secretDir, document); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "node=initialized name=%s config=%s\n", configured.nodeName, filepath.Join(configured.secretDir, "operator.json"))
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

func ensureToken(secretDir, name string) error {
	path := filepath.Join(secretDir, name)
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
			return fmt.Errorf("existing %s permissions are invalid", name)
		}
		raw, err := os.ReadFile(path)
		if err != nil || strings.TrimSpace(string(raw)) == "" {
			return fmt.Errorf("existing %s is unreadable or empty", name)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect %s: %w", name, err)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate %s: %w", name, err)
	}
	token := []byte(base64.RawURLEncoding.EncodeToString(raw))
	if err := storage.AtomicWritePrivateFile(path, token); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}
