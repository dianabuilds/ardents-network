package routesmoke

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	qualification "github.com/dianabuilds/ardents-network/internal/qualification/route"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// RecoveryStreamFixture is one bounded three-candidate-per-role Route fixture.
type RecoveryStreamFixture struct {
	StreamFixture
	Candidates      []route.Position
	RouteCase       qualification.Case
	PublisherPublic [32]byte
}

// PrepareRecoveryStreamFixture creates the private finite-alternate fixture
// used by the Stage 4 Route replacement tracer.
func PrepareRecoveryStreamFixture(root, clientSocket, publisherSocket string,
	at time.Time) (RecoveryStreamFixture, error) {
	if root == "" || !filepath.IsAbs(root) || clientSocket == "" || publisherSocket == "" || at.IsZero() {
		return RecoveryStreamFixture{}, errors.New("recovery stream Route fixture input is incomplete")
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		return RecoveryStreamFixture{}, errors.New("recovery stream Route fixture root must be new")
	}
	for _, path := range []string{"plans", "state"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0o700); err != nil {
			return RecoveryStreamFixture{}, err
		}
	}
	for _, role := range recoveryFixtureRoles() {
		if err := os.MkdirAll(filepath.Join(root, "secrets", role), 0o700); err != nil {
			return RecoveryStreamFixture{}, err
		}
	}
	prepared, err := buildRecoveryFixture(root, at)
	if err != nil {
		return RecoveryStreamFixture{}, err
	}
	for role, socket := range map[string]string{"client": clientSocket, "publisher": publisherSocket} {
		if err := addStream(filepath.Join(root, "plans", role+".json"), socket); err != nil {
			return RecoveryStreamFixture{}, err
		}
	}
	return RecoveryStreamFixture{StreamFixture: StreamFixture{NetworkID: prepared.base.NetworkID,
		ManifestDigest: prepared.manifest, At: at}, Candidates: append([]route.Position(nil), prepared.candidates...),
		RouteCase: prepared.base, PublisherPublic: prepared.publisher}, nil
}

func recoveryFixtureRoles() []string {
	result := []string{"client", "publisher"}
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder"} {
		result = append(result, role, role+"-2", role+"-3")
	}
	return result
}
