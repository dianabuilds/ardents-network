package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

func runSourceInspection(args []string, out io.Writer) error {
	if len(args) != 10 {
		return errors.New("usage: ardents-control source inspect-latest SERVE_PLAN CLIENT_CERT CLIENT_KEY SERVER_CA SERVER_NAME SERVER_KEY_PIN SOURCE_ID FAMILY HANDLE MATERIAL_INDEX")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	response, err := inspectSource(ctx, args)
	if err != nil {
		return &sourceControlFailure{err}
	}
	raw, err := json.Marshal(struct {
		Operation    string   `json:"operation"`
		Status       string   `json:"status"`
		ObjectDigest [32]byte `json:"object_digest"`
		Payload      []byte   `json:"payload"`
	}{"source.inspect-latest", response.Status, response.ObjectDigest, response.Payload})
	if err != nil {
		return err
	}
	return writeSourceControlBytes(ctx, out, append(raw, '\n'))
}

func inspectSource(ctx context.Context, args []string) (response source.Message, result error) {
	if ctx == nil || len(args) != 10 {
		return source.Message{}, errSourceControlInput
	}
	var pin, identity [32]byte
	for i, destination := range []*[32]byte{&pin, &identity} {
		raw, err := hex.DecodeString(args[5+i])
		if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != args[5+i] {
			return response, errSourceControlInput
		}
		copy(destination[:], raw)
	}
	index, err := strconv.ParseUint(args[9], 10, 32)
	if err != nil || index >= 64 || strconv.FormatUint(index, 10) != args[9] {
		return response, errSourceControlInput
	}
	inputs := &sourceControlInputs{ctx: ctx}
	defer func() {
		result = errors.Join(result, inputs.close(), ctx.Err())
		if result != nil {
			response = source.Message{}
		}
	}()
	var raw [4][]byte
	for i, path := range args[:4] {
		for _, previous := range args[:i] {
			if path == previous {
				return response, errSourceControlInput
			}
		}
		maximum := int64(64 << 10)
		if i == 0 || i == 2 {
			maximum = 16 << 10
		}
		raw[i], err = inputs.read(path, maximum)
		if err != nil {
			return response, err
		}
		if i == 2 {
			private := inputs.files[len(inputs.files)-1].info
			if private.Mode().Perm() != 0600 || private.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
				return response, errSourceControlInput
			}
		}
	}
	plan, err := source.DecodeServingDeclaration(raw[0])
	if err != nil {
		return response, err
	}
	config, err := sourcePreparationStateConfig(plan)
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
	if err = inputs.check(); err != nil {
		return response, err
	}
	response, err = source.InspectLatest(ctx, source.Inspection{Source: source.Source{Address: plan.Address, ServerName: args[4],
		Identity: identity, Family: args[7], EndpointHandle: args[8], RootPEM: raw[3], LeafKeyDigest: pin}, ClientCertificate: credential,
		Authorities: config.Authorities, VerificationClock: time.Now, NetworkID: config.NetworkID, MaterialIndex: uint32(index)})
	return response, errors.Join(err, inputs.check())
}
