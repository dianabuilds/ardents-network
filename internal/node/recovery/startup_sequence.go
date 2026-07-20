package recovery

import (
	"crypto/ed25519"
	"fmt"

	identityapi "ardents/internal/identity/api"
)

func LoadStartupState(
	loadState func() error,
	loadDiscovery func() error,
	loadData func() error,
	loadWorkloads func() error,
) error {
	if err := loadState(); err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	if err := loadDiscovery(); err != nil {
		return fmt.Errorf("load discovery: %w", err)
	}
	if err := loadData(); err != nil {
		return fmt.Errorf("load data: %w", err)
	}
	if err := loadWorkloads(); err != nil {
		return fmt.Errorf("load workloads: %w", err)
	}
	return nil
}

func InitializeIdentityForStartup(
	ensureNode func() (identityapi.Summary, ed25519.PrivateKey, error),
	setPrivate func(ed25519.PrivateKey),
	setLocalDataNodeID func(string),
	trustPublicKey func(string),
	syncTrustDiagnostics func(),
) error {
	summary, privateKey, err := ensureNode()
	if err != nil {
		return err
	}
	setPrivate(privateKey)
	setLocalDataNodeID(summary.Principal)
	trustPublicKey(summary.PublicKey)
	syncTrustDiagnostics()
	return nil
}
