package state

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type alphaGenesisSecrets struct {
	values  [][]byte
	prompts []AlphaGenesisPrompt
}

func (input *alphaGenesisSecrets) ReadSecret(_ context.Context, prompt AlphaGenesisPrompt) ([]byte, error) {
	input.prompts = append(input.prompts, prompt)
	if len(input.values) == 0 {
		return nil, errors.New("unexpected alpha genesis secret request")
	}
	value := append([]byte(nil), input.values[0]...)
	input.values = input.values[1:]
	return value, nil
}

func TestInitializeAlphaGenesisPublishesVerifierAcceptedEmptyState(t *testing.T) {
	root := alphaGenesisTestRoot(t)
	password := []byte("correct horse battery staple")
	input := &alphaGenesisSecrets{values: [][]byte{password, password}}
	now := time.Unix(1_800_100_000, 0).UTC()
	receipt, err := initializeAlphaGenesis(context.Background(), AlphaGenesisConfig{Root: root}, input,
		alphaGenesisPolicy{now: func() time.Time { return now }, random: bytes.NewReader(bytes.Repeat([]byte{0x5a}, 256)), kdfMemoryKiB: 64, kdfPasses: 1, kdfLanes: 1})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Profile != interactiveRouteProfile || receipt.Threshold != 1 || receipt.NotBefore != now ||
		receipt.NotAfter != now.Add(30*24*time.Hour) || len(receipt.Inputs) != 0 || len(receipt.Materials) != 0 {
		t.Fatalf("unexpected alpha genesis receipt: %+v", receipt)
	}
	if receipt.NetworkID == ([32]byte{}) || receipt.AuthorityPublic == ([32]byte{}) || receipt.EpochDigest == ([32]byte{}) {
		t.Fatal("alpha genesis receipt lacks public identity")
	}
	if len(input.prompts) != 2 || input.prompts[0] != AlphaGenesisCreate || input.prompts[1] != AlphaGenesisConfirm {
		t.Fatalf("unexpected secret prompts: %v", input.prompts)
	}

	publicPath := filepath.Join(root, alphaGenesisDirectory, alphaGenesisPublicFile)
	raw, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	var published struct {
		Schema      string   `json:"schema"`
		NetworkID   string   `json:"network_id"`
		EpochDigest string   `json:"epoch_digest"`
		Profile     string   `json:"profile"`
		Threshold   uint8    `json:"threshold"`
		Authorities []string `json:"authorities"`
		Inputs      [][]byte `json:"inputs"`
		Materials   [][]byte `json:"materials"`
		Topology    string   `json:"topology"`
		EnvelopeSHA string   `json:"envelope_sha256"`
	}
	if err := json.Unmarshal(raw, &published); err != nil {
		t.Fatal(err)
	}
	if published.Schema != "ardents-functional-alpha-state-v1" || published.NetworkID != hex.EncodeToString(receipt.NetworkID[:]) ||
		published.EpochDigest != hex.EncodeToString(receipt.EpochDigest[:]) || published.Profile != interactiveRouteProfile ||
		published.Threshold != 1 || len(published.Authorities) != 1 ||
		published.Authorities[0] != hex.EncodeToString(receipt.AuthorityPublic[:]) || len(published.Inputs) != 0 ||
		len(published.Materials) != 0 || published.Topology != "empty-no-persistent-node" ||
		published.EnvelopeSHA != hex.EncodeToString(receipt.EnvelopeDigest[:]) {
		t.Fatalf("unexpected published alpha state: %+v", published)
	}

	publishedReceipt, err := preflightAlphaGenesisPublic(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if !sameAlphaGenesisReceipt(receipt, publishedReceipt) {
		t.Fatalf("published alpha genesis differs from returned receipt: %+v", publishedReceipt)
	}

	envelope, err := os.ReadFile(filepath.Join(root, alphaGenesisDirectory, alphaGenesisSeedFile))
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(envelope) != receipt.EnvelopeDigest || bytes.Contains(envelope, bytes.Repeat([]byte{0x5a}, ed25519.SeedSize)) {
		t.Fatal("encrypted alpha authority record is not bound or exposes seed bytes")
	}
}

func TestInitializeAlphaGenesisRejectsUnsafeRootBeforeSecretAndNeverOverwrites(t *testing.T) {
	input := &alphaGenesisSecrets{}
	if _, err := InitializeAlphaGenesis(context.Background(), AlphaGenesisConfig{Root: filepath.Join(t.TempDir(), "missing")}, input); !errors.Is(err, ErrAlphaGenesisInvalid) {
		t.Fatalf("missing root returned %v", err)
	}
	if len(input.prompts) != 0 {
		t.Fatal("invalid root requested a secret")
	}

	root := alphaGenesisTestRoot(t)
	password := []byte("correct horse battery staple")
	first := &alphaGenesisSecrets{values: [][]byte{password, password}}
	policy := alphaGenesisPolicy{now: func() time.Time { return time.Unix(1_800_100_000, 0).UTC() },
		random: bytes.NewReader(bytes.Repeat([]byte{0x7b}, 256)), kdfMemoryKiB: 64, kdfPasses: 1, kdfLanes: 1}
	if _, err := initializeAlphaGenesis(context.Background(), AlphaGenesisConfig{Root: root}, first, policy); err != nil {
		t.Fatal(err)
	}
	second := &alphaGenesisSecrets{}
	if _, err := initializeAlphaGenesis(context.Background(), AlphaGenesisConfig{Root: root}, second, policy); !errors.Is(err, ErrAlphaGenesisExists) {
		t.Fatalf("existing alpha genesis returned %v", err)
	}
	if len(second.prompts) != 0 {
		t.Fatal("existing alpha genesis requested a secret")
	}
}

func TestInitializeAlphaGenesisRejectsNonOwnerOnlyRootBeforeSecret(t *testing.T) {
	root := alphaGenesisTestRoot(t)
	weakenAlphaGenesisTestRoot(t, root)
	input := &alphaGenesisSecrets{}
	if _, err := InitializeAlphaGenesis(context.Background(), AlphaGenesisConfig{Root: root}, input); !errors.Is(err, ErrAlphaGenesisInvalid) {
		t.Fatalf("non-owner-only root returned %v", err)
	}
	if len(input.prompts) != 0 {
		t.Fatal("non-owner-only root requested a secret")
	}
	if _, err := os.Lstat(filepath.Join(root, alphaGenesisDirectory)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("non-owner-only root published output: %v", err)
	}
}

func TestInitializeAlphaGenesisCancellationAfterSecretLeavesOutputAbsent(t *testing.T) {
	root := alphaGenesisTestRoot(t)
	ctx, cancel := context.WithCancel(context.Background())
	password := []byte("correct horse battery staple")
	input := &alphaGenesisSecrets{values: [][]byte{password, password}}
	policy := alphaGenesisPolicy{now: func() time.Time { return time.Unix(1_800_100_000, 0).UTC() },
		random: bytes.NewReader(bytes.Repeat([]byte{0x2d}, 256)), beforeCommit: cancel,
		kdfMemoryKiB: 64, kdfPasses: 1, kdfLanes: 1}
	if _, err := initializeAlphaGenesis(ctx, AlphaGenesisConfig{Root: root}, input, policy); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation after secret returned %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, alphaGenesisDirectory)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancellation after secret published output: %v", err)
	}
}

func TestInitializeAlphaGenesisRejectsUnsafeClockBeforeSecret(t *testing.T) {
	root := alphaGenesisTestRoot(t)
	input := &alphaGenesisSecrets{}
	policy := alphaGenesisPolicy{now: func() time.Time { return time.Time{} },
		random: bytes.NewReader(bytes.Repeat([]byte{0x6c}, 256)), kdfMemoryKiB: 64, kdfPasses: 1, kdfLanes: 1}
	if _, err := initializeAlphaGenesis(context.Background(), AlphaGenesisConfig{Root: root}, input, policy); !errors.Is(err, ErrAlphaGenesisInvalid) {
		t.Fatalf("unsafe clock returned %v", err)
	}
	if len(input.prompts) != 0 {
		t.Fatal("unsafe clock requested a secret")
	}
	if _, err := os.Lstat(filepath.Join(root, alphaGenesisDirectory)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsafe clock published output: %v", err)
	}
}

func TestInitializeAlphaGenesisRejectsPasswordPolicyAndConfirmationWithoutPublication(t *testing.T) {
	for _, test := range []struct {
		name   string
		values [][]byte
		want   error
	}{
		{name: "short", values: [][]byte{[]byte("too-short"), []byte("too-short")}, want: ErrAlphaGenesisPasswordLength},
		{name: "mismatch", values: [][]byte{[]byte("correct horse battery staple"), []byte("different password value")}, want: ErrAlphaGenesisConfirmation},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := alphaGenesisTestRoot(t)
			input := &alphaGenesisSecrets{values: test.values}
			policy := alphaGenesisPolicy{now: func() time.Time { return time.Unix(1_800_100_000, 0).UTC() },
				random: bytes.NewReader(bytes.Repeat([]byte{0x4d}, 256)), kdfMemoryKiB: 64, kdfPasses: 1, kdfLanes: 1}
			if _, err := initializeAlphaGenesis(context.Background(), AlphaGenesisConfig{Root: root}, input, policy); !errors.Is(err, test.want) {
				t.Fatalf("password rejection returned %v", err)
			}
			if _, err := os.Lstat(filepath.Join(root, alphaGenesisDirectory)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("password rejection published output: %v", err)
			}
		})
	}
}

func alphaGenesisTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	protectAlphaGenesisTestRoot(t, root)
	return root
}
