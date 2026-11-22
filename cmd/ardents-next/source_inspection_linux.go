package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

func runSourceInspection(parent context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) != 10 {
		fmt.Fprintln(diagnostic, "usage: ardents-next source inspect-latest SERVE_PLAN CLIENT_CERT CLIENT_KEY SERVER_CA SERVER_NAME SERVER_KEY_PIN SOURCE_ID FAMILY HANDLE MATERIAL_INDEX")
		return 2
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	response, err := inspectSource(ctx, args)
	if err != nil {
		fmt.Fprintln(diagnostic, "source inspection refused")
		return 1
	}
	raw, err := json.Marshal(struct {
		Operation    string   `json:"operation"`
		Status       string   `json:"status"`
		ObjectDigest [32]byte `json:"object_digest"`
		Payload      []byte   `json:"payload"`
	}{"source.inspect-latest", response.Status, response.ObjectDigest, response.Payload})
	if err != nil || artifacts.WriteReceipt(ctx, out, append(raw, '\n')) != nil {
		return 1
	}
	return 0
}

func inspectSource(ctx context.Context, args []string) (response source.Message, result error) {
	if ctx == nil || len(args) != 10 {
		return source.Message{}, artifacts.ErrInput
	}
	var pin, identity [32]byte
	for i, destination := range []*[32]byte{&pin, &identity} {
		raw, err := hex.DecodeString(args[5+i])
		if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != args[5+i] {
			return response, artifacts.ErrInput
		}
		copy(destination[:], raw)
	}
	index, err := strconv.ParseUint(args[9], 10, 32)
	if err != nil || index >= 64 || strconv.FormatUint(index, 10) != args[9] {
		return response, artifacts.ErrInput
	}
	inputs := artifacts.NewInputs(ctx)
	defer func() {
		result = errors.Join(result, inputs.Close(), ctx.Err())
		if result != nil {
			response = source.Message{}
		}
	}()
	var raw [4][]byte
	for i, path := range args[:4] {
		for _, previous := range args[:i] {
			if path == previous {
				return response, artifacts.ErrInput
			}
		}
		maximum := int64(64 << 10)
		if i == 0 || i == 2 {
			maximum = 16 << 10
		}
		raw[i], err = inputs.Read(path, maximum)
		if err != nil {
			return response, err
		}
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
	response, err = source.InspectLatest(ctx, source.Inspection{Source: source.Source{Address: plan.Address, ServerName: args[4],
		Identity: identity, Family: args[7], EndpointHandle: args[8], RootPEM: raw[3], LeafKeyDigest: pin}, ClientCertificate: credential,
		Authorities: config.Authorities, VerificationClock: time.Now, NetworkID: config.NetworkID, MaterialIndex: uint32(index)})
	return response, errors.Join(err, inputs.Check())
}
