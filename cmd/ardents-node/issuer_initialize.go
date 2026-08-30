package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

type issuerInitializationPlan struct {
	Schema             string `json:"schema"`
	Root               string `json:"root"`
	NetworkID          string `json:"network_id"`
	NodeID             string `json:"node_id"`
	IdentityKey        string `json:"identity_key"`
	InitiatorNodeID    string `json:"initiator_node_id"`
	InitiatorPublicKey string `json:"initiator_public_key"`
	AssignmentNotAfter string `json:"assignment_not_after"`
	Budget             uint16 `json:"budget"`
}

func runIssuer(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) == 3 && arguments[0] == "serve" && arguments[1] == "--config" {
		return runIssuerNode(ctx, arguments[2], output)
	}
	if len(arguments) != 3 || arguments[0] != "initialize" || arguments[1] != "--config" {
		return errors.New("usage: ardents-node issuer (initialize|serve) --config PATH")
	}
	var plan issuerInitializationPlan
	if err := decodeOperatorInput(arguments[2], 32<<10, &plan); err != nil {
		return err
	}
	if plan.Schema != "ardents-transit-issuer-initialize-v1" || !filepath.IsAbs(plan.Root) ||
		filepath.Clean(plan.Root) != plan.Root || !filepath.IsAbs(plan.IdentityKey) || filepath.Clean(plan.IdentityKey) != plan.IdentityKey {
		return errors.New("transit issuer initialization plan is not canonical")
	}
	config := credential.IssuerRootConfig{Root: plan.Root, Budget: plan.Budget, Clock: time.Now}
	if err := decodeOperatorFixedHex(plan.NetworkID, config.NetworkID[:]); err != nil {
		return err
	}
	if err := decodeOperatorFixedHex(plan.NodeID, config.NodeID[:]); err != nil {
		return err
	}
	if err := decodeOperatorFixedHex(plan.InitiatorNodeID, config.InitiatorNodeID[:]); err != nil {
		return err
	}
	if err := decodeOperatorFixedHex(plan.InitiatorPublicKey, config.InitiatorPublicKey[:]); err != nil {
		return err
	}
	var err error
	config.IdentityKey, err = node.IdentityKey(plan.IdentityKey)
	if err != nil {
		return err
	}
	config.AssignmentNotAfter, err = time.Parse(time.RFC3339, plan.AssignmentNotAfter)
	if err != nil || config.AssignmentNotAfter.Format(time.RFC3339) != plan.AssignmentNotAfter {
		return errors.New("transit issuer assignment deadline is invalid")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	receipt, err := credential.InitializeIssuerRoot(config)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Schema        string `json:"schema"`
		Profile       []byte `json:"profile"`
		ProfileSHA256 string `json:"profile_sha256"`
	}{Schema: "ardents-transit-issuer-profile-v1", Profile: receipt.Profile,
		ProfileSHA256: hex.EncodeToString(receipt.ProfileDigest[:])})
}

func runIssuerNode(ctx context.Context, path string, output io.Writer) error {
	runtime, err := readNodePlan(path)
	if err != nil {
		return err
	}
	if err := validateIssuerRuntime(runtime); err != nil {
		return err
	}
	return runNodeRuntime(ctx, runtime, output)
}

func validateIssuerRuntime(runtime nodeRuntime) error {
	if runtime.node.TransitIssuer.Root == "" || runtime.node.Rendezvous.Certificate.PrivateKey != nil ||
		runtime.node.Initiator.Certificate.PrivateKey != nil || runtime.node.Introduction.Certificate.PrivateKey != nil ||
		runtime.node.Responder.Certificate.PrivateKey != nil {
		return errors.New("issuer serve requires only one Transit Grant issuer reservation")
	}
	return nil
}
