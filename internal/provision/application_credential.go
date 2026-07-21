package provision

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	runtimeconfig "ardents/internal/config"
	"ardents/internal/storage"
)

const applicationCredentialLifetime = 30 * 24 * time.Hour

func RenewApplicationCredential(args []string, stdout io.Writer) error {
	return renewApplicationCredential(args, stdout, time.Now)
}

func renewApplicationCredential(args []string, stdout io.Writer, clock func() time.Time) error {
	set := flag.NewFlagSet("ardentsd application-credential renew", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	var path string
	set.StringVar(&path, "config", "", "active operator configuration path")
	if err := set.Parse(args); err != nil || set.NArg() != 0 || strings.TrimSpace(path) == "" {
		return fmt.Errorf("--config is required")
	}
	document, err := runtimeconfig.Load(path)
	if err != nil {
		return fmt.Errorf("load active operator configuration: %w", err)
	}
	if !document.ApplicationInterface.Enabled {
		return fmt.Errorf("application interface is disabled")
	}
	expiresAt := clock().UTC().Truncate(time.Second).Add(applicationCredentialLifetime)
	document.ApplicationInterface.CredentialExpiresAt = expiresAt.Format(time.RFC3339)
	if err := runtimeconfig.Validate(document); err != nil {
		return fmt.Errorf("renewed operator configuration is invalid: %w", err)
	}
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode renewed operator configuration")
	}
	if err := storage.AtomicWritePrivateFile(path, raw); err != nil {
		return fmt.Errorf("write renewed operator configuration: %w", err)
	}
	_, err = fmt.Fprintf(stdout, "application-credential=renewed expires_at=%s config=%s\n", expiresAt.Format(time.RFC3339), path)
	return err
}
