package recoverysmoke

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"
)

type hostScopeEvidence struct {
	Adapter, AdapterProjection       string
	Machine, Campaign, Source, Image [32]byte
	Commitment                       [32]byte
}

func (observer dockerObserver) observeDockerHostScope(ctx context.Context,
	manifest [32]byte, imageID string) (hostScopeEvidence, error) {
	raw, err := observer.docker(ctx, 10*time.Second, "info", "--format", "{{.ID}}")
	identity := strings.TrimSpace(string(raw))
	if err != nil || identity == "" {
		return hostScopeEvidence{}, errors.Join(err, errors.New("docker host identity is unavailable"))
	}
	result := hostScopeEvidence{Adapter: "docker-compose-v1", AdapterProjection: observer.project,
		Machine:  sha256.Sum256([]byte(identity)),
		Campaign: sha256.Sum256([]byte(observer.project + "\x00" + hex32(manifest))),
		Source:   sha256.Sum256([]byte(observer.sourceCommit)), Image: sha256.Sum256([]byte(imageID))}
	result.Commitment = hostScopeCommitment(result)
	return result, nil
}

func hostScopeCommitment(value hostScopeEvidence) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-qualification-host-scope-v1\x00" + value.Adapter + "\x00"))
	for _, field := range [][32]byte{value.Machine, value.Campaign, value.Source, value.Image} {
		_, _ = hash.Write(field[:])
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}
