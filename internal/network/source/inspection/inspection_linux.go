package inspection

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"os"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

// Request selects one Source and the original files used to authenticate it.
type Request struct {
	ServingPlan, ClientCertificate, ClientKey, ServerCA string
	ServerName, Family, Handle                          string
	ServerKeyPin, SourceID                              [32]byte
	MaterialIndex                                       uint32
}

// InspectLatest performs one contact while retaining exact file custody.
// Any contact, reobservation or cleanup failure discards the response.
func InspectLatest(ctx context.Context, request Request) (response source.Message, result error) {
	if ctx == nil || request.MaterialIndex >= 64 {
		return source.Message{}, artifacts.ErrInput
	}
	inputs := artifacts.NewInputs(ctx)
	defer func() {
		result = errors.Join(result, inputs.Close(), ctx.Err())
		if result != nil {
			response = source.Message{}
		}
	}()
	var raw [4][]byte
	paths := [...]string{request.ServingPlan, request.ClientCertificate, request.ClientKey, request.ServerCA}
	for i, path := range paths {
		for _, previous := range paths[:i] {
			if path == previous {
				return response, artifacts.ErrInput
			}
		}
		maximum := int64(64 << 10)
		if i == 0 || i == 2 {
			maximum = 16 << 10
		}
		input, err := inputs.Read(path, maximum)
		if err != nil {
			return response, err
		}
		raw[i] = input
		if i == 2 {
			private := inputs.Last().Info()
			if private.Mode().Perm() != 0600 || private.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
				return response, artifacts.ErrInput
			}
		}
	}
	plan, err := servingdeclaration.DecodeServingDeclaration(raw[0])
	if err != nil {
		return response, err
	}
	if plan.ProcessProfile != processresource.SourceProfile {
		return response, artifacts.ErrInput
	}
	config, err := state.SourcePreparationConfig(plan)
	if err != nil {
		return response, err
	}
	credential, err := tls.X509KeyPair(raw[1], raw[2])
	if err != nil {
		return response, err
	}
	defer func() {
		if key, ok := credential.PrivateKey.(ed25519.PrivateKey); ok {
			clear(key)
		}
	}()
	if err = inputs.Check(); err != nil {
		return response, err
	}
	response, err = source.InspectLatest(ctx, source.Inspection{Source: source.Source{Address: plan.Address, ServerName: request.ServerName,
		Identity: request.SourceID, Family: request.Family, EndpointHandle: request.Handle, RootPEM: raw[3], LeafKeyDigest: request.ServerKeyPin}, ClientCertificate: credential,
		Authorities: config.Authorities, VerificationClock: time.Now, NetworkID: config.NetworkID, MaterialIndex: request.MaterialIndex})
	return response, errors.Join(err, inputs.Check())
}
