package recoverysmoke

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func (observer dockerObserver) binaryIdentities(ctx context.Context) (map[string]string, error) {
	raw, err := observer.docker(ctx, time.Minute, "create", "--network", "none", "--label",
		"com.docker.compose.project="+observer.project, observer.imageID,
		"/usr/local/bin/ardents-recovery-qualify", "/run/ardents/evidence.json")
	identity := containerIDFromOutput(raw)
	if err != nil || !validContainerID(identity) {
		return nil, errors.New("binary-inspection container identity is invalid")
	}
	defer func() { _, _ = observer.docker(context.Background(), time.Minute, "rm", "-f", identity) }()
	result := make(map[string]string, 5)
	for _, name := range []string{"ardents-route", "ardents-qualify", "ardents-service", "ardents-stream-app", "ardents-recovery-qualify"} {
		path := filepath.Join(observer.input.FixtureRoot, "binary-"+name)
		if _, err := observer.docker(ctx, time.Minute, "cp", identity+":/usr/local/bin/"+name, path); err != nil {
			return nil, err
		}
		raw, err := byteio.ReadFile(path, 32<<20)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(raw)
		result[name] = hex.EncodeToString(digest[:])
	}
	return result, nil
}
