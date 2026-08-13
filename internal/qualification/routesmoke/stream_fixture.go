package routesmoke

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

// StreamFixture identifies one generated Stage 2 Route fixture adapted to a
// pair of caller-owned local stream sockets.
type StreamFixture struct {
	NetworkID, ManifestDigest [32]byte
	At                        time.Time
}

// PrepareStreamFixture creates one private Route fixture for a Stage 3 attempt.
func PrepareStreamFixture(root, clientSocket, publisherSocket string, at time.Time) (StreamFixture, error) {
	if root == "" || !filepath.IsAbs(root) || clientSocket == "" || publisherSocket == "" || at.IsZero() {
		return StreamFixture{}, errors.New("stream route fixture input is incomplete")
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		return StreamFixture{}, errors.New("stream route fixture root must be new")
	}
	for _, path := range []string{"plans", "state"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0o700); err != nil {
			return StreamFixture{}, err
		}
	}
	for _, role := range []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"} {
		if err := os.MkdirAll(filepath.Join(root, "secrets", role), 0o700); err != nil {
			return StreamFixture{}, err
		}
	}
	prepared, err := buildFixture(root, at)
	if err != nil {
		return StreamFixture{}, err
	}
	if err := addStream(filepath.Join(root, "plans", "client.json"), clientSocket); err != nil {
		return StreamFixture{}, err
	}
	if err := addStream(filepath.Join(root, "plans", "publisher.json"), publisherSocket); err != nil {
		return StreamFixture{}, err
	}
	return StreamFixture{NetworkID: prepared.base.NetworkID, ManifestDigest: prepared.manifest, At: at}, nil
}

func addStream(path, socket string) error {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > 64<<10 {
		return errors.New("route role plan is missing or oversized")
	}
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("route role plan is malformed")
	}
	value["Stream"] = socket
	return byteio.WriteJSON(path, value, 64<<10)
}
