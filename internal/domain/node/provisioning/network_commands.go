package provisioning

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkpreparation"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networksigning"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
)

// RunNetworkArtifacts dispatches public preparation, signing and key creation.
// These operations never open a retained runtime State.
func RunNetworkArtifacts(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) == 0 {
		return 2
	}
	switch operation := args[0]; {
	case operation == "prepare-initial-keys":
		return runInitialKeys(ctx, args, out)
	case networksigning.IsPurpose(operation):
		return runNetworkSigning(ctx, args, out)
	case networkdraft.IsPreparation(operation):
		return runNetworkPreparation(ctx, args, out)
	default:
		return 2
	}
}

func runNetworkPreparation(ctx context.Context, args []string, out io.Writer) int {
	if len(args) != 3 || !pathcheck.IsCleanAbsolute(args[1]) || !pathcheck.IsCleanAbsolute(args[2]) {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	prepared, err := networkpreparation.Prepare(ctx, args[0], args[1], args[2])
	if err != nil {
		if errors.Is(err, networkpreparation.ErrOutput) {
			return 1
		}
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	// Grammar preparation is deliberately weaker than signed State acceptance.
	receipt, err := json.Marshal(struct {
		Schema        string `json:"schema"`
		Operation     string `json:"operation"`
		Artifact      string `json:"artifact"`
		SHA256        string `json:"unsigned_sha256"`
		Signed        bool   `json:"signed"`
		StateAccepted bool   `json:"state_accepted"`
		Validation    string `json:"validation"`
	}{"ardents-network-prepared-v1", args[0], prepared.Name, hex.EncodeToString(prepared.Digest[:]), false, false, "public-grammar-only"})
	if err != nil {
		return 1
	}
	if writeNetworkArtifactReport(ctx, out, append(receipt, '\n')) != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	return 0
}

func runNetworkSigning(ctx context.Context, args []string, out io.Writer) int {
	// operation, public-plan, exact-prepared-artifact, reviewed-sha256,
	// original-private-key, fresh-output-directory. No secret bytes in argv.
	if len(args) != 6 {
		return 2
	}
	for _, i := range []int{1, 2, 4, 5} {
		if !pathcheck.IsCleanAbsolute(args[i]) {
			return 2
		}
	}
	if !canonicaldigest.Valid(args[3]) {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	result, err := networksigning.Sign(ctx, args, nil)
	if err != nil {
		return 1
	}
	if result.CheckTime(time.Now()) != nil {
		return 1
	}
	receipt, err := json.Marshal(struct {
		Schema    string `json:"schema"`
		Operation string `json:"operation"`
		Digest    string `json:"signed_sha256"`
		Verified  bool   `json:"signatures_verified"`
		State     bool   `json:"state_accepted"`
		Installed bool   `json:"installed_qualification"`
	}{"ardents-network-signed-v1", args[0], hex.EncodeToString(result.Digest[:]), true, false, false})
	if err != nil || writeNetworkArtifactReport(ctx, out, append(receipt, '\n')) != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	if result.CheckTime(time.Now()) != nil {
		return 1
	}
	return 0
}

func runInitialKeys(ctx context.Context, args []string, out io.Writer) int {
	if len(args) != 3 {
		return 2
	}
	for _, path := range args[1:] {
		if !pathcheck.IsCleanAbsolute(path) || filepath.Dir(path) == path {
			return 2
		}
	}
	if ctx.Err() != nil {
		return 130
	}
	receipt, err := PrepareInitialNetworkKeys(ctx, args[1], args[2])
	if err != nil {
		return 1
	}
	if writeNetworkArtifactReport(ctx, out, receipt) != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	return 0
}
