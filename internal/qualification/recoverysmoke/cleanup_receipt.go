package recoverysmoke

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type cleanupObservation struct {
	adapter           string
	scope, commitment [32]byte
	observedAt        int64
	owned             uint32
	adapterProjection []byte
}

type dockerCleanupProjection struct {
	Project                       string
	Containers, Networks, Volumes uint32
}

type cleanupDockerCommand func(context.Context, time.Duration, ...string) ([]byte, error)

const cleanupListMaximum = 64 << 10

func (observer dockerObserver) observeDockerCleanup(ctx context.Context, scope hostScopeEvidence,
	clock time.Time) (cleanupObservation, error) {
	command := func(ctx context.Context, timeout time.Duration, arguments ...string) ([]byte, error) {
		return observer.commandBounded(ctx, timeout, cleanupListMaximum, "docker", arguments...)
	}
	return collectDockerCleanup(ctx, command, observer.project, scope, clock)
}

func collectDockerCleanup(ctx context.Context, command cleanupDockerCommand, project string,
	scope hostScopeEvidence, clock time.Time) (cleanupObservation, error) {
	projection := dockerCleanupProjection{Project: project}
	counts := []*uint32{&projection.Containers, &projection.Networks, &projection.Volumes}
	for index, kind := range []string{"container", "network", "volume"} {
		raw, err := command(ctx, time.Minute, dockerOwnedListArguments(kind, project)...)
		if err != nil {
			return cleanupObservation{}, fmt.Errorf("enumerate owned Docker %s resources: %w", kind, err)
		}
		if len(raw) > cleanupListMaximum {
			return cleanupObservation{}, fmt.Errorf("enumerate owned Docker %s resources: output exceeds %d bytes",
				kind, cleanupListMaximum)
		}
		*counts[index] = uint32(len(strings.Fields(string(raw))))
	}
	raw, err := json.Marshal(projection)
	if err != nil {
		return cleanupObservation{}, fmt.Errorf("encode Docker cleanup projection: %w", err)
	}
	result := cleanupObservation{adapter: scope.Adapter, scope: scope.Commitment,
		observedAt:        max(int64(1), time.Since(clock).Nanoseconds()),
		owned:             projection.Containers + projection.Networks + projection.Volumes,
		adapterProjection: raw}
	result.commitment = cleanupCommitment(result)
	if result.owned != 0 {
		return cleanupObservation{}, fmt.Errorf("qualification scope retains %d owned resources", result.owned)
	}
	return result, nil
}

func dockerOwnedListArguments(kind, project string) []string {
	arguments := []string{kind, "ls"}
	if kind == "container" {
		arguments = append(arguments, "-a")
	}
	return append(arguments, "-q", "--filter", "label=com.docker.compose.project="+project)
}

func cleanupCommitment(value cleanupObservation) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-qualification-cleanup-observation-v1\x00"))
	_, _ = hash.Write(value.scope[:])
	_, _ = hash.Write([]byte(value.adapter))
	projection := sha256.Sum256(value.adapterProjection)
	_, _ = hash.Write(projection[:])
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(value.observedAt))
	_, _ = hash.Write(encoded[:])
	binary.BigEndian.PutUint32(encoded[:4], value.owned)
	_, _ = hash.Write(encoded[:4])
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}
