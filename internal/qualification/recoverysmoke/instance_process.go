package recoverysmoke

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

var generateInstance = generateInstanceProcess

func generateInstanceProcess(sourceRoot, privatePath string) ([32]byte, error) {
	var public [32]byte
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "run", "./cmd/ardents-instance-fixture", "generate", privatePath)
	command.Dir = sourceRoot
	command.Env = append(os.Environ(), "GOENV=off")
	output := byteio.NewBuffer(4 << 10)
	diagnostics := byteio.NewBuffer(4 << 10)
	command.Stdout, command.Stderr = output, diagnostics
	if err := command.Run(); err != nil || output.Overflowed() || diagnostics.Overflowed() {
		return public, errors.Join(err, errors.New(string(diagnostics.Bytes())))
	}
	var receipt struct {
		Schema string `json:"schema"`
		Public string `json:"instance_public"`
	}
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil || receipt.Schema != "ardents-h3-instance-host-v1" {
		return public, errors.New("publisher-host Instance receipt is invalid")
	}
	decoded, err := hex.DecodeString(receipt.Public)
	if err != nil || len(decoded) != len(public) {
		return public, errors.New("publisher-host Instance public key is invalid")
	}
	copy(public[:], decoded)
	return public, nil
}
