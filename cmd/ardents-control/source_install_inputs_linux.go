package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

type sourceInstallInputOptions struct {
	Network, StateKey, Identity, Family, Handle, Address, ServerName           string
	Installation, State, Participation, Credentials, Clock, PrivateKey, Output string
	Certificate, CertificateSHA, ServerCA, ServerCASHA, ServerPin              string
	ClientCertificate, ClientCertificateSHA, ClientCA, ClientCASHA, ClientPin  string
}

func runSourceInstallInputs(args []string, out io.Writer) error {
	var p sourceInstallInputOptions
	flags := flag.NewFlagSet("source prepare-install-inputs", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	for name, destination := range map[string]*string{"network": &p.Network, "state-key": &p.StateKey, "identity": &p.Identity, "family": &p.Family, "handle": &p.Handle, "address": &p.Address, "server-name": &p.ServerName,
		"installation-root": &p.Installation, "state-root": &p.State, "participation-root": &p.Participation, "credential-directory": &p.Credentials, "clock": &p.Clock, "server-private-key": &p.PrivateKey, "output": &p.Output,
		"server-certificate": &p.Certificate, "server-certificate-sha256": &p.CertificateSHA, "server-ca": &p.ServerCA, "server-ca-sha256": &p.ServerCASHA, "server-key-pin": &p.ServerPin,
		"client-certificate": &p.ClientCertificate, "client-certificate-sha256": &p.ClientCertificateSHA, "client-ca": &p.ClientCA, "client-ca-sha256": &p.ClientCASHA, "client-key-pin": &p.ClientPin} {
		flags.StringVar(destination, name, "", name)
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return errSourceControlInput
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	commitments, err := prepareSourceInstallInputs(ctx, p, nil)
	if err != nil {
		return &sourceControlFailure{err}
	}
	raw, err := json.Marshal(struct {
		Operation string            `json:"operation"`
		Files     map[string]string `json:"files"`
		Native    bool              `json:"native_admission"`
	}{"source.prepare-install-inputs", commitments, false})
	if err != nil {
		return err
	}
	return writeSourceControlBytes(ctx, out, append(raw, '\n'))
}

func sourcePublicID(text string) ([32]byte, error) {
	var id [32]byte
	raw, err := hex.DecodeString(text)
	if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != text {
		return id, errSourceControlInput
	}
	copy(id[:], raw)
	return id, nil
}

// Only public credentials are opened. The external runtime private-key path,
// mutable roots and clock are declarations; Installation/State retain their own
// admission, native delivery, actual time and semantic initialization decisions.
func prepareSourceInstallInputs(ctx context.Context, p sourceInstallInputOptions, observe func(string, *os.File)) (result map[string]string, returned error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || !sourceControlPath(p.Output) {
		return nil, errSourceControlInput
	}
	network, err := sourcePublicID(p.Network)
	if err != nil {
		return nil, err
	}
	stateKey, err := sourcePublicID(p.StateKey)
	if err != nil {
		return nil, err
	}
	identity, err := sourcePublicID(p.Identity)
	if err != nil {
		return nil, err
	}
	serverPin, err := sourcePublicID(p.ServerPin)
	if err != nil {
		return nil, err
	}
	clientPin, err := sourcePublicID(p.ClientPin)
	if err != nil {
		return nil, err
	}
	inputs := &sourceControlInputs{ctx: ctx}
	defer func() {
		returned = errors.Join(returned, inputs.close(), ctx.Err())
		if returned != nil {
			result = nil
		}
	}()
	var files []sourceArtifactFile
	read := func(path, pin, name string) ([]byte, error) {
		if path == p.PrivateKey || !sourceControlPath(path) {
			return nil, errSourceControlInput
		}
		expected, err := sourcePublicID(pin)
		if err != nil {
			return nil, err
		}
		raw, err := inputs.read(path, 64<<10)
		if err != nil {
			return nil, err
		}
		if sha256.Sum256(raw) != expected {
			return nil, errSourceControlInput
		}
		files = append(files, sourceArtifactFile{name, raw})
		return raw, nil
	}
	certificate, err := read(p.Certificate, p.CertificateSHA, "source-certificate.pem")
	if err != nil {
		return nil, err
	}
	serverCA, err := read(p.ServerCA, p.ServerCASHA, "source-ca.pem")
	if err != nil {
		return nil, err
	}
	clientCertificate, err := read(p.ClientCertificate, p.ClientCertificateSHA, "client-certificate.pem")
	if err != nil {
		return nil, err
	}
	clientCA, err := read(p.ClientCA, p.ClientCASHA, "client-ca.pem")
	if err != nil {
		return nil, err
	}
	reference := time.Now().UTC().Truncate(time.Second)
	trust := source.PublicContact{Source: source.PublicSource{Source: source.Source{Address: p.Address, ServerName: p.ServerName, Identity: identity, Family: p.Family, EndpointHandle: p.Handle, RootPEM: serverCA, LeafKeyDigest: serverPin}, CertificatePEM: certificate},
		ClientCertificatePEM: clientCertificate, ClientRootPEM: clientCA, ClientKeyDigest: clientPin}
	pins := map[[32]byte]ed25519.PublicKey{sha256.Sum256(stateKey[:]): ed25519.PublicKey(stateKey[:])}
	if err := source.ObservePublicContact(trust, pins, reference); err != nil {
		return nil, err
	}
	serving := source.ServingDeclaration{Network: &source.ServingNetwork{Root: p.State, NetworkID: network, Authorities: [][32]byte{stateKey}, Threshold: 1, ProfileAuthority: stateKey, ClockObservationFile: p.Clock}, ProcessProfile: "ardents-source-process-v1",
		ParticipationRoot: p.Participation, Address: p.Address, Certificate: filepath.Join(p.Credentials, "certificate.pem"), PrivateKey: filepath.Join(p.Credentials, "private-key.pem"), ClientRoot: filepath.Join(p.Credentials, "client-ca.pem"), ClientPins: [][32]byte{clientPin}}
	if _, err := sourcePreparationStateConfig(serving); err != nil {
		return nil, err
	}
	plan, err := json.Marshal(serving)
	if err != nil {
		return nil, err
	}
	plan = append(plan, '\n')
	if _, err := source.DecodeServingDeclaration(plan); err != nil {
		return nil, err
	}
	request := infrastructure.SourceRequest{Schema: "ardents-source-preflight-v1", Invocation: "source", Profile: "ardents-source-process-v1", InstallationRoot: p.Installation, StateRoot: p.State, ParticipationRoot: p.Participation,
		CredentialDirectory: p.Credentials, Plan: filepath.Join(p.Output, "source-serve.json"), Certificate: filepath.Join(p.Output, "source-certificate.pem"), PrivateKey: p.PrivateKey, ClientCA: filepath.Join(p.Output, "client-ca.pem"), Clock: p.Clock, Address: p.Address}
	requestRaw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	requestRaw = append(requestRaw, '\n')
	if _, err := infrastructure.DecodeSourceRequest(requestRaw); err != nil {
		return nil, err
	}
	files = append(files, sourceArtifactFile{"source-serve.json", plan}, sourceArtifactFile{"source-request.json", requestRaw})
	check := func() error {
		now := time.Now().UTC()
		if now.Before(reference.Add(-time.Minute)) || now.After(reference.Add(5*time.Minute)) {
			return errSourceControlInput
		}
		if err := source.ObservePublicContact(trust, pins, now.Truncate(time.Second)); err != nil {
			return err
		}
		return inputs.checkProtected()
	}
	if err := persistSourceArtifacts(ctx, p.Output, files, check, observe); err != nil {
		return nil, err
	}
	result = make(map[string]string, len(files))
	for _, file := range files {
		digest := sha256.Sum256(file.raw)
		result[file.name] = hex.EncodeToString(digest[:])
	}
	return result, check()
}
