package signing

import (
	"context"
	"io"
	"os/signal"
	"syscall"
	"time"

	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func runReleaseKeys(role nodeReleaseRole, args []string, out io.Writer) error {
	if len(args) != 1 {
		return artifactio.ErrInput
	}
	var refusal string
	switch role {
	case "source":
		refusal = "source control failed"
	case forwardingReleaseRole:
		refusal = "forwarding artifact preparation refused"
	case joinReleaseRole:
		refusal = "JOIN artifact preparation refused"
	case descriptorReleaseRole:
		refusal = "Descriptor artifact preparation refused"
	default:
		return artifactio.ErrInput
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	public, err := prepareReleaseKeys(ctx, args[0], nil)
	if err != nil {
		return artifactio.Refuse(err, refusal)
	}
	return writeReleaseKeyReceipt(ctx, out, string(role)+".prepare-release-keys", public)
}
