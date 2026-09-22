package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type retiredNameState struct {
	root        string
	network     [32]byte
	digest      [32]byte
	authorities [2]ed25519.PrivateKey
	gateway     ed25519.PrivateKey
	view        state.ResolutionView
}

func prepareRetiredNameState(t *testing.T, root string, now time.Time, endpoints [3]string) retiredNameState {
	t.Helper()
	network := sha256.Sum256([]byte("retired-name-command-network"))
	authorities := [2]ed25519.PrivateKey{
		ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, ed25519.SeedSize)),
		ed25519.NewKeyFromSeed(bytes.Repeat([]byte{78}, ed25519.SeedSize)),
	}
	keys := [3]ed25519.PrivateKey{
		ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, ed25519.SeedSize)),
		ed25519.NewKeyFromSeed(bytes.Repeat([]byte{73}, ed25519.SeedSize)),
		ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, ed25519.SeedSize)),
	}
	seed := [32]byte{75}
	domains := [3]string{"initiator", "rendezvous", "rendezvous"}
	families := [3]string{}
	for index, domain := range domains {
		for attempt := 0; attempt < 1000; attempt++ {
			family := fmt.Sprintf("name-%d-%d", index, attempt)
			if retiredNameDomain(network, seed, family) == domain {
				families[index] = family
				break
			}
		}
		if families[index] == "" {
			t.Fatal("no deterministic Name fixture family for role")
		}
	}
	records := make([][]byte, 3)
	for index := range records {
		records[index] = retiredNameRecord(network, [32]byte{byte(index + 1)}, keys[index],
			families[index], endpoints[index], now)
	}
	epoch, digest := retiredNameEpoch(network, authorities, records, seed, now)
	materials := make([][]byte, len(records))
	for index, record := range records {
		var raw bytes.Buffer
		raw.Write(digest[:])
		writeCommandU32(&raw, uint32(index))
		writeCommandU32(&raw, uint32(len(record)))
		raw.Write(record)
		siblings := retiredNameProof(records, index)
		writeCommandU16(&raw, uint16(len(siblings)))
		for _, sibling := range siblings {
			raw.Write(sibling[:])
		}
		materials[index] = raw.Bytes()
	}
	stateRoot := filepath.Join(root, "network-state")
	public := authorities[0].Public().(ed25519.PublicKey)
	secondPublic := authorities[1].Public().(ed25519.PublicKey)
	config := state.Config{Root: stateRoot, NetworkID: network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public,
			sha256.Sum256(secondPublic): secondPublic},
		Threshold: 2, Now: now, AcceptedProfile: "h3-role-probe-v1"}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	ownerClosed := false
	closeOwner := func() error {
		if ownerClosed {
			return nil
		}
		ownerClosed = true
		return owner.Close()
	}
	t.Cleanup(func() {
		if err := closeOwner(); err != nil {
			t.Errorf("close Name fixture State: %v", err)
		}
	})
	if _, err := owner.Accept(t.Context(), epoch, records, materials); err != nil {
		t.Fatal(err)
	}
	if err := closeOwner(); err != nil {
		t.Fatalf("close accepted Name fixture State: %v", err)
	}
	recovered, err := state.Open(config)
	if err != nil {
		t.Fatalf("recover accepted Name fixture State: %v", err)
	}
	recoveredClosed := false
	closeRecovered := func() error {
		if recoveredClosed {
			return nil
		}
		recoveredClosed = true
		return recovered.Close()
	}
	t.Cleanup(func() {
		if err := closeRecovered(); err != nil {
			t.Errorf("close recovered Name fixture State: %v", err)
		}
	})
	view, viewErr := recovered.CurrentResolution()
	closeErr := closeRecovered()
	if viewErr != nil || closeErr != nil {
		t.Fatalf("recovered Name fixture State: view=%v close=%v", viewErr, closeErr)
	}
	window := now.Add(15 * time.Second)
	for index := range records {
		candidate, available := view.Candidate([32]byte{byte(index + 1)}, now, window)
		if !available || candidate.Domain != domains[index] || candidate.Endpoint != endpoints[index] {
			t.Fatalf("accepted Name fixture candidate %d = %+v, available=%v", index, candidate, available)
		}
	}
	return retiredNameState{root: stateRoot, network: network, digest: digest,
		authorities: authorities, gateway: keys[1], view: view}
}

func retiredNameRecord(network, node [32]byte, private ed25519.PrivateKey,
	family, endpoint string, now time.Time) []byte {
	var raw bytes.Buffer
	raw.WriteString("ARNR")
	raw.WriteByte(1)
	raw.Write(network[:])
	raw.Write(node[:])
	writeCommandU64(&raw, 1)
	writeCommandI64(&raw, now.Add(-time.Minute).Unix())
	writeCommandI64(&raw, now.Add(time.Hour).Unix())
	writeCommandText(&raw, family)
	raw.WriteByte(1)
	writeCommandText(&raw, endpoint)
	writeCommandU16(&raw, 1)
	raw.Write(private.Public().(ed25519.PublicKey))
	raw.Write(ed25519.Sign(private, raw.Bytes()))
	return raw.Bytes()
}

func retiredNameEpoch(network [32]byte, authorities [2]ed25519.PrivateKey, records [][]byte,
	seed [32]byte, now time.Time) ([]byte, [32]byte) {
	var raw bytes.Buffer
	raw.WriteString("AREP")
	raw.WriteByte(1)
	raw.Write(network[:])
	writeCommandU64(&raw, 1)
	raw.Write(make([]byte, 32))
	writeCommandI64(&raw, now.Add(-time.Minute).Unix())
	writeCommandI64(&raw, now.Add(time.Hour).Unix())
	writeCommandU32(&raw, uint32(len(records)))
	writeCommandText(&raw, "h3-role-probe-v1")
	root := retiredNameRoot(records)
	raw.Write(root[:])
	raw.Write(root[:])
	writeCommandU32(&raw, uint32(len(records)))
	emptyRejected := sha256.Sum256([]byte{0x12})
	raw.Write(emptyRejected[:])
	writeCommandU32(&raw, 0)
	raw.Write(seed[:])
	writeCommandText(&raw, "ardents-h3-role-domain-v1")
	writeCommandU32(&raw, uint32(len(records)))
	writeCommandU32(&raw, uint32(len(records)))
	writeCommandU16(&raw, uint16(len(records)))
	writeCommandU16(&raw, 1)
	writeCommandU32(&raw, 1)
	raw.WriteByte(2)
	writeCommandText(&raw, "initiator")
	writeCommandU16(&raw, 1)
	writeCommandU32(&raw, 1)
	writeCommandText(&raw, "rendezvous")
	writeCommandU16(&raw, 2)
	writeCommandU32(&raw, 2)
	digest := sha256.Sum256(raw.Bytes())
	type signer struct {
		id      [32]byte
		private ed25519.PrivateKey
	}
	signers := make([]signer, 0, len(authorities))
	for _, private := range authorities {
		public := private.Public().(ed25519.PublicKey)
		signers = append(signers, signer{id: sha256.Sum256(public), private: private})
	}
	sort.Slice(signers, func(i, j int) bool { return bytes.Compare(signers[i].id[:], signers[j].id[:]) < 0 })
	raw.WriteByte(byte(len(signers)))
	for _, signer := range signers {
		raw.Write(signer.id[:])
		raw.Write(ed25519.Sign(signer.private, digest[:]))
	}
	return raw.Bytes(), digest
}

func retiredNameDomain(network, seed [32]byte, family string) string {
	selectDigest := func(domain string) [32]byte {
		var raw bytes.Buffer
		raw.WriteString("ardents-h3-role-domain-v1\x00")
		raw.Write(network[:])
		_ = binary.Write(&raw, binary.BigEndian, uint64(1))
		raw.Write(seed[:])
		raw.WriteString(family)
		raw.WriteString(domain)
		return sha256.Sum256(raw.Bytes())
	}
	initiator, rendezvous := selectDigest("initiator"), selectDigest("rendezvous")
	if bytes.Compare(initiator[:], rendezvous[:]) < 0 {
		return "initiator"
	}
	return "rendezvous"
}

func retiredNameRoot(records [][]byte) [32]byte {
	if len(records) == 1 {
		return commandMerkleLeaf(records[0])
	}
	split := 1
	for split<<1 < len(records) {
		split <<= 1
	}
	return retiredNameBranch(retiredNameRoot(records[:split]), retiredNameRoot(records[split:]))
}

func retiredNameProof(records [][]byte, index int) [][32]byte {
	if len(records) == 1 {
		return nil
	}
	split := 1
	for split<<1 < len(records) {
		split <<= 1
	}
	if index < split {
		return append(retiredNameProof(records[:split], index), retiredNameRoot(records[split:]))
	}
	return append(retiredNameProof(records[split:], index-split), retiredNameRoot(records[:split]))
}

func retiredNameBranch(left, right [32]byte) [32]byte {
	var raw [65]byte
	raw[0] = 1
	copy(raw[1:33], left[:])
	copy(raw[33:], right[:])
	return sha256.Sum256(raw[:])
}
