//go:build ignore

// PROTOTYPE ONLY. This TUI drives synthetic R-053 envelope and recovery states.
package main

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"time"
)

var prototypePassword = []byte("synthetic-r053-passphrase")

type screen struct {
	MemoryKiB  uint32
	Envelope   []byte
	EnvelopeID string
	LastKDF    time.Duration
	LastResult string
	State      machine
}

func main() {
	view := screen{
		MemoryKiB:  64 * 1024,
		LastResult: "ready: synthetic memory only",
		State:      machine{CustodyState: "active", Generation: 3, Revision: 7},
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		render(view)
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case '1':
			view.MemoryKiB = 64 * 1024
			view.LastResult = "selected 64 MiB"
		case '2':
			view.MemoryKiB = 128 * 1024
			view.LastResult = "selected 128 MiB"
		case '3':
			view.MemoryKiB = 256 * 1024
			view.LastResult = "selected 256 MiB"
		case 'e':
			encoded, elapsed, err := seal(prototypePassword, view.MemoryKiB, syntheticPayload())
			view.LastKDF = elapsed
			if err != nil {
				view.LastResult = err.Error()
				continue
			}
			view.Envelope = encoded
			sum := sha256.Sum256(encoded)
			view.EnvelopeID = fmt.Sprintf("%x", sum[:8])
			view.State, view.LastResult = reduce(view.State, action{Kind: "exported"})
		case 'u':
			inner, elapsed, err := openEnvelope(view.Envelope, prototypePassword, syntheticDigest('e'))
			view.LastKDF = elapsed
			if err != nil {
				view.LastResult = err.Error()
				continue
			}
			view.State, view.LastResult = reduce(view.State, action{Kind: "test-verified"})
			view.LastResult += fmt.Sprintf(" generation=%d revision=%d", inner.Authority.Generation, inner.Authority.Revision)
		case 'w':
			_, elapsed, err := openEnvelope(view.Envelope, []byte("wrong-synthetic-password"), syntheticDigest('e'))
			view.LastKDF = elapsed
			view.LastResult = result(err)
		case 't':
			mutated := tamperCiphertext(view.Envelope)
			_, elapsed, err := openEnvelope(mutated, prototypePassword, syntheticDigest('e'))
			view.LastKDF = elapsed
			view.LastResult = result(err)
		case 'x':
			_, elapsed, err := openEnvelope(view.Envelope, prototypePassword, syntheticDigest('x'))
			view.LastKDF = elapsed
			view.LastResult = result(err)
		case 'p':
			mutated := []byte(`{"profile":"ardents-authority-envelope-v1","schema_version":1,"purpose":"recovery-bundle","kdf":{"name":"argon2id","version":19,"memory_kib":524288,"passes":3,"lanes":4,"salt":"AAAAAAAAAAAAAAAAAAAAAA"},"aead":"aes-256-gcm-random-nonce","ciphertext":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`)
			_, elapsed, err := openEnvelope(mutated, prototypePassword, syntheticDigest('e'))
			view.LastKDF = elapsed
			view.LastResult = "pre-KDF parameter result: " + result(err)
		case 'r':
			view.State, view.LastResult = reduce(view.State, action{Kind: "restored", Generation: 3, Revision: 7})
		case 's':
			view.State, view.LastResult = reduce(view.State, action{Kind: "reconcile", Generation: 3, Revision: 8})
		case 'h':
			view.State, view.LastResult = reduce(view.State, action{Kind: "reconcile", Generation: 4, Revision: 8})
		case 'q':
			zero(prototypePassword)
			return
		default:
			view.LastResult = "unknown action"
		}
	}
}

func syntheticPayload() payload {
	return payload{
		Profile: stateName, SchemaVersion: 1, Purpose: "recovery-bundle",
		Environment: syntheticDigest('e'), Network: syntheticDigest('n'), Root: syntheticDigest('r'),
		Authority: authorityState{
			Kind: "service", IDCommitment: syntheticDigest('i'),
			RootMaterial: rawURL.EncodeToString([]byte("synthetic-authority-root-material")),
			Generation:   3, Revision: 7,
			Watermarks: []watermark{{Domain: "credential-generation", Value: 3}, {Domain: "signing-revision", Value: 7}},
		},
	}
}

func syntheticDigest(value byte) string {
	return fmt.Sprintf("%064x", value)
}

func result(err error) string {
	if err == nil {
		return "unexpected success"
	}
	return err.Error()
}

func render(view screen) {
	fmt.Print("\x1b[2J\x1b[H")
	fmt.Println("\x1b[1mR-053 Authority envelope prototype\x1b[0m")
	fmt.Printf("\x1b[1mRuntime\x1b[0m: %s %s/%s; x/crypto %s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH, cryptoVersion())
	fmt.Printf("\x1b[1mKDF\x1b[0m: Argon2id v19, m=%d MiB, t=3, p=4, key=32 B\n", view.MemoryKiB/1024)
	fmt.Printf("\x1b[1mEnvelope\x1b[0m: bytes=%d digest-prefix=%s last-kdf=%s\n", len(view.Envelope), view.EnvelopeID, view.LastKDF)
	fmt.Printf("\x1b[1mCustody\x1b[0m: state=%s generation=%d revision=%d exported=%t test-verified=%t\n", view.State.CustodyState, view.State.Generation, view.State.Revision, view.State.Exported, view.State.TestVerified)
	fmt.Printf("\x1b[1mDerived\x1b[0m: grants=%t instance-key=%q\n", view.State.Grants, view.State.InstanceKey)
	fmt.Printf("\x1b[1mLast result\x1b[0m: %s\n\n", view.LastResult)
	fmt.Println("\x1b[1m1\x1b[0m 64 MiB  \x1b[1m2\x1b[0m 128 MiB  \x1b[1m3\x1b[0m 256 MiB")
	fmt.Println("\x1b[1me\x1b[0m export  \x1b[1mu\x1b[0m unlock/test  \x1b[1mw\x1b[0m wrong secret  \x1b[1mt\x1b[0m tamper  \x1b[1mx\x1b[0m wrong environment")
	fmt.Println("\x1b[1mp\x1b[0m parameter DoS  \x1b[1mr\x1b[0m restore locked  \x1b[1ms\x1b[0m stale reconcile  \x1b[1mh\x1b[0m strictly higher  \x1b[1mq\x1b[0m quit")
	if view.Envelope == nil {
		fmt.Println("\x1b[2mExport before envelope actions. All material is synthetic and in memory.\x1b[0m")
	}
}

func cryptoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dependency := range info.Deps {
		if dependency.Path != "golang.org/x/crypto" {
			continue
		}
		if dependency.Replace != nil {
			return dependency.Replace.Version
		}
		return dependency.Version
	}
	return "unknown"
}
