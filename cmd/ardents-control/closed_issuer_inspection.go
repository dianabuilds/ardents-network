package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// inspectClosedIssuerProfile verifies a public Node export for offline State
// preparation. Its caller supplies the independently authorized Node key and
// identities; this inspection neither accepts State nor supplies its authority.
func inspectClosedIssuerProfile(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("inspect-closed-issuer-profile", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var profilePath, keyText, networkText, nodeText string
	flags.StringVar(&profilePath, "profile", "", "public issuer initialization JSON export")
	flags.StringVar(&keyText, "node-key", "", "independently authorized Node Ed25519 public key in lowercase hex")
	flags.StringVar(&networkText, "network", "", "independently authorized Network ID in lowercase hex")
	flags.StringVar(&nodeText, "node", "", "independently authorized issuer Node ID in lowercase hex")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return errors.New("closed issuer inspection arguments are invalid")
	}
	key, err := decodePublicKey(keyText)
	if err != nil {
		return errors.New("closed issuer Node key is invalid")
	}
	network, err := decodeIdentifier(networkText)
	if err != nil {
		return errors.New("closed issuer Network is invalid")
	}
	node, err := decodeIdentifier(nodeText)
	if err != nil {
		return errors.New("closed issuer Node is invalid")
	}
	raw, err := readControlFile(profilePath, maximumClosedProfilePlanBytes)
	if err != nil {
		return err
	}
	var export struct {
		Schema        string `json:"schema"`
		Profile       []byte `json:"profile"`
		ProfileSHA256 string `json:"profile_sha256"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&export); err != nil || decoder.Decode(&struct{}{}) != io.EOF || export.Schema != "ardents-closed-issuer-profile-v1" {
		return errors.New("closed issuer export is invalid")
	}
	digest := sha256.Sum256(export.Profile)
	if export.ProfileSHA256 != hex.EncodeToString(digest[:]) {
		return errors.New("closed issuer export digest is invalid")
	}
	profile, err := credential.DecodeClosedIssuerProfile(export.Profile, key)
	if err != nil || profile.NetworkID != network || profile.NodeID != node {
		return errors.New("closed issuer profile was not verified")
	}
	// No wall-clock acceptance is performed: successor hourly inventories may be
	// prepared offline before activation. State enforces Epoch and time validity.
	report := struct {
		Schema                  string
		ProfileSHA256           string
		NetworkID, IssuerNodeID string
		NotBefore, NotAfter     string
		TokenKeys               []closedProfilePlanKey
	}{Schema: "ardents-closed-issuer-inspection-v1", ProfileSHA256: export.ProfileSHA256,
		NetworkID: networkText, IssuerNodeID: nodeText,
		NotBefore: profile.NotBefore.Format(time.RFC3339), NotAfter: profile.NotAfter.Format(time.RFC3339)}
	for _, token := range profile.Keys {
		report.TokenKeys = append(report.TokenKeys, closedProfilePlanKey{WindowStart: token.WindowStart.Format(time.RFC3339),
			Class: token.Class, SPKI: base64.RawStdEncoding.EncodeToString(token.SPKI)})
	}
	return json.NewEncoder(output).Encode(report)
}
