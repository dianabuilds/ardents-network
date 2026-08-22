//go:build ignore

// R-066's standalone executable measures the disposable Namespace tracer.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/namestore"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

const (
	recordCount       = 127
	lookupSamples     = 100
	concurrentLookups = 8
)

type result struct {
	Records               int    `json:"records"`
	ProofBytes            int    `json:"proof_bytes"`
	LookupP50Micros       int64  `json:"lookup_p50_micros"`
	LookupP95Micros       int64  `json:"lookup_p95_micros"`
	ReopenLookupP50Micros int64  `json:"reopen_lookup_p50_micros"`
	ReopenLookupP95Micros int64  `json:"reopen_lookup_p95_micros"`
	HeapAllocBytes        uint64 `json:"heap_alloc_bytes"`
	ConcurrentLookups     int    `json:"concurrent_lookups"`
}

func main() {
	root, err := os.MkdirTemp("", "ardents-r066-")
	must(err)
	defer func() { must(os.RemoveAll(root)) }()
	network := [32]byte{6, 6}
	policy, signers := materializationPolicy(network)
	store, err := namestore.Open(root, policy)
	must(err)
	records, name := signedHierarchy(network)
	epoch := namestore.Epoch{Number: 1, Digest: [32]byte{1}, CutoffOffset: 1,
		TransitionRoot: sha256.Sum256([]byte("r066-transitions")), TransitionLength: recordCount,
		RejectionRoot: sha256.Sum256([]byte("r066-rejections"))}
	must(store.Commit(epoch, records, thresholdAttester(signers[:2])))
	proof, err := store.Lookup(name, epoch.Number)
	must(err)
	if len(proof) > 4096 {
		fail(fmt.Errorf("proof exceeds fixed response bound"))
	}
	lookup := samples(lookupSamples, func() { _, err := store.Lookup(name, epoch.Number); must(err) })
	checkConcurrent(store, name, epoch.Number)
	must(store.Close())
	reopen := samples(lookupSamples, func() {
		opened, openErr := namestore.Open(root, policy)
		must(openErr)
		_, lookupErr := opened.Lookup(name, epoch.Number)
		must(lookupErr)
		must(opened.Close())
	})
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	output, err := json.Marshal(result{Records: recordCount, ProofBytes: len(proof),
		LookupP50Micros: percentile(lookup, 50).Microseconds(), LookupP95Micros: percentile(lookup, 95).Microseconds(),
		ReopenLookupP50Micros: percentile(reopen, 50).Microseconds(), ReopenLookupP95Micros: percentile(reopen, 95).Microseconds(),
		HeapAllocBytes: memory.HeapAlloc, ConcurrentLookups: concurrentLookups})
	must(err)
	fmt.Println(string(output))
}

func signedHierarchy(network [32]byte) ([][]byte, string) {
	seed := sha256.Sum256([]byte("r066-name-authority"))
	authority := ed25519.NewKeyFromSeed(seed[:])
	records := make([][]byte, recordCount)
	for depth := 1; depth <= recordCount; depth++ {
		name := strings.Repeat("a.", depth-1) + "a"
		record := namespace.Record{Name: name, Generation: 1, Revision: 1, Lease: "active", Consistency: "current", Recovery: "stable", Authority: hex.EncodeToString(authority.Public().(ed25519.PublicKey)), LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000}
		if depth > 1 {
			record.ParentName, record.ParentGeneration = strings.Repeat("a.", depth-2)+"a", 1
		}
		if depth == recordCount {
			record.Target = [32]byte{1}
		}
		var err error
		records[depth-1], err = namespace.SignRecord(network, record, authority)
		must(err)
	}
	return records, strings.Repeat("a.", recordCount-1) + "a"
}

func materializationPolicy(network [32]byte) (namestore.Policy, []ed25519.PrivateKey) {
	policy := namestore.Policy{Network: network, Rule: "ardents-namespace-materialization-v1", Authorities: map[[32]byte]ed25519.PublicKey{}, Threshold: 2}
	keys := make([]ed25519.PrivateKey, 3)
	for i := range keys {
		seed := sha256.Sum256([]byte(fmt.Sprintf("r066-signer-%d", i)))
		keys[i] = ed25519.NewKeyFromSeed(seed[:])
		public := keys[i].Public().(ed25519.PublicKey)
		policy.Authorities[sha256.Sum256(public)] = public
	}
	return policy, keys
}

func thresholdAttester(keys []ed25519.PrivateKey) func([]byte) ([][32]byte, [][]byte, error) {
	return func(transcript []byte) ([][32]byte, [][]byte, error) {
		type signature struct {
			id  [32]byte
			raw []byte
		}
		values := make([]signature, len(keys))
		for i, key := range keys {
			values[i] = signature{id: sha256.Sum256(key.Public().(ed25519.PublicKey)), raw: ed25519.Sign(key, transcript)}
		}
		sort.Slice(values, func(i, j int) bool { return bytes.Compare(values[i].id[:], values[j].id[:]) < 0 })
		ids, signatures := make([][32]byte, len(values)), make([][]byte, len(values))
		for i := range values {
			ids[i], signatures[i] = values[i].id, values[i].raw
		}
		return ids, signatures, nil
	}
}

func samples(count int, run func()) []time.Duration {
	values := make([]time.Duration, count)
	for i := range values {
		started := time.Now()
		run()
		values[i] = time.Since(started)
	}
	return values
}
func percentile(values []time.Duration, value int) time.Duration {
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values[(len(values)-1)*value/100]
}
func checkConcurrent(store *namestore.Store, name string, epoch uint64) {
	var group sync.WaitGroup
	errors := make(chan error, concurrentLookups)
	for range concurrentLookups {
		group.Add(1)
		go func() {
			defer group.Done()
			proof, err := store.Lookup(name, epoch)
			if err != nil || len(proof) == 0 {
				errors <- fmt.Errorf("concurrent lookup: %w", err)
			}
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		must(err)
	}
}
func must(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
