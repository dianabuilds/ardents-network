package state

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type alphaGenesisPolicy struct {
	now          func() time.Time
	random       io.Reader
	beforeCommit func()
	kdfMemoryKiB uint32
	kdfPasses    uint32
	kdfLanes     uint8
}

type alphaGenesisPublic struct {
	Schema         string    `json:"schema"`
	NetworkID      string    `json:"network_id"`
	EpochDigest    string    `json:"epoch_digest"`
	Profile        string    `json:"profile"`
	Threshold      uint8     `json:"threshold"`
	Authorities    []string  `json:"authorities"`
	Epoch          []byte    `json:"epoch"`
	Inputs         [][]byte  `json:"inputs"`
	Materials      [][]byte  `json:"materials"`
	Topology       string    `json:"topology"`
	NotBefore      time.Time `json:"not_before"`
	NotAfter       time.Time `json:"not_after"`
	EnvelopeSHA256 string    `json:"envelope_sha256"`
}

func defaultAlphaGenesisPolicy() alphaGenesisPolicy {
	return alphaGenesisPolicy{now: time.Now, random: rand.Reader, kdfMemoryKiB: alphaGenesisKDFMemoryKiB,
		kdfPasses: alphaGenesisKDFPasses, kdfLanes: alphaGenesisKDFLanes}
}

func initializeAlphaGenesis(ctx context.Context, config AlphaGenesisConfig, secrets AlphaGenesisSecretInput,
	policy alphaGenesisPolicy) (AlphaGenesisReceipt, error) {
	if err := ctx.Err(); err != nil {
		return AlphaGenesisReceipt{}, err
	}
	root, target, err := checkAlphaGenesisRoot(config.Root)
	if err != nil {
		return AlphaGenesisReceipt{}, err
	}
	if secrets == nil || policy.now == nil || policy.random == nil || policy.kdfMemoryKiB == 0 || policy.kdfPasses == 0 || policy.kdfLanes == 0 {
		return AlphaGenesisReceipt{}, ErrAlphaGenesisInvalid
	}
	now := policy.now().UTC().Truncate(time.Second)
	if now.IsZero() {
		return AlphaGenesisReceipt{}, ErrAlphaGenesisInvalid
	}
	networkID, assignmentSeed, err := alphaGenesisRandomValues(policy.random)
	if err != nil {
		return AlphaGenesisReceipt{}, err
	}
	public, private, err := ed25519.GenerateKey(policy.random)
	if err != nil {
		return AlphaGenesisReceipt{}, fmt.Errorf("generate functional alpha State authority: %w", err)
	}
	defer zeroAlphaGenesis(private)
	var authorityPublic [32]byte
	copy(authorityPublic[:], public)
	notAfter := now.Add(30 * 24 * time.Hour)
	epoch, epochDigest := encodeAlphaGenesis(networkID, assignmentSeed, now, notAfter, private)
	receipt := AlphaGenesisReceipt{NetworkID: networkID, AuthorityPublic: authorityPublic, EpochDigest: epochDigest,
		Profile: interactiveRouteProfile, Threshold: 1, NotBefore: now, NotAfter: notAfter,
		Epoch: append([]byte(nil), epoch...), Inputs: make([][]byte, 0), Materials: make([][]byte, 0)}
	if err := preflightAlphaGenesis(ctx, receipt); err != nil {
		return AlphaGenesisReceipt{}, err
	}
	password, err := secrets.ReadSecret(ctx, AlphaGenesisCreate)
	if err != nil {
		return AlphaGenesisReceipt{}, err
	}
	defer zeroAlphaGenesis(password)
	confirmation, err := secrets.ReadSecret(ctx, AlphaGenesisConfirm)
	if err != nil {
		return AlphaGenesisReceipt{}, err
	}
	defer zeroAlphaGenesis(confirmation)
	if !validAlphaGenesisPassword(password) {
		return AlphaGenesisReceipt{}, ErrAlphaGenesisPasswordLength
	}
	if subtle.ConstantTimeCompare(password, confirmation) != 1 {
		return AlphaGenesisReceipt{}, ErrAlphaGenesisConfirmation
	}
	seed := private.Seed()
	defer zeroAlphaGenesis(seed)
	envelope, err := sealAlphaGenesisRecord(alphaGenesisSeedRecord{Schema: "ardents-functional-alpha-state-seed-v1",
		NetworkID: networkID, GenesisDigest: epochDigest, AuthoritySeed: seed}, password, policy)
	if err != nil {
		return AlphaGenesisReceipt{}, err
	}
	defer zeroAlphaGenesis(envelope)
	receipt.EnvelopeDigest = sha256.Sum256(envelope)
	publicBytes, err := marshalAlphaGenesisPublic(receipt)
	if err != nil {
		return AlphaGenesisReceipt{}, err
	}
	publishedReceipt, err := preflightAlphaGenesisPublic(ctx, publicBytes)
	if err != nil || !sameAlphaGenesisReceipt(receipt, publishedReceipt) {
		return AlphaGenesisReceipt{}, ErrAlphaGenesisInvalid
	}
	if err := publishAlphaGenesis(ctx, root, target, envelope, publicBytes, policy.beforeCommit); err != nil {
		return AlphaGenesisReceipt{}, err
	}
	return receipt, nil
}

func alphaGenesisRandomValues(random io.Reader) ([32]byte, [32]byte, error) {
	var networkID, assignmentSeed [32]byte
	if _, err := io.ReadFull(random, networkID[:]); err != nil {
		return [32]byte{}, [32]byte{}, fmt.Errorf("generate functional alpha Network identifier: %w", err)
	}
	if _, err := io.ReadFull(random, assignmentSeed[:]); err != nil {
		return [32]byte{}, [32]byte{}, fmt.Errorf("generate functional alpha assignment seed: %w", err)
	}
	if networkID == ([32]byte{}) || assignmentSeed == ([32]byte{}) {
		return [32]byte{}, [32]byte{}, ErrAlphaGenesisInvalid
	}
	return networkID, assignmentSeed, nil
}

func preflightAlphaGenesis(ctx context.Context, receipt AlphaGenesisReceipt) error {
	root, err := os.MkdirTemp("", "ardents-alpha-state-genesis-")
	if err != nil {
		return fmt.Errorf("create functional alpha State preflight root: %w", err)
	}
	defer os.RemoveAll(root)
	public := ed25519.PublicKey(receipt.AuthorityPublic[:])
	opened, err := Open(Config{Root: root, NetworkID: receipt.NetworkID,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1,
		AcceptedProfile: receipt.Profile, Now: receipt.NotBefore})
	if err != nil {
		return fmt.Errorf("open functional alpha State preflight: %w", err)
	}
	defer opened.Close()
	snapshot, err := opened.Accept(ctx, receipt.Epoch, receipt.Inputs, receipt.Materials)
	if err != nil {
		return fmt.Errorf("accept functional alpha State genesis: %w", err)
	}
	if snapshot.Digest != receipt.EpochDigest || snapshot.NetworkID != receipt.NetworkID || snapshot.Epoch != 1 ||
		snapshot.Profile != interactiveRouteProfile || snapshot.CandidateCount != 0 || snapshot.ViewLength != 0 || snapshot.RejectedLength != 0 {
		return ErrAlphaGenesisInvalid
	}
	return nil
}

func marshalAlphaGenesisPublic(receipt AlphaGenesisReceipt) ([]byte, error) {
	value := alphaGenesisPublic{Schema: "ardents-functional-alpha-state-v1", NetworkID: hex.EncodeToString(receipt.NetworkID[:]),
		EpochDigest: hex.EncodeToString(receipt.EpochDigest[:]), Profile: receipt.Profile, Threshold: receipt.Threshold,
		Authorities: []string{hex.EncodeToString(receipt.AuthorityPublic[:])}, Epoch: append([]byte(nil), receipt.Epoch...),
		Inputs: make([][]byte, 0), Materials: make([][]byte, 0), Topology: "empty-no-persistent-node",
		NotBefore: receipt.NotBefore, NotAfter: receipt.NotAfter, EnvelopeSHA256: hex.EncodeToString(receipt.EnvelopeDigest[:])}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func preflightAlphaGenesisPublic(ctx context.Context, raw []byte) (AlphaGenesisReceipt, error) {
	var value alphaGenesisPublic
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return AlphaGenesisReceipt{}, ErrAlphaGenesisInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return AlphaGenesisReceipt{}, ErrAlphaGenesisInvalid
	}
	if value.Schema != "ardents-functional-alpha-state-v1" || value.Profile != interactiveRouteProfile ||
		value.Threshold != 1 || value.Topology != "empty-no-persistent-node" || len(value.Authorities) != 1 ||
		len(value.Epoch) == 0 || len(value.Inputs) != 0 || len(value.Materials) != 0 ||
		value.NotBefore.IsZero() || !value.NotBefore.Equal(value.NotBefore.UTC().Truncate(time.Second)) ||
		!value.NotAfter.Equal(value.NotBefore.Add(30*24*time.Hour)) {
		return AlphaGenesisReceipt{}, ErrAlphaGenesisInvalid
	}
	networkID, err := decodeAlphaGenesisHex(value.NetworkID)
	if err != nil {
		return AlphaGenesisReceipt{}, err
	}
	epochDigest, err := decodeAlphaGenesisHex(value.EpochDigest)
	if err != nil {
		return AlphaGenesisReceipt{}, err
	}
	authority, err := decodeAlphaGenesisHex(value.Authorities[0])
	if err != nil {
		return AlphaGenesisReceipt{}, err
	}
	envelopeDigest, err := decodeAlphaGenesisHex(value.EnvelopeSHA256)
	if err != nil {
		return AlphaGenesisReceipt{}, err
	}
	receipt := AlphaGenesisReceipt{EnvelopeDigest: envelopeDigest, NetworkID: networkID, AuthorityPublic: authority,
		EpochDigest: epochDigest, Profile: value.Profile, Threshold: value.Threshold, NotBefore: value.NotBefore.UTC(),
		NotAfter: value.NotAfter.UTC(), Epoch: append([]byte(nil), value.Epoch...), Inputs: make([][]byte, 0), Materials: make([][]byte, 0)}
	if err := preflightAlphaGenesis(ctx, receipt); err != nil {
		return AlphaGenesisReceipt{}, err
	}
	return receipt, nil
}

func decodeAlphaGenesisHex(value string) ([32]byte, error) {
	var result [32]byte
	raw, err := hex.DecodeString(value)
	if err != nil || len(raw) != len(result) || value != hex.EncodeToString(raw) {
		return result, ErrAlphaGenesisInvalid
	}
	copy(result[:], raw)
	if result == ([32]byte{}) {
		return result, ErrAlphaGenesisInvalid
	}
	return result, nil
}

func sameAlphaGenesisReceipt(first, second AlphaGenesisReceipt) bool {
	return first.EnvelopeDigest == second.EnvelopeDigest && first.NetworkID == second.NetworkID &&
		first.AuthorityPublic == second.AuthorityPublic && first.EpochDigest == second.EpochDigest &&
		first.Profile == second.Profile && first.Threshold == second.Threshold && first.NotBefore.Equal(second.NotBefore) &&
		first.NotAfter.Equal(second.NotAfter) && bytes.Equal(first.Epoch, second.Epoch) &&
		len(first.Inputs) == len(second.Inputs) && len(first.Materials) == len(second.Materials)
}

func checkAlphaGenesisRoot(path string) (string, string, error) {
	if path == "" {
		return "", "", ErrAlphaGenesisInvalid
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return "", "", ErrAlphaGenesisInvalid
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", ErrAlphaGenesisInvalid
	}
	if err := validateAlphaGenesisRootAccess(root, info); err != nil {
		return "", "", ErrAlphaGenesisInvalid
	}
	target := filepath.Join(root, alphaGenesisDirectory)
	if _, err := os.Lstat(target); err == nil {
		return "", "", ErrAlphaGenesisExists
	} else if !os.IsNotExist(err) {
		return "", "", fmt.Errorf("inspect functional alpha State output: %w", err)
	}
	return root, target, nil
}
